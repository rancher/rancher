package configserver

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	ConnectClusterInfo    = "/v3/connect/cluster-info"
	ConnectConfigYamlPath = "/v3/connect/config-yaml"
	ConnectAgent          = "/v3/connect/agent"
)

var (
	tokenIndex = "tokenIndex"
)

type RKE2ConfigServer struct {
	clusterTokenCache        mgmtcontroller.ClusterRegistrationTokenCache
	clusterTokens            mgmtcontroller.ClusterRegistrationTokenController
	serviceAccountsCache     corecontrollers.ServiceAccountCache
	serviceAccounts          corecontrollers.ServiceAccountClient
	secretsCache             corecontrollers.SecretCache
	secrets                  corecontrollers.SecretController
	settings                 mgmtcontroller.SettingCache
	machineCache             capicontrollers.MachineCache
	machines                 capicontrollers.MachineClient
	bootstrapCache           rkecontroller.RKEBootstrapCache
	provisioningClusterCache provisioningcontrollers.ClusterCache
	k8s                      kubernetes.Interface
}

func New(clients *wrangler.Context) *RKE2ConfigServer {
	clients.Core.Secret().Cache().AddIndexer(tokenIndex, func(obj *corev1.Secret) ([]string, error) {
		if obj.Type == corev1.SecretTypeServiceAccountToken {
			hash := sha256.Sum256(obj.Data["token"])
			return []string{base64.URLEncoding.EncodeToString(hash[:])}, nil
		}
		return nil, nil
	})

	clients.Mgmt.ClusterRegistrationToken().Cache().AddIndexer(tokenIndex,
		func(obj *v3.ClusterRegistrationToken) ([]string, error) {
			return []string{obj.Status.Token}, nil
		})

	return &RKE2ConfigServer{
		serviceAccountsCache:     clients.Core.ServiceAccount().Cache(),
		serviceAccounts:          clients.Core.ServiceAccount(),
		secretsCache:             clients.Core.Secret().Cache(),
		secrets:                  clients.Core.Secret(),
		clusterTokenCache:        clients.Mgmt.ClusterRegistrationToken().Cache(),
		clusterTokens:            clients.Mgmt.ClusterRegistrationToken(),
		machineCache:             clients.CAPI.Machine().Cache(),
		machines:                 clients.CAPI.Machine(),
		bootstrapCache:           clients.RKE.RKEBootstrap().Cache(),
		provisioningClusterCache: clients.Provisioning.Cluster().Cache(),
		k8s:                      clients.K8s,
	}
}

func (r *RKE2ConfigServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !r.secrets.Informer().HasSynced() || !r.clusterTokens.Informer().HasSynced() {
		if err := r.secrets.Informer().GetIndexer().Resync(); err != nil {
			logrus.Errorf("error re-syncing secrets informer in rke2configserver: %v", err)
		}
		if err := r.clusterTokens.Informer().GetIndexer().Resync(); err != nil {
			logrus.Errorf("error re-syncing clustertokens informer in rke2configserver: %v", err)
		}
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	planSecret, secret, err := r.findSA(req)
	if apierrors.IsNotFound(err) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	} else if secret == nil || secret.Data[corev1.ServiceAccountTokenKey] == nil {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch req.URL.Path {
	case ConnectConfigYamlPath:
		r.connectConfigYaml(planSecret, secret.Namespace, rw)
	case ConnectAgent:
		r.connectAgent(planSecret, secret, rw, req)
	case ConnectClusterInfo:
		r.connectClusterInfo(secret, rw, req)
	}
}

func (r *RKE2ConfigServer) connectAgent(planSecret string, secret *v1.Secret, rw http.ResponseWriter, req *http.Request) {
	var ca []byte
	url, pem := settings.ServerURL.Get(), settings.CACerts.Get()
	if strings.TrimSpace(pem) != "" {
		ca = []byte(pem)
	}

	if url == "" {
		pem = settings.InternalCACerts.Get()
		url = fmt.Sprintf("https://%s", req.Host)
		if strings.TrimSpace(pem) != "" {
			ca = []byte(pem)
		}
	} else if v, ok := req.Context().Value(tls.InternalAPI).(bool); ok && v {
		pem = settings.InternalCACerts.Get()
		if strings.TrimSpace(pem) != "" {
			ca = []byte(pem)
		}
	}

	kubeConfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"agent": {
				Server:                   url,
				CertificateAuthorityData: ca,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"agent": {
				Token: string(secret.Data["token"]),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"agent": {
				Cluster:  "agent",
				AuthInfo: "agent",
			},
		},
		CurrentContext: "agent",
	})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]string{
		"namespace":  secret.Namespace,
		"secretName": planSecret,
		"kubeConfig": string(kubeConfig),
	})
}

