#!/bin/bash
set -e -x

cd  ~/go/src/github.com/rancher/rancher/tests/v2/codecoverage/ranchercover/cover

ls -al

dirlist=(`ls -p | tr '\n' ',' | tr -d '/' | sed 's/.$//'`) 
mkdir merged
go tool covdata merge -i=${dirlist}  -o merged

go tool covdata textfmt -i=merged -o profile.txt
# Output Total Coverage 
go tool cover -func=profile.txt | grep -E '^total\:' | sed -E 's/\s+/ /g'

go tool cover -html profile.txt -o merged.html