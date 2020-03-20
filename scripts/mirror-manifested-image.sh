#!/bin/bash

# This script is not part of automation. It is committed as a means to have a history of images we've manually pushed
# Usage: ./mirror-manifested-image.sh <docker hub org>

set -e -x

echo $# arguments 
if [ "$#" -ne 1 ]; then
    echo "Must pass in orginization that the images will be pushed to, ie rancher"
    exit 1
fi

ORG=$1

pull_tag_push() {
    original_repo=$1
    sha=$2
    arch=$3
    target=$4
    
    docker rmi -f ${target}-${arch}
    docker rmi -f ${original_repo}@${sha}

    docker pull ${original_repo}@${sha}
    docker tag ${original_repo}@${sha} ${target}-${arch}
    docker push ${target}-${arch}
}

annotate() {
    targ_repo_ver=$1
    arch=$2
    
    if [ "$#" -eq 2 ]; then
        docker manifest annotate ${targ_repo_ver} ${targ_repo_ver}-${arch} --arch ${arch}
    elif [ "$#" -eq 3 ]; then
        docker manifest annotate ${targ_repo_ver} ${targ_repo_ver}-${arch} --arch ${arch} --variant $3
    fi
}

#traefik:v1.7.19
orig_repo='traefik'
targ_repo_ver="$ORG/library-traefik:v1.7.19"

# cleanup
docker rmi -f ${targ_repo_ver}

pull_tag_push $orig_repo 'sha256:9f43c8af046daed133c7f5906d0835a16a5b60a4f629bccde60eb5e0a5e6683c' 'amd64' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:a70791be11b998b78e32fa6a77ce4d9615529d7d1e6d84d1e7e11dd32a070058' 'arm' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:c0b8ceccdbffd5fc697d03ebcef193bf95dc921d06070e849acd715391b171c6' 'arm64' $targ_repo_ver

docker manifest create ${targ_repo_ver} ${targ_repo_ver}-amd64  ${targ_repo_ver}-arm  ${targ_repo_ver}-arm64
annotate ${targ_repo_ver} amd64
annotate ${targ_repo_ver} arm v6
annotate ${targ_repo_ver} arm64 v8
docker manifest push -p ${targ_repo_ver}


#coredns/coredns:1.6.3
orig_repo='coredns/coredns'
targ_repo_ver="$ORG/coredns-coredns:1.6.3"

# cleanup
docker rmi -f ${targ_repo_ver}

pull_tag_push $orig_repo 'sha256:ef941660dead21452320242f6849448cf95573549f6dcdf11efc4e67fffe7314' 'amd64' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:5333c6546b4753695e9524029480864f0f8bbf97abf53c5bcffd527e4f39c088' 'arm' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:7eb40906c31a1610d9c1aeb5c818da5f68029f3e772ac226e2eac67965537017' 'arm64' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:88fe8dd89a0a4cabd599f37b63cb699f60951f9984e07f662bc8dae9774c363e' 'ppc64le' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:0f422d13366da733c3bbc249a1b4f1cc95fd36c4768483ca4f65090e3c63b9b4' 's390x' $targ_repo_ver

docker manifest create ${targ_repo_ver} ${targ_repo_ver}-amd64  ${targ_repo_ver}-arm  ${targ_repo_ver}-arm64 ${targ_repo_ver}-ppc64le ${targ_repo_ver}-s390x
annotate ${targ_repo_ver} amd64
annotate ${targ_repo_ver} arm
annotate ${targ_repo_ver} arm64
annotate ${targ_repo_ver} ppc64le 
annotate ${targ_repo_ver} s390x
docker manifest push -p ${targ_repo_ver}



#coredns/coredns:1.6.7
orig_repo='coredns/coredns'
targ_repo_ver="$ORG/coredns-coredns:1.6.7"

# cleanup
docker rmi -f ${targ_repo_ver}

pull_tag_push $orig_repo 'sha256:695a5e109604331f843d2c435f488bf3f239a88aec49112d452c1cbf87e88405' 'amd64' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:a8be13d1f9fbd24d75dbc2013bb37f810cd1aa217d135173dab6cdbef652485e' 'arm' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:a46c07fa2a502040e5e7fe0cc7169165f09f348ee178b22d1fe4aa4cb959523e' 'arm64' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:cf6908fdfa864f4243f4df8117da2b83355588e3c990d056110fc618b990980b' 'ppc64le' $targ_repo_ver
pull_tag_push $orig_repo 'sha256:fa01cc8aca57664cfd42e1a9b8188bccf1032dfc3485f0141623e461f6517a35' 's390x' $targ_repo_ver

docker manifest create ${targ_repo_ver} ${targ_repo_ver}-amd64  ${targ_repo_ver}-arm  ${targ_repo_ver}-arm64 ${targ_repo_ver}-ppc64le ${targ_repo_ver}-s390x
annotate ${targ_repo_ver} amd64
annotate ${targ_repo_ver} arm
annotate ${targ_repo_ver} arm64
annotate ${targ_repo_ver} ppc64le 
annotate ${targ_repo_ver} s390x
docker manifest push -p ${targ_repo_ver}

