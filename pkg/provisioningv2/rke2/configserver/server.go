package configserver

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	machineIDLabel        = "rke.cattle.io/machine-id"
	machineNameLabel      = "rke.cattle.io/machine-name"
	machineNamespaceLabel = "rke.cattle.io/machine-namespace"
	planSecret            = "rke.cattle.io/plan-secret-name"
	roleLabel             = "rke.cattle.io/service-account-role"
	roleBootstrap         = "bootstrap"
	rolePlan              = "plan"
)

var (
	tokenIndex = "tokenIndex"
)

type RKE2ConfigServer struct {
	clusterTokenCache    mgmtcontroller.ClusterRegistrationTokenCache
	serviceAccountsCache corecontrollers.ServiceAccountCache
	serviceAccounts      corecontrollers.ServiceAccountClient
	secretsCache         corecontrollers.SecretCache
	secrets              corecontrollers.SecretClient
	settings             mgmtcontroller.SettingCache
	machineCache         capicontrollers.MachineCache
	machines             capicontrollers.MachineClient
	bootstrapCache       rkecontroller.RKEBootstrapCache
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
		serviceAccountsCache: clients.Core.ServiceAccount().Cache(),
		serviceAccounts:      clients.Core.ServiceAccount(),
		secretsCache:         clients.Core.Secret().Cache(),
		secrets:              clients.Core.Secret(),
		clusterTokenCache:    clients.Mgmt.ClusterRegistrationToken().Cache(),
		machineCache:         clients.CAPI.Machine().Cache(),
		machines:             clients.CAPI.Machine(),
		bootstrapCache:       clients.RKE.RKEBootstrap().Cache(),
	}
}

func (r *RKE2ConfigServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	planSecret, secret, err := r.findSA(req)
	if apierrors.IsNotFound(err) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	} else if secret == nil {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	var ca []byte
	url, pem := settings.ServerURL.Get(), settings.CACerts.Get()
	if strings.TrimSpace(pem) != "" {
		ca = []byte(pem)
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

func (r *RKE2ConfigServer) findSA(req *http.Request) (string, *corev1.Secret, error) {
	machineID := req.Header.Get(machineIDHeader)
	if machineID == "" {
		return "", nil, nil
	}

	machineNamespace, machineName, err := r.findMachineByProvisioningSA(req)
	if err != nil {
		return "", nil, err
	}
	if machineName == "" {
		machineNamespace, machineName, err = r.findMachineByClusterToken(req)
		if err != nil {
			return "", nil, err
		}
	}

	if machineName == "" {
		return "", nil, nil
	}

	if err := r.setOrUpdateMachineID(machineNamespace, machineName, machineID); err != nil {
		return "", nil, err
	}

	planSAs, err := r.serviceAccountsCache.List(machineNamespace, labels.SelectorFromSet(map[string]string{
		machineNameLabel: machineName,
	}))
	if err != nil {
		return "", nil, err
	}

	for _, planSA := range planSAs {
		planSecret, secret, err := r.getSecret(machineName, planSA)
		if err != nil || planSecret != "" {
			return planSecret, secret, err
		}
	}

	resp, err := r.serviceAccounts.Watch(machineNamespace, metav1.ListOptions{
		LabelSelector: machineNameLabel + "=" + machineName,
	})
	if err != nil {
		return "", nil, err
	}
	defer func() {
		resp.Stop()
		for range resp.ResultChan() {
		}
	}()

	for event := range resp.ResultChan() {
		if planSA, ok := event.Object.(*corev1.ServiceAccount); ok {
			planSecret, secret, err := r.getSecret(machineName, planSA)
			if err != nil || planSecret != "" {
				return planSecret, secret, err
			}
		}
	}

	return "", nil, fmt.Errorf("timeout waiting for plan")
}

func (r *RKE2ConfigServer) setOrUpdateMachineID(machineNamespace, machineName, machineID string) error {
	machine, err := r.machineCache.Get(machineNamespace, machineName)
	if err != nil {
		return err
	}

	machineID = name.SafeConcatName(machineID)

	if machine.Labels[machineIDLabel] == machineID {
		return nil
	}

	machine = machine.DeepCopy()
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}

	machine.Labels[machineIDLabel] = machineID
	_, err = r.machines.Update(machine)
	return err
}

func (r *RKE2ConfigServer) isOwnedByMachine(machineName string, sa *corev1.ServiceAccount) (bool, error) {
	for _, owner := range sa.OwnerReferences {
		if owner.Kind == "RKEBootstrap" {
			bootstrap, err := r.bootstrapCache.Get(sa.Namespace, owner.Name)
			if err != nil {
				return false, err
			}
			for _, owner := range bootstrap.OwnerReferences {
				if owner.Kind == "Machine" && owner.Name == machineName {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (r *RKE2ConfigServer) getSecret(machineName string, planSA *corev1.ServiceAccount) (string, *corev1.Secret, error) {
	if planSA.Labels[machineNameLabel] != machineName ||
		planSA.Labels[roleLabel] != rolePlan ||
		planSA.Labels[planSecret] == "" {
		return "", nil, nil
	}

	if len(planSA.Secrets) == 0 {
		return "", nil, nil
	}

	if foundParent, err := r.isOwnedByMachine(machineName, planSA); err != nil || !foundParent {
		return "", nil, err
	}

	secret, err := r.secretsCache.Get(planSA.Namespace, planSA.Secrets[0].Name)
	return planSA.Labels[planSecret], secret, err
}
