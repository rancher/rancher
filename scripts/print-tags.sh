#!/usr/bin/env bash

if [ -z "$1" ]; then
    master="(Rancher tag not specified)"
    tag='master'
else
    tag=$1
fi

CATTLE_VER=$(curl https://raw.githubusercontent.com/rancher/rancher/$tag/server/Dockerfile 2>/dev/null | grep 'ENV CATTLE_CATTLE_VERSION' | awk '{print $3}')

echo -e "\nPrinting project/service tag information for Rancher tag $tag. $master"

echo -e "\nCattle Tag: $CATTLE_VER\n"

echo "Other project tags:"
while read -r l; do 
    PROP=$(echo "$l" | awk -F= '{print $1}')
    URL=$(echo "$l" | awk -F= '{print $2}')
    IFS=', ' read -r -a array <<< "$(echo $l | awk -F= '{print $2}')"
    for element in "${array[@]}"
    do
        if [[ $element == http* ]]; then
            URL=$element
        fi
    done

    VER_AND_FILE=$(echo "$l" | grep "releases/download" | sed  "s/^.*download\///g")
    #echo $VER_AND_FILE
    VER=$(echo "$VER_AND_FILE" | awk -F/ '{print $1}')
    FILE=$(echo "$VER_AND_FILE" | awk -F/ '{print $2}')
    if [[ -n "$VER" ]]; then
        printf "%40s %15s %s\n" "$PROP" "$VER" "$URL"
    fi

done < <(curl -s https://raw.githubusercontent.com/rancher/cattle/$CATTLE_VER/resources/content/cattle-global.properties)

echo -e "\n\nCatalog item versions: (Pulling image may take several minutes)"
docker run --rm -it rancher/server:$tag bash -c 'for i in /var/lib/cattle/cache/global/*; do git -C $i remote -vv; git -C $i rev-parse HEAD; for j in $i/infra-templates/*; do A=$(grep version $j/config.yml 2>/dev/null | cut -d" " -f2); echo $(basename $j): $A; for x in $(grep -Irl --exclude=config.yml $A $j 2>/dev/null); do grep -rh --include="docker-compose.*" -e "^[ \t]*image: " $(dirname $x) 2>/dev/null; done; done; echo -e "\n"; done' 2>/dev/null
