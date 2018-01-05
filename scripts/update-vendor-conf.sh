#!/bin/bash

while read l; do
	if [[ $l == github.com/rancher/* ]] && [[ $l != github.com/rancher/cattle ]]; then
		r=$(echo $l | awk '{print $1}')
		repo="https://$r.git"
		hash=$(echo $l | awk '{print $2}')

		newhash=$(git ls-remote $repo refs/heads/master | awk '{print $1}')
		if [[ $hash != $newhash ]]; then
			echo Updating $repo from $hash to $newhash
			sed -i -e "s/$hash/$newhash/g" vendor.conf
		fi
		
	fi
done <vendor.conf
