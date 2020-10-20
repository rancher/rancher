#!/bin/sh -xe

info()
{
    echo '[INFO] ' "$@"
}

fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

get_rancherd_process_info() {
  rancherd_PID=$(ps -ef | grep -E "/usr(/local)*/bin/rancherd .*(server|agent)" | grep -E -v "(init|grep)" | awk '{print $1}')
  if [ -z "$rancherd_PID" ]; then
    fatal "rancherd is not running on this server"
  fi
  info "rancherd binary is running with pid $rancherd_PID"
  rancherd_BIN_PATH=$(cat /host/proc/${rancherd_PID}/cmdline | awk '{print $1}' | head -n 1)
  if [ -z "$rancherd_BIN_PATH" ]; then
    fatal "Failed to fetch the rancherd binary path from process $rancherd_PID"
  fi
  return
}

replace_binary() {
  NEW_BINARY="/opt/rancherd"
  FULL_BIN_PATH="/host$rancherd_BIN_PATH"
  if [ ! -f $NEW_BINARY ]; then
    fatal "The new binary $NEW_BINARY doesn't exist"
  fi
  info "Comparing old and new binaries"
  BIN_COUNT="$(sha256sum $NEW_BINARY $FULL_BIN_PATH | cut -d" " -f1 | uniq | wc -l)"
  if [ $BIN_COUNT == "1" ]; then
    info "Binary already been replaced"
    exit 0
  fi	  	
  info "Deploying new rancherd binary to $rancherd_BIN_PATH"
  cp $NEW_BINARY $FULL_BIN_PATH
  info "rancherd binary has been replaced successfully"
  return
}

kill_rancherd_process() {
    # the script sends SIGTERM to the process and let the supervisor
    # to automatically restart rancherd with the new version
    kill -SIGTERM $rancherd_PID
    info "Successfully Killed old rancherd process $rancherd_PID"
}

prepare() {
  set +e
  MASTER_PLAN=${1}
  if [ -z "$MASTER_PLAN" ]; then
    fatal "Master Plan name is not passed to the prepare step. Exiting"
  fi
  NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
  while true; do
    # make sure master plan does exist
    PLAN=$(kubectl get plan $MASTER_PLAN -o jsonpath='{.metadata.name}' -n $NAMESPACE 2>/dev/null)
    if [ -z "$PLAN" ]; then
	    info "master plan $MASTER_PLAN doesn't exist"
	    sleep 5
	    continue
    fi
    NUM_NODES=$(kubectl get plan $MASTER_PLAN -n $NAMESPACE -o json | jq '.status.applying | length')
    if [ "$NUM_NODES" == "0" ]; then
      break
    fi
    info "Waiting for all master nodes to be upgraded"
    sleep 5
  done
  verify_masters_versions
}

verify_masters_versions() {
  while true; do
    all_updated="true"
    MASTER_NODE_VERSION=$(kubectl get nodes --selector='node-role.kubernetes.io/master' -o json | jq -r '.items[].status.nodeInfo.kubeletVersion' | sort -u | tr '+' '-')
    if [ -z "$MASTER_NODE_VERSION" ]; then
      sleep 5
      continue
    fi
    if [ "$MASTER_NODE_VERSION" == "$SYSTEM_UPGRADE_PLAN_LATEST_VERSION" ]; then
        info "All master nodes has been upgraded to version to $MASTER_NODE_VERSION"
		    break
		fi
    info "Waiting for all master nodes to be upgraded to version $MODIFIED_VERSION"
	  sleep 5
	  continue
  done
}

upgrade() {
  get_rancherd_process_info
  replace_binary
  kill_rancherd_process
}

"$@"
