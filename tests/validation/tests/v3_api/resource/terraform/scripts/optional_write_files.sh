#!/bin/bash
# This script pulls raw files and writes to specified locations.
# For example, it can be used to write HelmChartConfig or custom PSA files.


files=$1

if [ -n "$files" ]
then
  file_array=($(echo "$files" | tr ' ' '\n'))
  for current_file in "${file_array[@]}"; do
    file_location=$(echo "$current_file" | awk -F, '{print $1}')
    mkdir -p "$(dirname "$file_location")"

    raw_data=$(echo "$current_file" | awk -F, '{print $2}')
    curl -s "$raw_data" -o "$file_location"
  done

fi
