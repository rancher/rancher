package machineprovision

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	namespace2 "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	name2 "github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	regExHyphen     = regexp.MustCompile("([a-z])([A-Z])")
	envNameOverride = map[string]string{
		"amazonec2":       "AWS",
		"rackspace":       "OS",
		"openstack":       "OS",
		"vmwarevsphere":   "VSPHERE",
		"vmwarefusion":    "FUSION",
		"vmwarevcloudair": "VCLOUDAIR",
	}
)

type driverArgs struct {
	rkev1.RKEMachineStatus

	DriverName          string
	ImageName           string
	CapiMachineName     string
	MachineName         string
	MachineNamespace    string
	MachineGVK          schema.GroupVersionKind
	ImagePullPolicy     corev1.PullPolicy
	EnvSecret           *corev1.Secret
	FilesSecret         *corev1.Secret
	CertsSecret         *corev1.Secret
	StateSecretName     string
	BootstrapSecretName string
	BootstrapRequired   bool
	Args                []string
	BackoffLimit        int32
}

func (h *handler) getArgsEnvAndStatus(infra *infraObject, args map[string]interface{}, driver string, create bool) (driverArgs, error) {
	var (
		url, hash, cloudCredentialSecretName string
		jobBackoffLimit                      int32
		filesSecret                          *corev1.Secret
	)

	if infra.data.String("spec", "providerID") != "" && !infra.data.Bool("status", "jobComplete") {
		// If the providerID is set, but jobComplete is false, then we need to re-enqueue the job so the proper status is set from that handler.
		job, err := h.getJobFromInfraMachine(infra)
		if err != nil {
			return driverArgs{}, err
		}
		h.jobController.Enqueue(infra.meta.GetNamespace(), job.Name)
		return driverArgs{}, generic.ErrSkip
	}

	nd, err := h.nodeDriverCache.Get(driver)
	if !create && apierror.IsNotFound(err) {
		url = infra.data.String("status", "driverURL")
		hash = infra.data.String("status", "driverHash")
	} else if err != nil {
		return driverArgs{}, err
	} else if !strings.HasPrefix(nd.Spec.URL, "local://") {
		url, hash, err = getDriverDownloadURL(nd)
		if err != nil {
			return driverArgs{}, err
		}
	}

	envSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name2.SafeConcatName(infra.meta.GetName(), "machine", "driver", "secret"),
			Namespace: infra.meta.GetNamespace(),
		},
		Data: getWhitelistedEnvVars(),
	}
	machine, err := rke2.GetMachineByOwner(h.machineCache, infra.meta)
	if err != nil && (create || !errors.Is(err, rke2.ErrNoMachineOwnerRef)) {
		return driverArgs{}, err
	}

	bootstrapName, cloudCredentialSecretName, secrets, err := h.getSecretData(machine, infra.data, create)
	if err != nil {
		return driverArgs{}, err
	}

	for k, v := range secrets {
		_, k = kv.RSplit(k, "-")
		envName := envNameOverride[driver]
		if envName == "" {
			envName = driver
		}
		k := strings.ToUpper(envName + "_" + regExHyphen.ReplaceAllString(k, "${1}_${2}"))
		envSecret.Data[k] = []byte(v)
	}
	secretName := rke2.MachineStateSecretName(infra.meta.GetName())

	cmd := []string{
		fmt.Sprintf("--driver-download-url=%s", url),
		fmt.Sprintf("--driver-hash=%s", hash),
		fmt.Sprintf("--secret-namespace=%s", infra.meta.GetNamespace()),
		fmt.Sprintf("--secret-name=%s", secretName),
	}

	if create {
		cmd = append(cmd, "create",
			fmt.Sprintf("--driver=%s", driver),
			fmt.Sprintf("--custom-install-script=/run/secrets/machine/value"))

		rancherCluster, err := h.rancherClusterCache.Get(infra.meta.GetNamespace(), infra.meta.GetLabels()[capi.ClusterLabelName])
		if err != nil {
			return driverArgs{}, err
		}
		// The files secret must be constructed before toArgs is called because
		// constructFilesSecret replaces file contents and creates a secret to be passed as a volume.
		filesSecret = constructFilesSecret(driver, args)
		cmd = append(cmd, toArgs(driver, args, rancherCluster.Status.ClusterName)...)
	} else {
		cmd = append(cmd, "rm", "-y")
		jobBackoffLimit = 3
	}

	certsSecret, err := h.constructCertsSecret(infra.meta.GetName(), infra.meta.GetNamespace())
	if err != nil {
		return driverArgs{}, err
	}

	// cloud-init will split the hostname on '.' and set the hostname to the first chunk. This causes an issue where all
	// nodes in a machine pool may have the same node name in Kubernetes. Converting the '.' to '-' here prevents this.
	cmd = append(cmd, strings.ReplaceAll(infra.meta.GetName(), ".", "-"))

	return driverArgs{
		DriverName:          driver,
		CapiMachineName:     machine.Name,
		MachineName:         infra.meta.GetName(),
		MachineNamespace:    infra.meta.GetNamespace(),
		MachineGVK:          infra.obj.GetObjectKind().GroupVersionKind(),
		ImageName:           settings.PrefixPrivateRegistry(settings.MachineProvisionImage.Get()),
		ImagePullPolicy:     corev1.PullAlways,
		EnvSecret:           envSecret,
		FilesSecret:         filesSecret,
		CertsSecret:         certsSecret,
		StateSecretName:     secretName,
		BootstrapSecretName: bootstrapName,
		BootstrapRequired:   create,
		Args:                cmd,
		BackoffLimit:        jobBackoffLimit,
		RKEMachineStatus: rkev1.RKEMachineStatus{
			Ready:                     infra.data.String("spec", "providerID") != "" && infra.data.Bool("status", "jobComplete"),
			DriverHash:                hash,
			DriverURL:                 url,
			CloudCredentialSecretName: cloudCredentialSecretName,
		},
	}, nil
}

