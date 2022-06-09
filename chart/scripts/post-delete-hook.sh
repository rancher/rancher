#!/bin/bash

set -e

namespaces="${NAMESPACES}"
rancher_namespace="${RANCHER_NAMESPACE}"
timeout="${TIMEOUT}"
ignoreTimeoutError="${IGNORETIMEOUTERROR}"

if [[ -z ${namespaces} ]]; then
  echo "No namespace is provided."
  exit 1
fi

if [[ -z ${rancher_namespace} ]]; then
  echo "No rancher namespace is provided."
  exit 1
fi

if [[ -z ${timeout} ]]; then
  echo "No timeout value is provided."
  exit 1
fi

if [[ -z ${ignoreTimeoutError} ]]; then
  echo "No ignoreTimeoutError value is provided."
  exit 1
fi

succeeded=()
failed=()

get_pod_count() {
  kubectl get pods --selector app="${1}" -n "${2}" -o json | jq '.items | length'
}

echo "Uninstalling Rancher resources in the following namespaces: ${namespaces}"

for namespace in ${namespaces}; do
  for app in $(helm list -n "${namespace}" -q); do
    if [[ ${app} =~ .crd$ ]]; then
      echo "--- Skip the app [${app}] in the namespace [${namespace}]"
      continue
    fi
    echo "--- Deleting the app [${app}] in the namespace [${namespace}]"
    if [[ ! $(helm uninstall "${app}" -n "${namespace}") ]]; then
      failed=("${failed[@]}" "${app}")
      continue
    fi

    t=0
    while true; do
      if [[ $(get_pod_count "${app}" "${namespace}") -eq 0 ]]; then
        echo "successfully uninstalled [${app}] in the namespace [${namespace}]"
        succeeded=("${succeeded[@]}" "${app}")
        break
      fi
      if [[ ${t} -ge ${timeout} ]]; then
        echo "timeout uninstalling [${app}] in the namespace [${namespace}]"
        failed=("${failed[@]}" "${app}")
        break
      fi
      # by default, wait 120 seconds in total for an app to be uninstalled
      echo "waiting 5 seconds for pods of [${app}] to be terminated ..."
      sleep 5
      t=$((t + 5))
    done
  done

  # delete the helm operator pods
  for pod in $(kubectl get pods -n "${namespace}" -o name); do
    if [[ ${pod} =~ ^pod\/helm-operation-* ]]; then
      echo "--- Deleting the pod [${pod}] in the namespace [${namespace}]"
      kubectl delete "${pod}" -n "${namespace}"
    fi
  done
done

echo "Removing Rancher bootstrap secret in the following namespace: ${rancher_namespace}"
kubectl --ignore-not-found=true delete secret bootstrap-secret -n "${rancher_namespace}"

echo "------ Summary ------"
if [[ ${#succeeded[@]} -ne 0 ]]; then
  echo "Succeeded to uninstall the following apps:" "${succeeded[@]}"
fi

if [[ ${#failed[@]} -ne 0 ]]; then
  echo "Failed to uninstall the following apps:" "${failed[@]}"
  if [[ "${ignoreTimeoutError}" == "false" ]]; then
    exit 2
  fi
else
  echo "Cleanup finished successfully."
fi
