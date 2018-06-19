#!/bin/bash

ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/ssl/kube-ca.pem --cert /etc/kubernetes/ssl/kube-apiserver.pem --key /etc/kubernetes/ssl/kube-apiserver-key.pem "$@"
