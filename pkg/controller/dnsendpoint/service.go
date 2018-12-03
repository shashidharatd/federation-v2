/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dnsendpoint

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	restclient "k8s.io/client-go/rest"

	feddnsv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns/v1alpha1"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset/versioned"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
)

func StartServiceDNSEndpointController(config *util.ControllerConfig, stopChan <-chan struct{}) error {
	restclient.AddUserAgent(config.KubeConfig, "Service DNSEndpoint")
	client := fedclientset.NewForConfigOrDie(config.KubeConfig)

	listFunc := func(options metav1.ListOptions) (pkgruntime.Object, error) {
		return client.MulticlusterdnsV1alpha1().ServiceDNSRecords(config.TargetNamespace).List(options)
	}
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		return client.MulticlusterdnsV1alpha1().ServiceDNSRecords(config.TargetNamespace).Watch(options)
	}

	controller, err := newDNSEndpointController(client, &feddnsv1a1.ServiceDNSRecord{}, "service",
		listFunc, watchFunc, getServiceDNSEndpoints, config.MinimizeLatency)
	if err != nil {
		return err
	}

	go controller.Run(stopChan)
	return nil
}

// getServiceDNSEndpoints returns endpoint objects for each ServiceDNSRecord object that should be processed.
func getServiceDNSEndpoints(obj interface{}) ([]*feddnsv1a1.Endpoint, error) {
	var endpoints []*feddnsv1a1.Endpoint
	var commonPrefix string
	labels := make(map[string]string)

	dnsObject, ok := obj.(*feddnsv1a1.ServiceDNSRecord)
	if !ok {
		return nil, fmt.Errorf("received event for unknown object %v", obj)
	}

	if dnsObject.Spec.ExternalName != "" {
		commonPrefix = strings.Join([]string{dnsObject.Spec.ExternalName, dnsObject.Namespace, dnsObject.Spec.DomainRef,
			"svc"}, ".")
		labels["serviceName"] = dnsObject.Name
	} else {
		commonPrefix = strings.Join([]string{dnsObject.Name, dnsObject.Namespace, dnsObject.Spec.DomainRef, "svc"}, ".")
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		zone := clusterDNS.Zone
		region := clusterDNS.Region

		dnsNames := []string{
			strings.Join([]string{commonPrefix, zone, region, dnsObject.Status.Domain}, "."), // zone level
			strings.Join([]string{commonPrefix, region, dnsObject.Status.Domain}, "."),       // region level, one up from zone level
			strings.Join([]string{commonPrefix, dnsObject.Status.Domain}, "."),               // global level, one up from region level
			"", // nowhere to go up from global level
		}

		var zoneTargets, regionTargets, globalTargets feddnsv1a1.Targets
		for _, clusterDNS := range dnsObject.Status.DNS {
			if clusterDNS.Region == region && clusterDNS.Zone == zone {
				zoneTargets = append(zoneTargets, ExtractLoadBalancerTargets(clusterDNS.LoadBalancer)...)
			}
		}

		for _, clusterDNS := range dnsObject.Status.DNS {
			if clusterDNS.Region == region {
				regionTargets = append(regionTargets, ExtractLoadBalancerTargets(clusterDNS.LoadBalancer)...)
			}
		}

		for _, clusterDNS := range dnsObject.Status.DNS {
			globalTargets = append(globalTargets, ExtractLoadBalancerTargets(clusterDNS.LoadBalancer)...)
		}

		targets := [][]string{zoneTargets, regionTargets, globalTargets}

		ttl := dnsObject.Spec.RecordTTL
		if ttl == 0 {
			ttl = defaultDNSTTL
		}
		for i, target := range targets {
			endpoint, err := generateEndpointForServiceDNSObject(dnsNames[i], target, dnsNames[i+1], ttl, labels)
			if err != nil {
				return nil, err
			}
			endpoints = append(endpoints, endpoint)
		}
		if dnsObject.Spec.DNSPrefix != "" {
			endpoint := &feddnsv1a1.Endpoint{
				DNSName:    dnsObject.Spec.DNSPrefix + "." + dnsObject.Status.Domain,
				RecordTTL:  ttl,
				RecordType: RecordTypeCNAME,
			}
			endpoint.Targets = []string{strings.Join([]string{commonPrefix, dnsObject.Status.Domain}, ".")}
			endpoints = append(endpoints, endpoint)
		}
	}

	endpoints = append(endpoints, getWeightedServiceDNSEndpoints(dnsObject)...)

	return DedupeAndMergeEndpoints(endpoints), nil
}

