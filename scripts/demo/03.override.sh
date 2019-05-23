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
run "kubectl patch federatedconfigmap web-file -n ${TEST_NS} --type=merge --patch '{\"spec\" : {\"overrides\": [{\"clusterName\" : \"cluster2\", \"clusterOverrides\": [{\"path\": \"data.content\", \"value\" : \"HelloKubeCon!\"}]}]}}'"

echo
IP2=$(kubectl  --context cluster2 get node -o jsonpath="{.items[0].status.addresses[0].address}")
PORT2=$(kubectl --context cluster2 -n ${TEST_NS} get service -o jsonpath="{.items[0].spec.ports[0].nodePort}")

run "curl ${IP2}:${PORT2}"
