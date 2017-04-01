#!/bin/bash

metrics_url=http://localhost:9108/metrics

while IFS= read -r line
do
 name=$(echo $line |sed 's/\(^servers\.[^\.]*\)\.\(.*$\)/\2/'| tr '.' '_')
 echo $(echo $line |sed 's/\(^servers\.\)\([^\.]*\)\(.*$\)/\1\*\3/')
 echo "name=\"$name\""
 echo "cattle_id=\"\$1\""
 echo 
done < <(curl -s $metrics_url | grep ^#\ HELP|grep servers\. |awk '{print $6}')