func (h *handler) getBootstrapSecret(machine *capi.Machine) (string, error) {
	if machine == nil || machine.Spec.Bootstrap.ConfigRef == nil {
		return "", nil
	}

	gvk := schema.FromAPIVersionAndKind(machine.Spec.Bootstrap.ConfigRef.APIVersion,
		machine.Spec.Bootstrap.ConfigRef.Kind)
	bootstrap, err := h.dynamic.Get(gvk, machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name)
	if apierror.IsNotFound(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	d, err := data.Convert(bootstrap)
	if err != nil {
		return "", err
	}
	return d.String("status", "dataSecretName"), nil
}

func (h *handler) getSecretData(machine *capi.Machine, obj data.Object, create bool) (string, string, map[string]string, error) {
	var (
		err    error
		result = map[string]string{}
	)

	oldCredential := obj.String("status", "cloudCredentialSecretName")
	cloudCredentialSecretName := obj.String("spec", "common", "cloudCredentialSecretName")

	if machine == nil && create {
		return "", "", nil, generic.ErrSkip
	}

	if cloudCredentialSecretName == "" {
		cloudCredentialSecretName = oldCredential
	}

	if cloudCredentialSecretName != "" && machine != nil {
		secret, err := GetCloudCredentialSecret(h.secrets, machine.GetNamespace(), cloudCredentialSecretName)
		if err != nil {
			return "", "", nil, err
		}

		for k, v := range secret.Data {
			result[k] = string(v)
		}
	}

	bootstrapName, err := h.getBootstrapSecret(machine)
	if err != nil {
		return "", "", nil, err
	}

	return bootstrapName, cloudCredentialSecretName, result, nil
}

func GetCloudCredentialSecret(secrets corecontrollers.SecretCache, namespace, name string) (*corev1.Secret, error) {
	globalNS, globalName := kv.Split(name, ":")
	if globalName != "" && globalNS == namespace2.GlobalNamespace {
		return secrets.Get(globalNS, globalName)
	}
	return secrets.Get(namespace, name)
}

func toArgs(driverName string, args map[string]interface{}, clusterID string) (cmd []string) {
	if driverName == "amazonec2" {
		tagValue := fmt.Sprintf("kubernetes.io/cluster/%s,owned", clusterID)
		if tags, ok := args["tags"]; !ok || convert.ToString(tags) == "" {
			args["tags"] = tagValue
		} else {
			args["tags"] = convert.ToString(tags) + "," + tagValue
		}
	}

	for k, v := range args {
		dmField := "--" + driverName + "-" + strings.ToLower(regExHyphen.ReplaceAllString(k, "${1}-${2}"))
		if v == nil {
			continue
		}

		switch v.(type) {
		case float64:
			cmd = append(cmd, fmt.Sprintf("%s=%v", dmField, v))
		case string:
			if v.(string) != "" {
				cmd = append(cmd, fmt.Sprintf("%s=%s", dmField, v.(string)))
			}
		case bool:
			if v.(bool) {
				cmd = append(cmd, dmField)
			}
		case []interface{}:
			for _, s := range v.([]interface{}) {
				if _, ok := s.(string); ok {
					cmd = append(cmd, fmt.Sprintf("%s=%s", dmField, s.(string)))
				}
			}
		}
	}

	if driverName == "amazonec2" &&
		convert.ToString(args["securityGroup"]) != "rancher-nodes" &&
		args["securityGroupReadonly"] == nil {
		cmd = append(cmd, "--amazonec2-security-group-readonly")
	}

	sort.Strings(cmd)
	return
}

func getNodeDriverName(typeMeta meta.Type) string {
	return strings.ToLower(strings.TrimSuffix(typeMeta.GetKind(), "Machine"))
}

// getDriverDownloadURL checks for a local version of the driver to download for air-gapped installs.
// If no local version is found or CATTLE_DEV_MODE is set, then the URL from the node driver is returned.
func getDriverDownloadURL(nd *v3.NodeDriver) (string, string, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return nd.Spec.URL, nd.Spec.Checksum, nil
	}

	driverName := nd.Name
	if !strings.HasPrefix(driverName, drivers.DockerMachineDriverPrefix) {
		driverName = drivers.DockerMachineDriverPrefix + driverName
	}

	path := filepath.Join(settings.UIPath.Get(), "assets", driverName)
	if _, err := os.Stat(path); err != nil {
		return nd.Spec.URL, nd.Spec.Checksum, nil
	}

	hash, err := hashFile(path)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s/assets/%s", settings.ServerURL.Get(), driverName), hash, nil
}

func hashName(name string) string {
	b := sha256.Sum256([]byte(name))
	return hex.EncodeToString(b[:16])
}

func hashFile(path string) (string, error) {
	hasher := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func getWhitelistedEnvVars() map[string][]byte {
	result := make(map[string][]byte)
	settings.IterateWhitelistedEnvVars(func(name, value string) {
		result[name] = []byte(value)
	})
	return result
}
