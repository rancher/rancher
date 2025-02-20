package planner

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sort"
	"text/template"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

const ClusterAgentInitialCreationManifest = `
apiVersion: v1
kind: Secret
metadata:
  name: rancher-cluster-agent-genesis-helper
  namespace: kube-system
data:
  "create-cluster-agent-if-not-exists.sh": {{.ScriptB64}}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: rancher-cluster-agent-genesis
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: rancher-cluster-agent-genesis-binding
  namespace: kube-system
subjects:
- kind: ServiceAccount
  name: rancher-cluster-agent-genesis
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: rancher-cluster-agent-genesis-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: rancher-cluster-agent-genesis-role
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
---
apiVersion: batch/v1
kind: Job
metadata:
  name: rancher-cluster-agent-genesis-deploy-job
  namespace: kube-system
spec:
  backoffLimit: 10
  template:
    metadata:
       name: rancher-cluster-agent-genesis-deploy
    spec:
        tolerations:
        - operator: Exists
        hostNetwork: true
        serviceAccountName: rancher-cluster-agent-genesis
        containers:
          - name: pod
            image: {{.AgentImage}}
            env:
            - name: CATTLE_SERVER
              value: {{.URL}}
            - name: CATTLE_CA_CHECKSUM
              value: {{.CAChecksum}}
            - name: CATTLE_TOKEN
              value: {{.Token}}
            - name: CATTLE_AGENT_STRICT_VERIFY
              value: "{{.StrictVerify}}"
            command: ["/bin/sh"]
            args: ["/helper/create-cluster-agent-if-not-exists.sh"]
            volumeMounts:
            - name: secret-volume
              mountPath: /helper
        volumes:
          - name: secret-volume
            secret:
              secretName: rancher-cluster-agent-genesis-helper
        restartPolicy: Never
`

