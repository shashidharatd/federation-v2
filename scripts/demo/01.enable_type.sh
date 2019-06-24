#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/demo_util.sh"
source "$(dirname "${BASH_SOURCE}")/../util.sh"

# Prerequisite:
# - a cluster scoped federation deployment with `cluster1` and
#   `cluster2` joined as member clusters
# - kubectl config use-context cluster1
#

run "kubectl config get-contexts"

run "kubectl -n kube-federation-system get kubefedclusters --show-labels"


# enable/disable federated types

run "kubefedctl disable deployments.apps"

run "kubectl get crds |grep '^federated'"

run "kubefedctl enable deployments.apps"

run "kubectl -n kube-federation-system get federatedtypeconfigs  deployments.apps -o yaml"
