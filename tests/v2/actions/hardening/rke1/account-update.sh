#!/bin/bash -e

for namespace in $(kubectl get namespaces -A -o=jsonpath="{.items[*]['metadata.name']}"); do
  echo -n "Patching namespace $namespace - "; 
  kubectl patch serviceaccount default -n ${namespace} -p "$(cat account_update.yaml)"; 
done