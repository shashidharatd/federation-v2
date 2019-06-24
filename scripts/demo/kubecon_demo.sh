#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/demo_util.sh"
source "$(dirname "${BASH_SOURCE}")/../util.sh"

# Prerequisite:
# - a cluster scoped federation deployment with `cluster1` and
#   `cluster2` joined as member clusters
# - kubectl config use-context cluster1
#



# Demo
kubectl config use-context cluster1

run "kubectl config get-contexts"

run "kubectl -n kube-federation-system get kubefedclusters"



# enable/disable federated types
kubefedctl disable deployments.apps >/dev/null 2>&1

run "kubectl get crds |grep '^federated'"

run "kubefedctl enable deployments.apps"

run "kubectl -n kube-federation-system get federatedtypeconfigs  deployments.apps -o yaml"



TEST_NS="demo"

# Create kubernetes namespace for the resources
echo "Creating a namespace ${TEST_NS}."
run "kubectl create ns ${TEST_NS}"

echo "Creating a ngnix application resources in namespace ${TEST_NS}."
echo
run "kubectl apply -n ${TEST_NS} -f example/sample1/configmap.yaml"
run "kubectl apply -n ${TEST_NS} -f example/sample1/deployment.yaml"
run "kubectl apply -n ${TEST_NS} -f example/sample1/service.yaml"

echo "Federating resources in namespace ${TEST_NS} to member clusters."
echo
run "kubefedctl federate ns ${TEST_NS} --contents --skip-api-resources 'pods,secrets,serviceaccount,replicaset'"

echo "Checking status of federated resources."
echo
run "kubectl -n ${TEST_NS} get federateddeployment nginx -o yaml |grep -A10 status:"

echo
echo "Querying nginx application from member clusters."
run  "kubectl --context cluster1 get node -o jsonpath="{.items[0].status.addresses[0].address}""
echo
IP1=$(kubectl --context cluster1 get node -o jsonpath="{.items[0].status.addresses[0].address}")
run    "kubectl --context cluster1 -n ${TEST_NS} get service -o jsonpath='{.items[0].spec.ports[0].nodePort}'"
echo
PORT1=$(kubectl --context cluster1 -n ${TEST_NS} get service -o jsonpath="{.items[0].spec.ports[0].nodePort}")
run "curl ${IP1}:${PORT1}"
echo

run  "kubectl  --context cluster2 get node -o jsonpath="{.items[0].status.addresses[0].address}""
echo
IP2=$(kubectl  --context cluster2 get node -o jsonpath="{.items[0].status.addresses[0].address}")
run    "kubectl --context cluster2 -n ${TEST_NS} get service -o jsonpath='{.items[0].spec.ports[0].nodePort}'"
echo
PORT2=$(kubectl --context cluster2 -n ${TEST_NS} get service -o jsonpath="{.items[0].spec.ports[0].nodePort}")
run "curl ${IP2}:${PORT2}"
echo


# Override example
echo
echo "Updating override of federated deployment nginx to increase 'replicas' to 2 in cluster2."
run "kubectl patch federateddeployment nginx -n ${TEST_NS} --type=merge --patch '{\"spec\" : {\"overrides\": [{\"clusterName\" : \"cluster2\", \"clusterOverrides\": [{\"path\": \"/spec/replicas\", \"value\" : 2}]}]}}'"
run "kubectl --context cluster2 -n ${TEST_NS} get deployment nginx"



#Selector baced placement example"

echo
run "kubectl -n ${TEST_NS} get federateddeployment nginx -o yaml |grep -A10 status:"

run "kubectl -n kube-federation-system label kubefedclusters cluster1 region=eu"

echo "Updating placement of federated deployment nginx to only deploy in clusters which match the labels"
run "kubectl -n ${TEST_NS} patch federateddeployment nginx --type=merge --patch '{\"spec\": {\"placement\": {\"clusterSelector\": {\"matchLabels\": {\"region\": \"eu\"}}}}}'"

run "kubectl -n ${TEST_NS} get federateddeployment nginx -o yaml |grep -A10 status:"


#Cleanup
kubectl -n kube-federation-system label kubefedclusters cluster1 region-
kubectl delete ns ${TEST_NS}
