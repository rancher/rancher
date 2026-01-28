package params

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/version"
)

type SCCOperatorParams struct {
	rancherVersion      string
	rancherGitCommit    string
	useDeployerOperator bool

	RefreshHash string

	SCCOperatorImage string
}

func ExtractSccOperatorParams() (*SCCOperatorParams, error) {
	// Allow for disabling the deployer actually deploying the operator for dev mode (running scc-operator from IDE)
	useDeployerOperator := true
	if GetBuiltinDisabledEnv() {
		useDeployerOperator = false
	}

	// TODO: this may need to take some input to get current state
	params := &SCCOperatorParams{
		useDeployerOperator: useDeployerOperator,
		rancherVersion:      version.Version,
		rancherGitCommit:    version.GitCommit,
		SCCOperatorImage:    settings.FullSCCOperatorImage(),
	}
	if err := params.setConfigHash(); err != nil {
		return nil, err
	}

	return params, nil
}

// setConfigHash generates a hash based on the relevant configuration details
func (p *SCCOperatorParams) setConfigHash() error {
	var hashInputData []byte
	hasher := md5.New()
	if p.useDeployerOperator {
		hashInputData = append(hashInputData, byte(1))
	}
	hashInputData = append(hashInputData, []byte(p.rancherVersion)...)
	hashInputData = append(hashInputData, []byte(p.rancherGitCommit)...)

	podSpecBytes, err := json.Marshal(p.preparePodSpec())
	if err != nil {
		hashInputData = append(hashInputData, []byte(p.SCCOperatorImage)...)
	} else {
		hashInputData = append(hashInputData, podSpecBytes...)
	}

	// Generate the hash...
	if _, err := hasher.Write(hashInputData); err != nil {
		return err
	}
	p.RefreshHash = hex.EncodeToString(hasher.Sum(nil))

	return nil
}

func (p *SCCOperatorParams) baseLabels() map[string]string {
	return map[string]string{
		consts.LabelK8sManagedBy:              "rancher",
		consts.LabelK8sPartOf:                 "rancher",
		consts.LabelK8sManagedBy + "-version": p.rancherVersion,
	}
}

func (p *SCCOperatorParams) targetLabels(target LabelTargets) map[string]string {
	switch target {
	case TargetSelector:
		return map[string]string{
			consts.LabelK8sPartOf:    "rancher",
			consts.LabelK8sName:      "rancher-scc-operator",
			consts.LabelK8sComponent: "scc-operator",
		}
	case TargetPod:
		return map[string]string{
			consts.LabelK8sName:      "rancher-scc-operator",
			consts.LabelK8sComponent: "scc-operator",
		}
	default:
		return map[string]string{}
	}
}

type LabelTargets string

const (
	TargetNamespace          LabelTargets = "namespace"
	TargetServiceAccount     LabelTargets = "service-account"
	TargetClusterRoleBinding LabelTargets = "cluster-role-binding"
	TargetDeployment         LabelTargets = "deployment"
	TargetSelector           LabelTargets = "selector"
	TargetPod                LabelTargets = "pod"
)

func (p *SCCOperatorParams) Labels(target LabelTargets) map[string]string {
	targetLabels := p.targetLabels(target)
	if target == TargetSelector {
		return targetLabels
	}
	defaultLabels := p.baseLabels()

	maps.Copy(defaultLabels, targetLabels)

	return defaultLabels
}

func (p *SCCOperatorParams) PrepareDeployment() *appsv1.Deployment {
	deploymentLabels := p.Labels(TargetDeployment)
	deploymentLabels[consts.LabelSccOperatorHash] = p.RefreshHash
	var scale int32 = 1
	if !p.useDeployerOperator {
		scale = 0
	}

	// TODO: We should support the "extra tolerations" feature users are asking for
	// ref: https://github.com/rancher/rancher/issues/48541
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: consts.DefaultSCCNamespace,
			Name:      consts.DeploymentName,
			Labels:    deploymentLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &scale,
			Selector: &metav1.LabelSelector{
				MatchLabels: p.Labels(TargetSelector),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: p.Labels(TargetPod),
				},
				Spec: p.preparePodSpec(),
			},
		},
	}
}

func (p *SCCOperatorParams) preparePodSpec() corev1.PodSpec {
	// TODO: should pass in some relevant ENVs to the container
	t := true
	u1000 := int64(1000)
	return corev1.PodSpec{
		ServiceAccountName: consts.ServiceAccountName,
		Containers: []corev1.Container{
			{
				Name:            "scc-operator",
				Image:           p.SCCOperatorImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:   &t,
					RunAsGroup:     &u1000,
					RunAsUser:      &u1000,
					SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
			},
		},
	}
}
