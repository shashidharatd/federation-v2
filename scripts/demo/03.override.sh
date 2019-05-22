#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/demo_util.sh"
source "$(dirname "${BASH_SOURCE}")/../util.sh"

# Prerequisite:
# - a cluster scoped federation deployment with `cluster1` and
#   `cluster2` joined as member clusters
# - kubectl config use-context cluster1
#

TEST_NS="demo"

# Override example
echo
echo "Updating override of federated deployment nginx to increase 'replicas' to 2 in cluster2."
run "kubectl patch federateddeployment nginx -n ${TEST_NS} --type=merge --patch '{\"spec\" : {\"overrides\": [{\"clusterName\" : \"cluster2\", \"clusterOverrides\": [{\"path\": \"spec.replicas\", \"value\" : 2}]}]}}'"
run "kubectl --context cluster2 -n ${TEST_NS} get deployment nginx"

