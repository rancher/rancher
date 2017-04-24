#!/bin/bash

# Location of the default metrics endpoint
metrics_url=http://localhost:9108/metrics

# Loop through 
while IFS= read -r line; do
  case "$line" in 
    *.Mean|*.Min|*.Max|*.FifteenMinuteRate|*.FiveMinuteRate|*.OneMinuteRate|*.95thPercentile)
      ## Calculations can be done by scraping server
      ;; 
    *) 
      name=$(echo $line | sed 's/\(^servers\.[^\.]*\)\.\(.*$\)/\2/' | sed 's/^/cattle_/' | sed 's/\.Count/_total/' | sed 's/\.*Count$/_total/' | tr '.' '_')      echo $(echo $line | sed 's/\(^servers\.\)\([^\.]*\)\(.*$\)/\1\*\3/')
      echo "name=\"$name\""
      echo "cattle_id=\"\$1\""
      echo
      ;;
  esac 

done < <(curl -s $metrics_url | grep ^#\ HELP | grep servers\. | awk '{print $6}')