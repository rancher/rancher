#!/bin/bash

# Usage: ./mirror.sh <docker hub org>

# Requires manifest-tool for inspecting from gcr.
#   GO111MODULE=off go get github.com/estesp/manifest-tool
# Can mirror existing manifests or create new manifests from template.
# Forces os, arch, & variant for platform specific tags (modifies image).
# Platform tags as REPO:OS-ARCH(-VARIANT).

set -e

ORG=${ORG:-'k3sio'}
TEMPLATE=${TEMPLATE:-'REPO-ARCH:VERSION'}
PLATFORMS=${PLATFORMS:-'linux/amd64,linux/arm64,linux/arm'}
PARALLEL=${PARALLEL:-'false'}

info() {
  echo "$@" >&2
}

if [ -z "${ORG}" ]; then
    info "Must pass in organization that the images will be pushed to using ORG env"
    exit 1
fi

lock() {
  local waiting=0
  while ! (set -o noclobber; >$1 ) 2>/dev/null; do
    [ ${waiting} = 0 ] && info "... Waiting for lock $1"; waiting=1
    sleep 1
  done
  info "... Obtained lock $1"
}
global_lock="${HOME}/.docker/mirror.lock"
lock "${global_lock}"

workspace=$(mktemp -d)
cd "${workspace}"

cleanup() {
  local code=$?
  set +e
  trap - INT TERM EXIT
  info "... Cleanup code ${code}"
  rm -rf "${workspace}" "${global_lock}"
  [ $code -ne 0 ] && kill 0
  exit ${code}
}
trap cleanup INT TERM EXIT

cleanup-mirror() {
  local code=$?
  local mirror=$1
  local mirror_lock=$2
  set +e
  [ $code -ne 0 ] && info "!!! Failed mirror ${mirror}"
  rm -f "${mirror_lock}"
  exit ${code}
}

tr_repo() {
  (tr '/' '_' | tr ':' '-') <<<$1
}

create-mirror() {
  local image=${1#docker.io/}
  local repo=${image%:*}
  local version=${image#$repo:}
  local mirror="${ORG}/$(tr_repo ${repo}):${version}"
  local mirror_repo="${ORG}/$(tr_repo ${repo})"
  local mirror_lock="${workspace}/$(tr_repo ${mirror_repo}).lock"
  local manifest_amend

  process-manifest() {
    while read -r arch_img os arch variant; do
      local tag="${mirror_repo}:${os}-${arch}"
      local annotate_args="--os ${os} --arch ${arch}"
      local platform="${os}/${arch}"
      if [ -n "${variant}" ]; then
        tag+="-${variant}"
        annotate_args+=" --variant ${variant}"
        platform+="/${variant}"
      fi

      tag-arch-img() {
        info "... Found platform ${platform} image ${arch_img}"
        docker pull -q ${arch_img} >/dev/null
        docker tag ${arch_img} ${tag} >/dev/null
      }

      set-image-platform() {
        info "... Set platform ${platform} for image ${tag}"
        mkdir -p ${tag}
        docker image save ${tag} -o ${tag}.tar >/dev/null
        for f in $(tar --list -f ${tag}.tar | grep -e '[./]json$'); do
          tar -C ${tag} -xf ${tag}.tar ${f}
          if jq '
              if has("os") then .os = "'${os}'" else . end |
              if has("architecture") then .architecture = "'${arch}'" else . end |
              if has("variant") then .variant = "'${variant}'" else . end
            ' <${tag}/${f} >${tag}/${f}.tmp 2>/dev/null; then
            mv -f ${tag}/${f}.tmp ${tag}/${f}
            tar -C ${tag} -uf ${tag}.tar ${f}
          fi
        done
        docker image load -q -i ${tag}.tar >/dev/null
        rm -rf ${tag}*
      }

      annotate-manifest() {
        info "... Annotate platform ${platform} manifest ${mirror}"
        docker push ${tag} >/dev/null
        local digest=$(docker image inspect ${tag} | jq -r '.[] | .RepoDigests[0]')
        docker manifest create ${manifest_amend} ${mirror} ${digest} >/dev/null
        docker manifest annotate ${mirror} ${digest} ${annotate_args} >/dev/null
        docker image rm -f ${arch_img} ${tag} >/dev/null
        manifest_amend='--amend'
      }

      {
        tag-arch-img
        set-image-platform
        annotate-manifest
      }

    done < <(manifest-list ${image})
    info "--- Push mirror ${mirror}"
    docker manifest push ${mirror} >/dev/null
  }

  lock "${mirror_lock}"
  (
    trap 'cleanup-mirror ${mirror} ${mirror_lock}' EXIT
    info "+++ Create mirror ${mirror}"
    process-manifest    
  )
}

manifest-inspect() {
  # docker manifest inspect $1 | \
  #     jq -r '.manifests[] | "'$1'@\(.digest) \(.platform.os) \(.platform.architecture) \(.platform.variant // "")"'
  manifest-tool inspect --raw $1 2>/dev/null | \
      jq -r '.[] | 
        select((.Platform.architecture // "") != "") |
        "'$1'@\(.Digest) \(.Platform.os) \(.Platform.architecture) \(.Platform.variant // "")"
      '
}

manifest-template() {
  local image=${1#docker.io/}
  local repo=${image%:*}
  local version=${image#$repo:}
  info "??? Generating manifest template for ${image}"
  for platform in $(tr ',' ' ' <<<${PLATFORMS}); do
    read -r os arch variant <<<$(tr '/' ' ' <<<${platform})
    local img=$(sed -e "
      s|REPO|${repo}|g;
      s|VERSION|${version}|g;
      s|OS|${os}|g;
      s|ARCH|${arch}|g;
      s|VARIANT|${variant}|g;
    " <<<${TEMPLATE})
    echo "${img} ${os} ${arch} ${variant}"
  done
}

manifest-list() {
  manifest=$(manifest-inspect $1)
  manifest=${manifest:-$(manifest-template $1)}
  echo "${manifest}"
}

create-mirrors() {
  info "+++ Create mirrors"
  local pids=()
  local code=0
  wait-pid() {
    wait $1 || code=$((code+1))
  }

  for mirror in $@; do
    [ -z "${mirror}" ] &&  continue
    create-mirror ${mirror} &
    local pid=$!
    if [[ "${PARALLEL}" = 'true' ]]; then
      pids+=(${pid})
    else
      wait-pid ${pid}
    fi
  done
  for pid in ${pids[@]}; do
    wait-pid ${pid}
  done

  info "--- Done create mirrors"
  if [ $code -ne 0 ]; then
    info "!!! Failed $code mirrors"
  fi
  return ${code}
}

clear-cache() {
  info "... Clear cache"
  rm -rf "${HOME}/.docker/manifests/"
  docker image prune -a -f >/dev/null
}

{
  clear-cache
  create-mirrors $@
}