#!/bin/bash
# This script defines which role this node will be and writes that to a file
# that is readable by rke2


if [ $# != 8 ]; then
  echo "Usage: define_node_roles.sh node_index role_order all_role_nodes etcd_only_nodes etcd_cp_nodes etcd_worker_nodes cp_only_nodes cp_worker_nodes"
  exit 1
fi

let node_index=$1+1
role_order=$2
all_role_nodes=$3
etcd_only_nodes=$4
etcd_cp_nodes=$5
etcd_worker_nodes=$6
cp_only_nodes=$7
cp_worker_nodes=$8

# Set the desired role into an array based on the index
order_array=($(echo "$role_order" | tr ',' '\n'))
role_array=()
for order_num in "${order_array[@]}"
do
  if [[ "$order_num" = "1" ]]
  then
    until [ $all_role_nodes -le "0" ]
    do
       role_array+=("all-roles")
       let "all_role_nodes-=1"
    done
  elif [[ "$order_num" = "2" ]]
  then
    until [ $etcd_only_nodes -le "0" ]
    do
       role_array+=("etcd-only")
       let "etcd_only_nodes-=1"
    done
  elif [[ "$order_num" = "3" ]]
  then
    until [ $etcd_cp_nodes -le "0" ]
    do
       role_array+=("etcd-cp")
       let "etcd_cp_nodes-=1"
    done
  elif [[ "$order_num" = "4" ]]
  then
    until [ $etcd_worker_nodes -le "0" ]
    do
       role_array+=("etcd-worker")
       let "etcd_worker_nodes-=1"
    done
  elif [[ "$order_num" = "5" ]]
  then
    until [ $cp_only_nodes -le "0" ]
    do
       role_array+=("cp-only")
       let "cp_only_nodes-=1"
    done
  elif [[ "$order_num" = "6" ]]
  then
    until [ $cp_worker_nodes -le "0" ]
    do
       role_array+=("cp-worker")
       let "cp_worker_nodes-=1"
    done
  fi
done

# Get role based on which node is being created
role="${role_array[$node_index]}"
echo "Writing config for a ${role} node."

# Write config
mkdir -p /etc/rancher/rke2/config.yaml.d
if [[ $role == "etcd-only" ]]
then
cat << EOF > /etc/rancher/rke2/config.yaml.d/role_config.yaml
disable-apiserver: true
disable-controller-manager: true
disable-scheduler: true
node-taint:
  - node-role.kubernetes.io/etcd:NoExecute
EOF

elif [[ $role == "etcd-cp" ]]
then
cat << EOF > /etc/rancher/rke2/config.yaml.d/role_config.yaml
node-taint:
  - node-role.kubernetes.io/control-plane:NoSchedule
  - node-role.kubernetes.io/etcd:NoExecute
EOF

elif [[ $role == "etcd-worker" ]]
then
cat << EOF > /etc/rancher/rke2/config.yaml.d/role_config.yaml
disable-apiserver: true
disable-controller-manager: true
disable-scheduler: true
EOF

elif [[ $role == "cp-only" ]]
then
cat << EOF > /etc/rancher/rke2/config.yaml.d/role_config.yaml
disable-etcd: true
node-taint:
  - node-role.kubernetes.io/control-plane:NoSchedule
EOF

elif [[ $role == "cp-worker" ]]
then
cat << EOF > /etc/rancher/rke2/config.yaml.d/role_config.yaml
disable-etcd: true
EOF
fi