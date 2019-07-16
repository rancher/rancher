#!/bin/bash

mkdir -p /etc/rancher/ssl/
cd /etc/rancher/ssl/
# Create 2 certificate authorities, one for passing and one for failing
openssl req -x509 -newkey rsa:4096 -sha256 -days 365 -nodes -keyout ca.key -out proxy-authentication-ca.pem -subj /CN=localhost
openssl req -x509 -newkey rsa:4096 -sha256 -days 365 -nodes -keyout failca.key -out failca.pem -subj /CN=localhost
# Create a client csr
openssl genrsa -out client.key 4096
openssl req -new -sha256 -key client.key -subj /CN=localhost -out client.csr
# Sign good client cert and bundle
openssl x509 -req -in client.csr -CA proxy-authentication-ca.pem -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256
cat client.key client.crt proxy-authentication-ca.pem > client.pem
# Sign bad client crt
openssl x509 -req -in client.csr -CA failca.pem -CAkey failca.key -CAcreateserial -out failclient.pem -days 365 -sha256
# Cleanup
rm ca.key client.key client.csr client.crt proxy-authentication-ca.srl failca.*