func (r *RKE2ConfigServer) connectConfigYaml(name, ns string, rw http.ResponseWriter) {
	mpSecret, err := r.getMachinePlanSecret(ns, name)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	config := make(map[string]interface{})
	if err := json.Unmarshal(mpSecret.Data[capr.RolePlan], &config); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, ok := config["files"]; !ok {
		http.Error(rw, "no files in the plan", http.StatusInternalServerError)
		return
	}

	kubernetesVersion, err := r.getClusterKubernetesVersion(mpSecret.Labels[capi.ClusterNameLabel], ns)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	var content string
	for _, f := range config["files"].([]interface{}) {
		f := f.(map[string]interface{})
		if path, ok := f["path"].(string); ok && path == fmt.Sprintf(planner.ConfigYamlFileName, capr.GetRuntime(kubernetesVersion)) {
			if _, ok := f["content"]; ok {
				content = f["content"].(string)
			}
		}
	}

	if content == "" {
		http.Error(rw, "no config content", http.StatusInternalServerError)
		return
	}

	jsonContent, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(jsonContent)
}

func (r *RKE2ConfigServer) connectClusterInfo(secret *v1.Secret, rw http.ResponseWriter, req *http.Request) {
	headers := dataFromHeaders(req)

	// expecting -H "X-Cattle-Field: kubernetesversion" -H "X-Cattle-Field: name"
	fields, ok := headers["field"]
	if !ok {
		http.Error(rw, "no field headers", http.StatusInternalServerError)
		return
	}

	castedFields, ok := fields.([]string)
	if !ok || len(castedFields) == 0 {
		http.Error(rw, "no field headers", http.StatusInternalServerError)
		return
	}

	var info = make(map[string]string)
	for _, f := range castedFields {
		switch strings.ToLower(f) {
		case "kubernetesversion":
			k8sv, err := r.infoKubernetesVersion(req.Header.Get(machineIDHeader), secret.Namespace)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
			info[f] = k8sv
		}
	}

	jsonContent, err := json.Marshal(info)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(jsonContent)
}

func (r *RKE2ConfigServer) infoKubernetesVersion(machineID, ns string) (string, error) {
	if machineID == "" {
		return "", nil
	}
	machine, err := r.findMachineByID(machineID, ns)
	if err != nil {
		return "", err
	}

	clusterName, ok := machine.Labels[capi.ClusterNameLabel]
	if !ok {
		return "", fmt.Errorf("unable to find cluster name for machine")
	}

	return r.getClusterKubernetesVersion(clusterName, ns)
}

func (r *RKE2ConfigServer) getClusterKubernetesVersion(clusterName, ns string) (string, error) {
	cluster, err := r.provisioningClusterCache.Get(ns, clusterName)
	if err != nil {
		return "", err
	}

	return cluster.Spec.KubernetesVersion, nil
}

