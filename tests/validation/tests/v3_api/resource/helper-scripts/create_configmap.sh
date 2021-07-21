#!/bin/bash
configmaps=$1 # Number of configmaps to deploy
deployments_num=$2 # Number of deployments to create / Number of config maps per namespace
ns_name=$3 # number of namespaces, created while creating a deployment
deployment_name=$4 # name of the deployment
configmap_name=$5 # name of config map
filename=$6 # Create configmap from this file

num=$((configmaps/deployments_num))
remaining=$((configmaps%deployments_num))

if [ $num -gt 0 ]
then
  for j in $(seq 1 $deployments_num); do
    kubectl create namespace $ns_name$j
    kubectl create deployment $deployment_name$j --image=nginx -n $ns_name$j;
      for i in $(seq 1 $num); do
        kubectl create configmap $configmap_name$i --from-file=./$filename -n $ns_name$j;
        kubectl set env --from=configmap/$configmap_name$i deployment/$deployment_name$j -n $ns_name$j;
      done
  done
fi


if [ $remaining -gt 0 ]
then
  kubectl create deployment $deployment_name --image=nginx;
  for k in $(seq 1 $remaining)
  do  	
    kubectl create configmap $configmap_name$k --from-file=./$filename;
    kubectl set env --from=configmap/$configmap_name$k deployment/$deployment_name;
  done
fi

echo Total number of configmaps : 
kubectl get configmaps -A --no-headers | wc -l