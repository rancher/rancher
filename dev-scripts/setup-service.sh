#!/bin/bash

# For dev mode, create a dummy service pointing to the Rancher instance running on localhost.
# Allows the project resources kubernetes extension to work.
# Not needed for CI, helm or docker setups because the service is automatically created.
ip=$(ip addr show docker0 | awk '/inet /{gsub(/\/16/, ""); print $2}')

cat <<EOF | sed -e "s/{ip}/${ip}/" | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    run: port-forward
  name: port-forward
  namespace: cattle-system
spec:
  containers:
  - env:
    - name: REMOTE_HOST
      value: {ip} # localhost gateway
    - name: REMOTE_PORT
      value: "8443"
    - name: LOCAL_PORT
      value: "443"
    image: marcnuri/port-forward
    imagePullPolicy: Always
    name: port-forward
    ports:
    - containerPort: 443
      protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: rancher
  namespace: cattle-system
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 443
  selector:
    run: port-forward
  type: ClusterIP
EOF