// findSA uses the request machineID to find and deliver the plan secret name and a service account token (or an error).
func (r *RKE2ConfigServer) findSA(req *http.Request) (string, *corev1.Secret, error) {
	machineID := req.Header.Get(machineIDHeader)
	logrus.Debugf("[rke2configserver] parsed %s as machineID", machineID)
	if machineID == "" {
		return "", nil, nil
	}

	machineNamespace, machineName, err := r.findMachineByProvisioningSA(req)
	if err != nil {
		return "", nil, err
	}
	logrus.Debugf("[rke2configserver] Got %s/%s machine from provisioning SA", machineNamespace, machineName)
	if machineName == "" {
		machineNamespace, machineName, err = r.findMachineByClusterToken(req)
		if err != nil {
			return "", nil, err
		}
		logrus.Debugf("[rke2configserver] Got %s/%s machine from cluster token", machineNamespace, machineName)
	}

	if machineName == "" || machineNamespace == "" {
		return "", nil, fmt.Errorf("machine not found by request")
	}

	if err := r.setOrUpdateMachineID(machineNamespace, machineName, machineID); err != nil {
		return "", nil, err
	}

	planSAs, err := r.serviceAccountsCache.List(machineNamespace, labels.SelectorFromSet(map[string]string{
		capr.MachineNameLabel: machineName,
		capr.RoleLabel:        capr.RolePlan,
	}))
	if err != nil {
		return "", nil, err
	}

	logrus.Debugf("[rke2configserver] %s/%s listed %d planSAs", machineNamespace, machineName, len(planSAs))

	for _, planSA := range planSAs {
		if err := capr.PlanSACheck(r.bootstrapCache, machineName, planSA); err != nil {
			logrus.Errorf("[rke2configserver] error encountered when searching for checking planSA %s/%s against machine %s: %v", planSA.Namespace, planSA.Name, machineName, err)
			continue
		}
		planSecret, err := capr.GetPlanSecretName(planSA)
		if err != nil {
			logrus.Errorf("[rke2configserver] error encountered when searching for plan secret name for planSA %s/%s: %v", planSA.Namespace, planSA.Name, err)
			continue
		}
		logrus.Debugf("[rke2configserver] %s/%s plan secret was %s", machineNamespace, machineName, planSecret)
		if planSecret == "" {
			continue
		}
		tokenSecret, _, err := capr.GetPlanServiceAccountTokenSecret(r.secrets, r.k8s, planSA)
		if err != nil {
			logrus.Errorf("[rke2configserver] error encountered when searching for token secret for planSA %s/%s: %v", planSA.Namespace, planSA.Name, err)
			continue
		}
		if tokenSecret == nil {
			logrus.Debugf("[rke2configserver] %s/%s token secret for planSecret %s was nil", machineNamespace, machineName, planSecret)
			continue
		}
		logrus.Infof("[rke2configserver] %s/%s machineID: %s delivering planSecret %s with token secret %s/%s to system-agent", machineNamespace, machineName, machineID, planSecret, tokenSecret.Namespace, tokenSecret.Name)
		return planSecret, tokenSecret, err
	}

	logrus.Debugf("[rke2configserver] %s/%s watching for plan secret to become ready for consumption", machineNamespace, machineName)

	// The plan service account will likely not exist yet -- the plan service account is created by the bootstrap controller.
	respSA, err := r.serviceAccounts.Watch(machineNamespace, metav1.ListOptions{
		LabelSelector: capr.MachineNameLabel + "=" + machineName + "," + capr.RoleLabel + "=" + capr.RolePlan,
	})
	if err != nil {
		return "", nil, err
	}
	defer func() {
		respSA.Stop()
		for range respSA.ResultChan() {
		}
	}()

	// The following logic will start a watch for plan service accounts --
	// once we see the first valid plan service account come through, we then will open a watch for secrets to look for the corresponding secret for that plan service account.
	var planSA *corev1.ServiceAccount
	var planSecret string

	for event := range respSA.ResultChan() {
		var ok bool
		if planSA, ok = event.Object.(*corev1.ServiceAccount); ok {
			if err := capr.PlanSACheck(r.bootstrapCache, machineName, planSA); err != nil {
				logrus.Errorf("[rke2configserver] error encountered when searching for checking planSA %s/%s against machine %s: %v", planSA.Namespace, planSA.Name, machineName, err)
				continue
			}
			planSecret, err = capr.GetPlanSecretName(planSA)
			if err != nil {
				logrus.Errorf("[rke2configserver] error encountered when searching for plan secret name for planSA %s/%s: %v", planSA.Namespace, planSA.Name, err)
				continue
			}
			logrus.Debugf("[rke2configserver] %s/%s plan secret was %s", machineNamespace, machineName, planSecret)
			if planSecret == "" {
				continue
			}
			tokenSecret, watchable, err := capr.GetPlanServiceAccountTokenSecret(r.secrets, r.k8s, planSA)
			if err != nil || tokenSecret == nil {
				logrus.Debugf("[rke2configserver] %s/%s token secret for planSecret %s was nil or error received", machineNamespace, machineName, planSecret)
				if err != nil {
					logrus.Errorf("[rke2configserver] error encountered when searching for token secret for planSA %s/%s: %v", planSA.Namespace, planSA.Name, err)
				}
				if watchable {
					logrus.Debugf("[rke2configserver] %s/%s token secret for planSecret %s is watchable, starting secret watch to wait for token to populate", machineNamespace, machineName, planSecret)
					break
				}
				continue
			}
			logrus.Infof("[rke2configserver] %s/%s machineID: %s delivering planSecret %s with token secret %s/%s to system-agent from plan service account watch", machineNamespace, machineName, machineID, planSecret, tokenSecret.Namespace, tokenSecret.Name)
			return planSecret, tokenSecret, nil
		}
	}

	if planSecret == "" || planSA == nil {
		return "", nil, fmt.Errorf("could not start secret watch for token secret")
	}

	logrus.Debugf("[rke2configserver] %s/%s starting token secret watch for planSA %s/%s", machineNamespace, machineName, planSA.Namespace, planSA.Name)
	// start watch for the planSA corresponding secret, using a label selector.
	respSecret, err := r.secrets.Watch(machineNamespace, metav1.ListOptions{
		LabelSelector: labels.Set{
			serviceaccounttoken.ServiceAccountSecretLabel: planSA.Name,
		}.String(),
	})
	if err != nil {
		return "", nil, err
	}
	defer func() {
		respSecret.Stop()
		for range respSecret.ResultChan() {
		}
	}()
	for event := range respSecret.ResultChan() {
		if secret, ok := event.Object.(*corev1.Secret); ok {
			logrus.Infof("[rke2configserver] %s/%s machineID: %s delivering planSecret %s with token secret %s/%s to system-agent from secret watch", machineNamespace, machineName, machineID, planSecret, secret.Namespace, secret.Name)
			return planSecret, secret, nil
		}
	}

	return "", nil, fmt.Errorf("timeout waiting for plan")
}

func (r *RKE2ConfigServer) setOrUpdateMachineID(machineNamespace, machineName, machineID string) error {
	machine, err := r.machineCache.Get(machineNamespace, machineName)
	if err != nil {
		return err
	}

	if machine.Labels[capr.MachineIDLabel] == machineID {
		return nil
	}

	machine = machine.DeepCopy()
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}

	machine.Labels[capr.MachineIDLabel] = machineID
	_, err = r.machines.Update(machine)
	logrus.Debugf("[rke2configserver] %s/%s updated machine ID to %s", machineNamespace, machineName, machineID)
	return err
}

func (r *RKE2ConfigServer) getMachinePlanSecret(ns, name string) (*v1.Secret, error) {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2,
		Steps:    10,
		Cap:      2 * time.Second,
	}
	var secret *v1.Secret
	return secret, wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		secret, err = r.secretsCache.Get(name, ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return false, err // hard error out if there's a problem
			}
			return false, nil // retry if secret not found
		}

		if len(secret.Data) == 0 || string(secret.Data[capr.RolePlan]) == "" {
			return false, nil // retry if no secret Data or plan, backoff and wait for the controller
		}

		return true, nil
	})
}
