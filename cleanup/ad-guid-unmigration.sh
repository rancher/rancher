#!/bin/bash
# set -x
set -e

# Text to display in the banner
banner_text="This utility will go through all Rancher users and perform an Active Directory lookup using
the configured service account to get the user's distinguished name.  Next, it will perform lookups inside Rancher
for all the user's Tokens, ClusterRoleTemplateBindings, and ProjectRoleTemplateBindings.  If any of those objects,
including the user object itself are referencing a principalID based on the GUID of that user, those objects will be
updated to reference the distinguished name-based principalID (unless the utility is run with -dry-run, in that case
the only results are log messages indicating the changes that would be made by a run without that flag).

This utility will also detect and correct the case where a single ActiveDirectory GUID is mapped to multiple Rancher
users.  That condition was likely caused by a race in the original migration to use GUIDs and resulted in a second
Rancher user being created.  This caused Rancher logins to fail for the duplicated user.  The utility remedies
that situation by mapping any tokens and bindings to the original user before removing the newer user, which was
created in error.

It is also important to note that migration of ClusterRoleTemplateBindings and ProjectRoleTemplateBindings require
a delete/create operation rather than an update.  This will result in new object names for the migrated bindings.
A label with the former object name will be included in the migrated bindings.

The Rancher Agent image to be used with this utility can be found at rancher/rancher-agent:v2.7.6

It is recommended that you perform a Rancher backup prior to running this utility."

CLEAR='\033[0m'
RED='\033[0;31m'

# cluster resources, including the service account used to run the script
cluster_resources_yaml=$(cat << 'EOF'
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cattle-cleanup-sa
  namespace: cattle-system
  labels:
    rancher-cleanup: "true"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cattle-cleanup-binding
  labels:
    rancher-cleanup: "true"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cattle-cleanup-role
subjects:
  - kind: ServiceAccount
    name: cattle-cleanup-sa
    namespace: cattle-system
---
apiVersion: batch/v1
kind: Job
metadata:
  name: cattle-cleanup-job
  namespace: cattle-system
  labels:
    rancher-cleanup: "true"
spec:
  backoffLimit: 6
  completions: 1
  parallelism: 1
  selector:
  template:
    metadata:
      creationTimestamp: null
    spec:
      containers:
        - env:
            - name: AD_GUID_CLEANUP
              value: "true"
            #dryrun - name: DRY_RUN
              #dryrun value: "true"
            #deletemissing - name: AD_DELETE_MISSING_GUID_USERS
              #deletemissing value: "true"
            #- name: RANCHER_DEBUG
            #  value: "true"
          image: agent_image
          imagePullPolicy: Always
          command: ["agent"]
          name: cleanup-agent
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: OnFailure
      schedulerName: default-scheduler
      securityContext: {}
      serviceAccountName: cattle-cleanup-sa
      terminationGracePeriodSeconds: 30
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cattle-cleanup-role  
  labels:
    rancher-cleanup: "true"
rules:
  - apiGroups:
      - '*'
    resources:
      - '*'
    verbs:
      - '*'
  - nonResourceURLs:
      - '*'
    verbs:
      - '*'
EOF
)

# Agent image to use in the yaml file
agent_image="$1"

show_usage() {
  if [ -n "$1" ]; then
    echo -e "${RED}👉 $1${CLEAR}\n";
  fi
  echo "Usage: $0 AGENT_IMAGE [OPTIONS]"
  echo ""
  echo "Options:"
  echo -e "\t-h, --help              Display this help message"
  echo -e "\t-n, --dry-run           Display the resources that would be updated without making changes"
  echo -e "\t-d, --delete-missing    Permanently remove user objects whose GUID cannot be found in Active Directory"
}

display_banner() {
    local text="$1"
    local border_char="="
    local text_width=$(($(tput cols)))
    local border=$(printf "%${text_width}s" | tr " " "$border_char")

    echo "$border"
    printf "%-${text_width}s \n" "$text"
    echo "$border"
    echo "Dry run: $dry_run"
    echo "Delete missing: $delete_missing"
    echo "Agent image: $agent_image"
    if [[ "$dry_run" = true ]] && [[ "$delete_missing" = true ]]
    then
        echo "Setting the dry-run option to true overrides the delete-missing option.  NO CHANGES WILL BE MADE."
    fi
    echo "$border"
}

