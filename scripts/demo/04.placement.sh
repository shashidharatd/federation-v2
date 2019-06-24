#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/demo_util.sh"
source "$(dirname "${BASH_SOURCE}")/../util.sh"

# Prerequisite:
# - a cluster scoped federation deployment with `cluster1` and
#   `cluster2` joined as member clusters
# - kubectl config use-context cluster1
#

TEST_NS="demo"

#Selector baced placement example"

echo
run "kubectl -n ${TEST_NS} get federateddeployment nginx -o yaml |grep -A10 status:"

run "kubectl -n kube-federation-system label kubefedclusters cluster1 region=eu"

echo "Updating placement of federated deployment nginx to only deploy in clusters which match the labels"
run "kubectl -n ${TEST_NS} patch federateddeployment nginx --type=merge --patch '{\"spec\": {\"placement\": {\"clusterSelector\": {\"matchLabels\": {\"region\": \"eu\"}}}}}'"

run "kubectl -n ${TEST_NS} get federateddeployment nginx -o yaml |grep -A10 status:"
