#!/bin/bash
deployments=$1 # number of deployments to create
ns_num=$2 # Number of namespaces/ Number of deployments per namespace
ns_name=$3 # name of namespace
deployment_name=$4 # name of deployment
 
num=$((deployments/ns_num)) 
remaining=$((deployments%ns_num))

if [ $num -gt 0 ]
then
  for j in $(seq 1 $ns_num); do
    kubectl create namespace $ns_name$j
      for i in $(seq 1 $num); do
        kubectl create deployment $deployment_name$i --image=nginx -n $ns_name$j;
      done
  done
fi


if [ $remaining -gt 0 ]
then
  for k in $(seq 1 $remaining)
  do
  	kubectl create deployment $deployment_name$k --image=nginx
  done
fi

echo Total number of deployments : 
kubectl get deployments -A --no-headers | wc -l