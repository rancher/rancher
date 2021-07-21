#!/bin/bash
pods=$1 # number of pods to be deployed
ns_num=$2 # number of pods per namespace/ number of namespaces
ns_name=$3 # namespace name
pod_name=$4 # Pod name
 
num=$((pods/ns_num)) #give this as an input parameter
remaining=$((pods%ns_num))

if [ $num -gt 0 ]
then
  for j in $(seq 1 $ns_num); do
    kubectl create namespace $ns_name$j
      for i in $(seq 1 $num); do
        kubectl run $pod_name$i --image=nginx -n $ns_name$j;
      done
  done
fi


if [ $remaining -gt 0 ]
then
  for k in $(seq 1 $remaining)
  do
  	kubectl run $pod_name$i --image=nginx;
  done
fi

echo Total number of pods : 
kubectl get pods -A --no-headers | wc -l