const ApplyManifestIfNotExistsScript string = `#!/bin/sh

if [ -z "$CATTLE_SERVER" ] || [ -z "$CATTLE_TOKEN" ]; then
	exit 1
fi

# info logs the given argument at info log level.
info() {
    echo "[INFO] " "$@"
}

# warn logs the given argument at warn log level.
warn() {
    echo "[WARN] " "$@" >&2
}

# error logs the given argument at error log level.
error() {
    echo "[ERROR] " "$@" >&2
}

# fatal logs the given argument at fatal log level.
fatal() {
    echo "[FATAL] " "$@" >&2
    exit 1
}

if kubectl get deployments -n cattle-system cattle-cluster-agent > /dev/null; then
	info "cattle-cluster-agent already exists"
	exit 0
fi

check_x509_cert()
{
    cert=$1
    err=$(openssl x509 -in "${cert}" -noout 2>&1)
    if [ $? -eq 0 ]
    then
        echo ""
    else
        echo "${err}"
    fi
}

ip_to_int() {
    ip_addr="${1}"

    ip_1=$(echo "${ip_addr}" | cut -d'.' -f1)
    ip_2=$(echo "${ip_addr}" | cut -d'.' -f2)
    ip_3=$(echo "${ip_addr}" | cut -d'.' -f3)
    ip_4=$(echo "${ip_addr}" | cut -d'.' -f4)

    echo $(( $ip_1 * 256*256*256 + $ip_2 * 256*256 + $ip_3 * 256 + $ip_4 ))
}

valid_ip() {
    local IP="$1" IFS="." PART
    set -- $IP
    [ "$#" != 4 ] && echo 1 && return
    for PART; do
        case "$PART" in
            *[!0-9]*) echo 1 && return
        esac
        [ "$PART" -gt 255 ] && echo 1 && return
    done
    echo 0
}

in_no_proxy() {
    # Get just the host name/IP
    ip_addr="${1#http://}"
    ip_addr="${ip_addr#https://}"
    ip_addr="${ip_addr%%/*}"
    ip_addr="${ip_addr%%:*}"

    # If this isn't an IP address, then there is nothing to check
    if [ "$(valid_ip "$ip_addr")" = "1" ]; then
      echo 1
      return
    fi

    i=1
    proxy_ip=$(echo "$NO_PROXY" | cut -d',' -f$i)
    while [ -n "$proxy_ip" ]; do
      subnet_ip=$(echo "${proxy_ip}" | cut -d'/' -f1)
      cidr_mask=$(echo "${proxy_ip}" | cut -d'/' -f2)

      if [ "$(valid_ip "$subnet_ip")" = "0" ]; then
        # If these were the same, then proxy_ip is an IP address, not a CIDR. curl handles this correctly.
        if [ "$cidr_mask" != "$subnet_ip" ]; then
          cidr_mask=$(( 32 - cidr_mask ))
          shift_multiply=1
          while [ "$cidr_mask" -gt 0 ]; do
            shift_multiply=$(( shift_multiply * 2 ))
            cidr_mask=$(( cidr_mask - 1 ))
          done

          # Manual left-shift (<<) by original cidr_mask value
          netmask=$(( 0xFFFFFFFF * shift_multiply ))

          # Apply netmask to both the subnet IP and the given IP address
          ip_addr_subnet=$(and "$(ip_to_int "$subnet_ip")" $netmask)
          subnet=$(and "$(ip_to_int "$ip_addr")" $netmask)

          # Subnet IPs will match if given IP address is in CIDR subnet
          if [ "${ip_addr_subnet}" -eq "${subnet}" ]; then
            echo 0
            return
          fi
        fi
      fi

      i=$(( i + 1 ))
      proxy_ip=$(echo "$NO_PROXY" | cut -d',' -s -f$i)
    done

    echo 1
}

validate_ca_required() {
    CA_REQUIRED=false
    if [ -n "${CATTLE_SERVER}" ]; then
        i=1
        while [ "${i}" -ne "${RETRYCOUNT}" ]; do
            VERIFY_RESULT=$(curl $noproxy --connect-timeout 60 --max-time 60 --write-out "%{ssl_verify_result}\n" ${CURL_LOG} -fL "${CATTLE_SERVER}/healthz" -o /dev/null 2>/dev/null)
            CURL_EXIT="$?"
            case "${CURL_EXIT}" in
              0|60)
                case "${VERIFY_RESULT}" in
                  0)
                    info "Determined CA is not necessary to connect to Rancher"
                    CA_REQUIRED=false
                    CATTLE_CA_CHECKSUM=""
                    break
                    ;;
                  *)
                    i=$((i + 1))
                    if [ "${CURL_EXIT}" -eq "60" ]; then
                      info "Determined CA is necessary to connect to Rancher"
                      CA_REQUIRED=true
                      break
                    fi
                    error "Error received while testing necessity of CA. Sleeping for 5 seconds and trying again"
                    sleep 5
                    continue
                    ;;
                esac
                ;;
              *)
                error "Error while connecting to Rancher to verify CA necessity. Sleeping for 5 seconds and trying again."
                sleep 5
                continue
                ;;
            esac
        done
    fi
}

RETRYCOUNT=4500
CACERTS_PATH=cacerts

validate_ca_checksum() {
    if [ -n "${CATTLE_CA_CHECKSUM}" ]; then
        CACERT=$(mktemp)
        i=1
        while [ "${i}" -ne "${RETRYCOUNT}" ]; do
            RESPONSE=$(curl $noproxy --connect-timeout 60 --max-time 60 --write-out "%{http_code}\n" --insecure ${CURL_LOG} -fL "${CATTLE_SERVER}/${CACERTS_PATH}" -o ${CACERT})
            case "${RESPONSE}" in
            200)
                info "Successfully downloaded CA certificate"
                break
                ;;
            *)
                i=$((i + 1))
                error "$RESPONSE received while downloading the CA certificate. Sleeping for 5 seconds and trying again"
                sleep 5
                continue
                ;;
            esac
        done
        if [ ! -s "${CACERT}" ]; then
          error "The environment variable CATTLE_CA_CHECKSUM is set but there is no CA certificate configured at ${CATTLE_SERVER}/${CACERTS_PATH}"
          exit 1
        fi
        err=$(check_x509_cert "${CACERT}")
        if [ -n "${err}" ]; then
            error "Value from ${CATTLE_SERVER}/${CACERTS_PATH} does not look like an x509 certificate (${err})"
            error "Retrieved cacerts:"
            cat "${CACERT}"
            rm -f "${CACERT}"
            exit 1
        else
            info "Value from ${CATTLE_SERVER}/${CACERTS_PATH} is an x509 certificate"
        fi
        CATTLE_SERVER_CHECKSUM=$(sha256sum "${CACERT}" | awk '{print $1}')
        if [ "${CATTLE_SERVER_CHECKSUM}" != "${CATTLE_CA_CHECKSUM}" ]; then
            rm -f "${CACERT}"
            error "Configured cacerts checksum ($CATTLE_SERVER_CHECKSUM) does not match given --ca-checksum ($CATTLE_CA_CHECKSUM)"
            error "Please check if the correct certificate is configured at${CATTLE_SERVER}/${CACERTS_PATH}"
            exit 1
        fi
        CURL_CAFLAG="--cacert ${CACERT}"
    fi
}

validate_rancher_connection() {
    RANCHER_SUCCESS=false
    if [ -n "${CATTLE_SERVER}" ]; then
        i=1
        while [ "${i}" -ne "${RETRYCOUNT}" ]; do
            RESPONSE=$(curl $noproxy --connect-timeout 60 --max-time 60 --write-out "%{http_code}\n" ${CURL_CAFLAG} ${CURL_LOG} -fL "${CATTLE_SERVER}/healthz" -o /dev/null)
            case "${RESPONSE}" in
            200)
                info "Successfully tested Rancher connection"
                RANCHER_SUCCESS=true
                break
                ;;
            *)
                i=$((i + 1))
                error "$RESPONSE received while testing Rancher connection. Sleeping for 5 seconds and trying again"
                sleep 5
                continue
                ;;
            esac
        done
        if [ "${RANCHER_SUCCESS}" != "true" ]; then
          fatal "Error connecting to Rancher. Perhaps --ca-checksum needs to be set?"
        fi
    fi
}

MANIFEST=$(mktemp)

i=1
noproxy=""
if [ "$(in_no_proxy "${CATTLE_SERVER}")" = "0" ]; then
	noproxy="--noproxy '*'"
fi
if [ -z "${CATTLE_CA_CHECKSUM}" ] && [ $(echo "${CATTLE_AGENT_STRICT_VERIFY}" | tr '[:upper:]' '[:lower:]') = "true" ]; then
	fatal "Aborting cluster agent installation due to requested strict CA verification with no CA checksum provided"
fi
if [ -n "${CATTLE_CA_CHECKSUM}" ] && [ $(echo "${CATTLE_AGENT_STRICT_VERIFY}" | tr '[:upper:]' '[:lower:]') != "true" ]; then
	validate_ca_required
fi
validate_ca_checksum
if [ -z "$CATTLE_CA_CHECKSUM" ] && [ -z "$CACERT" ] && [ "$(echo "${CATTLE_AGENT_STRICT_VERIFY}" | tr '[:upper:]' '[:lower:]')" = "false" ]; then
	info "No CA checksum or cert provided and strict CA verification is disabled"
	CURL_CAFLAG="-k"
fi
validate_rancher_connection

while [ "${i}" -ne "${RETRYCOUNT}" ]; do
	RESPONSE=$(curl $noproxy --connect-timeout 60 --max-time 300 --write-out "%{http_code}\n" ${CURL_CAFLAG} -fL "${CATTLE_SERVER}/v3/connect/cluster-agent/${CATTLE_TOKEN}.yaml" -o "${MANIFEST}")
	case "${RESPONSE}" in
	200)
		break
		;;
	*)
		i=$((i + 1))
		sleep 5
		continue
		;;
	esac
done

if ! kubectl create -f "$MANIFEST"; then
	info "error applying cattle-cluster-agent"
	exit 1
fi

rm $MANIFEST
`

