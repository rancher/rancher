#!/bin/bash
num_ingress=$1 # number of ingress to be deployed
ns_num=$2 # number of ingress per namespace/number of namespace
ns_name=$3 # namespace name
ing_name=$4 # ingress name
ingress_file=$5 # Create ingress from this file

num=$((num_ingress/ns_num))
remaining=$((num_ingress%ns_num))
replace="test-ingress" # name of resource
replace2="k8test1-service" # selector or match label


if [ $num -gt 0 ]
then
  for j in $(seq 1 $ns_num); 
  do
    kubectl create namespace $ns_name$j;
	  sed -i "" "s/.*namespace.*/  namespace: $ns_name$j/" $ingress_file
    for i in $(seq 1 $num); do
      search=$replace
      replace=$ing_name$i
      search2=$replace2
      replace2=$ing_name$i
	    if [[ $search != "" && $replace != "" ]]; then
		    sed -i "" "s/${search}/${replace}/" $ingress_file
        sed -i "" "s/${search2}/${replace2}/" $ingress_file
	    fi
      kubectl apply -f $ingress_file
      search=$replace
      search2=$replace2
    done
  done
fi

sed -i "" "s/${search}/test-ingress/" $ingress_file
sed -i "" "s/${search2}/k8test1-service/" $ingress_file

if [ $remaining -gt 0 ]
then
  for k in $(seq 1 $remaining)    
  do
    sed -i "" "s/.*namespace.*/  namespace: default/" $ingress_file
  	search=$replace
    replace=$ing_name$k
    search2=$replace2
    replace2=$ing_name$i
	if [[ $search != "" && $replace != "" ]]; then
	  sed -i "" "s/${search}/${replace}/" $ingress_file
    sed -i "" "s/${search2}/${replace2}/" $ingress_file
	fi
    kubectl apply -f $ingress_file
    search=$replace
    search2=$replace2
  done
fi

sed -i "" "s/${search}/test-ingress/" $ingress_file
sed -i "" "s/${search2}/k8test1-service/" $ingress_file

echo Total number of ingresses : 
kubectl get ingress -A --no-headers | wc -l
sed -i "" "s/${search2}/k8test1-service/" $ingress_file