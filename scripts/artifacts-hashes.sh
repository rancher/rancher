#!/bin/bash

cd $(dirname $0)/..

dist_folder="./dist"

for filename in $dist_folder/*; do
  sum_file=$(sha256sum $filename)
  sum=$(echo $sum_file | awk '{print $1}')
  file_path=$(echo $sum_file | awk '{print $2}')
  file=${file_path#"$dist_folder/"}
  echo "$sum $file" >> ./dist/sha256sum.txt
done