type tc struct {
	CAChecksum   string
	Token        string
	URL          string
	ScriptB64    string
	AgentImage   string
	StrictVerify string
}

// generateClusterAgentManifest generates a cluster agent manifest
func (p *Planner) generateClusterAgentManifest(controlPlane *rkev1.RKEControlPlane) ([]byte, error) {
	if controlPlane.Spec.ManagementClusterName == "local" {
		return nil, nil
	}

	tokens, err := p.clusterRegistrationTokenCache.GetByIndex(ClusterRegToken, controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no cluster registration token found")
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].Name < tokens[j].Name
	})

	mgmtCluster, err := p.managementClusters.Get(controlPlane.Spec.ManagementClusterName)
	if err != nil {
		return nil, err
	}

	t := template.Must(template.New("cam-apply").Parse(ClusterAgentInitialCreationManifest))

	strictVerify := settings.AgentTLSMode.Get() == settings.AgentTLSModeStrict
	for _, ev := range controlPlane.Spec.AgentEnvVars {
		if ev.Name == "STRICT_VERIFY" {
			strictVerify = true
		}
	}

	templateContext := tc{
		URL:          settings.ServerURL.Get(),
		Token:        tokens[0].Status.Token,
		CAChecksum:   systemtemplate.CAChecksum(),
		ScriptB64:    base64.StdEncoding.EncodeToString([]byte(ApplyManifestIfNotExistsScript)),
		AgentImage:   systemtemplate.GetDesiredAgentImage(mgmtCluster),
		StrictVerify: fmt.Sprintf("%t", strictVerify),
	}

	buf := &bytes.Buffer{}

	err = t.Execute(buf, templateContext)
	return buf.Bytes(), err
}
