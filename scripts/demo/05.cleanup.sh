#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/demo_util.sh"
source "$(dirname "${BASH_SOURCE}")/../util.sh"

# Prerequisite:
# - a cluster scoped federation deployment with `cluster1` and
#   `cluster2` joined as member clusters
# - kubectl config use-context cluster1
#


TEST_NS="demo"

#Cleanup
kubectl -n kube-federation-system label kubefedclusters cluster1 region-
kubectl delete ns ${TEST_NS}