OPTS=$(getopt -o hnd -l help,dry-run,delete-missing -- "$@")
if [ $? != 0 ]; then
  show_usage "Invalid option"
  exit 1
fi

eval set -- "$OPTS"

dry_run=false
delete_missing=false

while true; do
  case "$1" in
    -h | --help)
      show_usage
      exit 0
      ;;
    -n | --dry-run)
      dry_run=true
      shift
      ;;
    -d | --delete-missing)
      delete_missing=true
      shift
      ;;
    --)
      shift
      break
      ;;
    *)
      show_usage "Invalid option"
      exit 1
      ;;
  esac
done

shift "$((OPTIND - 1))"
# Ensure AGENT_IMAGE is provided
if [ $# -lt 1 ]; then
  show_usage "AGENT_IMAGE is a required argument"
  exit 1
fi

display_banner "${banner_text}"

if [ "$dry_run" != true ]
then
    # Check the Rancher version before doing anything.
    # If it is v2.7.5, make it clear that configuration is not the recommended way to run this utility.
    rancher_version=$(kubectl get settings server-version --template='{{.value}}')
    if [ "$rancher_version" = "v2.7.5" ]; then
      echo -e "${RED}IT IS NOT RECOMMENDED TO RUN THIS UTILITY AGAINST RANCHER VERSION v2.7.5${CLEAR}"
      echo -e "${RED}IF RANCHER v.2.7.5 RESTARTS AFTER RUNNING THIS UTILITY, IT WILL UNDO THE EFFECTS OF THIS UTILITY.${CLEAR}"
      echo -e "${RED}IF YOU DO WANT TO RUN THIS UTILITY, IT IS RECOMMENDED THAT YOU MAKE A BACKUP PRIOR TO CONTINUING.${CLEAR}"
      read -p "Do you want to continue? (y/n): " choice
      if [[ ! $choice =~ ^[Yy]$ ]]; then
          echo "Exiting..."
          exit 0
      fi
    fi
fi


read -p "Do you want to continue? (y/n): " choice
if [[ ! $choice =~ ^[Yy]$ ]]; then
    echo "Exiting..."
    exit 0
fi

# apply the provided rancher agent
yaml=$(sed -e 's=agent_image='"$agent_image"'=' <<< $cluster_resources_yaml)

if [ "$dry_run" = true ]
then
    # Uncomment the env var for dry-run mode
    yaml=$(sed -e 's/#dryrun // ' <<< "$yaml")
elif [ "$delete_missing" = true ]
then
    # Instead uncomment the env var for missing user cleanup
    yaml=$(sed -e 's/#deletemissing // ' <<< "$yaml")
fi

echo "$yaml" | kubectl apply -f -

# Get the pod ID to tail the logs
retry_interval=1
max_retries=10
retry_count=0
pod_id=""
while [ $retry_count -lt $max_retries ]; do
    pod_id=$(kubectl --namespace=cattle-system get pod -l job-name=cattle-cleanup-job -o jsonpath="{.items[0].metadata.name}")
    if [ -n "$pod_id" ]; then
        break
    else
        sleep $retry_interval
        ((retry_count++))
    fi
done

# 600 is equal to 5 minutes, because the sleep interval is 0.5 seconds
job_start_timeout=600

declare -i count=0
until kubectl --namespace=cattle-system logs $pod_id -f
do
    if [ $count -gt $job_start_timeout ]
    then
        echo "Timeout reached, check the job by running kubectl --namespace=cattle-system get jobs"
        echo "To cleanup manually, you can run:"
        echo "  kubectl --namespace=cattle-system delete serviceaccount,job -l rancher-cleanup=true"
        echo "  kubectl delete clusterrole,clusterrolebinding -l rancher-cleanup=true"
        exit 1
    fi
    sleep 0.5
    count+=1
done

# Cleanup after it completes successfully
echo "$yaml" | kubectl delete -f -
