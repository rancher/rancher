#!/bin/bash

mkdir -p /etc/rancher/ssl/
cd /etc/rancher/ssl/
# Create 2 certificate authorities, one for passing and one for failing
openssl req -x509 -newkey rsa:4096 -sha256 -days 365 -nodes -keyout proxy-ca.key -out proxy-authentication-ca.pem -subj /CN=localhost
openssl req -x509 -newkey rsa:4096 -sha256 -days 365 -nodes -keyout proxy-failca.key -out proxy-failca.pem -subj /CN=localhost
# Create a client csr
openssl genrsa -out proxy-client.key 4096
openssl req -new -sha256 -key proxy-client.key -subj /CN=localhost -out proxy-client.csr
# Sign good client cert and bundle
openssl x509 -req -in proxy-client.csr -CA proxy-authentication-ca.pem -CAkey proxy-ca.key -CAcreateserial -out proxy-client.crt -days 365 -sha256
cat proxy-client.key proxy-client.crt proxy-authentication-ca.pem > proxy-client.pem
# Sign bad client crt
openssl x509 -req -in proxy-client.csr -CA proxy-failca.pem -CAkey proxy-failca.key -CAcreateserial -out proxy-failclient.pem -days 365 -sha256
# Cleanup
rm proxy-ca.key proxy-client.key proxy-client.csr proxy-client.crt proxy-authentication-ca.srl proxy-failca.*
