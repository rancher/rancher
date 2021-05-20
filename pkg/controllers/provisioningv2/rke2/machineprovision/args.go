package machineprovision

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/generic"
	name2 "github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

var (
	regExHyphen     = regexp.MustCompile("([a-z])([A-Z])")
	envNameOverride = map[string]string{
		"amazonec2": "AWS",
		"rackspace": "OS",
		"openstack": "OS",
	}
)

type driverArgs struct {
	rkev1.RKEMachineStatus

	DriverName          string
	ImageName           string
	ImagePullPolicy     corev1.PullPolicy
	EnvSecret           *corev1.Secret
	StateSecretName     string
	BootstrapSecretName string
	BootstrapOptional   bool
	Args                []string
}

func MachineStateSecretName(machineName string) string {
	return name2.SafeConcatName(machineName, "machine", "state")
}

func (h *handler) getArgsEnvAndStatus(typeMeta meta.Type, meta metav1.Object, data data.Object, create bool) (driverArgs, error) {
	var (
		url, hash, cloudCredentialSecretName string
	)

	args := data.Map("spec")
	driver := getNodeDriverName(typeMeta)

	nd, err := h.nodeDriverCache.Get(driver)
	if !create && apierror.IsNotFound(err) {
		url = data.String("status", "driverURL")
		hash = data.String("status", "driverHash")
	} else if err != nil {
		return driverArgs{}, err
	} else {
		url = nd.Spec.URL
		hash = nd.Spec.Checksum
	}

	if strings.HasPrefix(url, "local://") {
		url = ""
		hash = ""
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name2.SafeConcatName(meta.GetName(), "machine", "driver", "secret"),
			Namespace: meta.GetNamespace(),
		},
		Data: map[string][]byte{},
	}

	bootstrapName, cloudCredentialSecretName, secrets, err := h.getSecretData(meta, data, create)
	if err != nil {
		return driverArgs{}, err
	}

	for k, v := range secrets {
		envName := envNameOverride[driver]
		if envName == "" {
			envName = driver
		}
		k := strings.ToUpper(envName + "_" + regExHyphen.ReplaceAllString(k, "${1}_${2}"))
		secret.Data[k] = []byte(v)
	}

	secretName := MachineStateSecretName(meta.GetName())

	cmd := []string{
		fmt.Sprintf("--driver-download-url=%s", url),
		fmt.Sprintf("--driver-hash=%s", hash),
		fmt.Sprintf("--secret-namespace=%s", meta.GetNamespace()),
		fmt.Sprintf("--secret-name=%s", secretName),
	}

	if create {
		cmd = append(cmd, "create",
			fmt.Sprintf("--driver=%s", driver),
			fmt.Sprintf("--custom-install-script=/run/secrets/machine/value"))
		cmd = append(cmd, toArgs(driver, args)...)
	} else {
		cmd = append(cmd, "rm", "-y")
	}
	cmd = append(cmd, meta.GetName())

	return driverArgs{
		DriverName:          driver,
		ImageName:           settings.PrefixPrivateRegistry(settings.MachineProvisionImage.Get()),
		ImagePullPolicy:     corev1.PullAlways,
		EnvSecret:           secret,
		StateSecretName:     secretName,
		BootstrapSecretName: bootstrapName,
		BootstrapOptional:   !create,
		Args:                cmd,

		RKEMachineStatus: rkev1.RKEMachineStatus{
			Ready:                     data.String("spec", "providerID") != "" && data.Bool("status", "jobComplete"),
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

func (h *handler) getSecretData(meta metav1.Object, obj data.Object, create bool) (string, string, map[string]string, error) {
	var (
		err     error
		machine *capi.Machine
		result  = map[string]string{}
	)

	oldCredential := obj.String("status", "cloudCredentialSecretName")
	cloudCredentialSecretName := obj.String("spec", "common", "cloudCredentialSecretName")

	for _, ref := range meta.GetOwnerReferences() {
		if ref.Kind != "Machine" {
			continue
		}

		machine, err = h.machines.Get(meta.GetNamespace(), ref.Name)
		if err != nil {
			return "", "", nil, err
		}
	}

	if machine == nil && create {
		return "", "", nil, generic.ErrSkip
	}

	if cloudCredentialSecretName == "" {
		cloudCredentialSecretName = oldCredential
	}

	if cloudCredentialSecretName != "" {
		secret, err := h.secrets.Get(meta.GetNamespace(), cloudCredentialSecretName)
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

func toArgs(driverName string, args map[string]interface{}) (cmd []string) {
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