// getWeightedServiceDNSEndpoints creates additional DNS SRV records by calculating weights for the region based on
// number of target endpoints available for the service in the region.
func getWeightedServiceDNSEndpoints(dnsObject *feddnsv1a1.ServiceDNSRecord) []*feddnsv1a1.Endpoint {
	var endpoints []*feddnsv1a1.Endpoint
	regionTargets := make(map[string]int32)
	totalTargets := int32(0)

	// Calculate Total number of target endpoints and regions for the service
	for _, clusterDNS := range dnsObject.Status.DNS {
		totalTargets += clusterDNS.EndpointNum
		if _, exist := regionTargets[clusterDNS.Region]; exist {
			regionTargets[clusterDNS.Region] = regionTargets[clusterDNS.Region] + clusterDNS.EndpointNum
		} else {
			regionTargets[clusterDNS.Region] = clusterDNS.EndpointNum
		}
	}

	// If there are no target endpoints for the service, do not create any weighted records.
	if totalTargets == 0 {
		return endpoints
	}

	commonPrefix := strings.Join([]string{dnsObject.Name, dnsObject.Namespace, dnsObject.Spec.DomainRef, "svc"}, ".")
	for region, regionTargetNum := range regionTargets {
		hostname := strings.Join([]string{commonPrefix, dnsObject.Status.Domain}, ".")
		regionHostname := strings.Join([]string{commonPrefix, region, dnsObject.Status.Domain}, ".")

		for _, port := range dnsObject.Status.Ports {
			portName := port.Name
			if portName == "" {
				// TODO: should the port name be picked from NodePort?
				portName = fmt.Sprintf("%d", port.Port)
			}

			protocol := strings.ToLower(string(port.Protocol))
			if protocol == "" {
				protocol = "tcp"
			}

			// Calculate weight for the region
			weight := regionTargetNum * 100 / totalTargets

			ttl := dnsObject.Spec.RecordTTL
			if ttl == 0 {
				ttl = defaultDNSTTL
			}

			// build a target with a priority of 0, with calculated weight, and pointing the given port on the given host
			target := fmt.Sprintf("0 %d %d %s.", weight, port.Port, regionHostname)

			endpoint := &feddnsv1a1.Endpoint{
				DNSName:    fmt.Sprintf("_%s._%s.%s", portName, protocol, hostname),
				RecordTTL:  ttl,
				RecordType: RecordTypeSRV,
				Targets:    []string{target},
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func generateEndpointForServiceDNSObject(name string, targets feddnsv1a1.Targets, uplevelCname string,
	ttl feddnsv1a1.TTL, labels map[string]string) (ep *feddnsv1a1.Endpoint, err error) {
	ep = &feddnsv1a1.Endpoint{
		DNSName:   name,
		RecordTTL: ttl,
	}

	if len(labels) > 0 {
		ep.Labels = labels
	}

	if len(targets) > 0 {
		targets, err = getResolvedTargets(targets, netWrapper)
		if err != nil {
			return nil, err
		}
		ep.Targets = targets
		ep.RecordType = RecordTypeA
	} else {
		ep.Targets = []string{uplevelCname}
		ep.RecordType = RecordTypeCNAME
	}

	return ep, nil
}
