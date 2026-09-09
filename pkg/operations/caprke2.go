package operations

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	bootstrapv1beta2 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta2"
	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	capiv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// CAPRKE2EndpointCAPath is where the CAPRKE2 bootstrap provider writes the S3 endpoint CA it
// resolves from EndpointCASecret. Operations render the same path so their s3-endpoint-ca argument
// points at the file that is already on the node.
const CAPRKE2EndpointCAPath = "/etc/rancher/rke2/etcd-s3-ca.crt"

// CAPRKE2Adapter implements the Adapter interface for clusters provisioned via the upstream
// cluster-api-provider-rke2 (CAPRKE2) project — i.e. a CAPI `Cluster` whose `controlPlaneRef`
// targets a `controlplane.cluster.x-k8s.io/v1beta2 RKE2ControlPlane`.
//
// Differences from CAPRAdapter (which targets Rancher's own RKEControlPlane):
//   - Runtime is always RKE2 (CAPRKE2 has no k3s support), so methods that branch on distro
//     collapse to the RKE2 case.
//   - There is no `Spec.Networking.StackPreference` field; loopback is always IPv4.
//   - There is no `MachineGlobalConfig` / `MachineSelectorConfig` indirection; per-component
//     extra args come directly from `Spec.ServerConfig.KubeAPIServer/KubeControllerManager/KubeScheduler.ExtraArgs`.
//   - There is no `Spec.DataDirectories.Provisioning` override; the provisioning data dir uses
//     the standard `/var/lib/rancher/capr` default.
type CAPRKE2Adapter struct {
	cluster      *capiv1beta2.Cluster
	controlPlane *controlplanev1beta2.RKE2ControlPlane
	clients      *wrangler.CAPIContext
	// mgmtClusterName is the name of the turtles mgmt v3 Cluster shell that back-references this
	// CAPI Cluster. It is populated on the turtles-imported dispatch path (see imported.go's
	// turtlesCAPIAdapter → capiClusterAdapter chain) and is used to locate mgmt-side artifacts
	// that live under `<mgmtClusterName>` — currently just the rkev1.ETCDSnapshot CRs. Empty when
	// the adapter is created via the direct-CAPI factory (no mgmt shell in play).
	mgmtClusterName string
}

// BeaconRef returns the CAPI cluster's (namespace, name). Turtles-imported CAPRKE2 clusters
// keep every piece of cluster-scoped state — beacon, machine-plan secrets, etcd-snapshot CRs —
// in the CAPI cluster's namespace, not the mgmt-shell namespace.
func (a *CAPRKE2Adapter) BeaconRef() (string, string) {
	return a.cluster.Namespace, a.cluster.Name
}

// EtcdSnapshotNamespace returns the mgmt v3 Cluster name that back-references this CAPI Cluster
// — where snapshotbackpopulate writes rkev1.ETCDSnapshot CRs. Falls back to the CAPI Cluster's
// namespace when no mgmt shell is in play (direct-CAPI factory path), which keeps snapshot
// namespace resolution consistent with BeaconRef in that case.
func (a *CAPRKE2Adapter) EtcdSnapshotNamespace() string {
	if a.mgmtClusterName != "" {
		return a.mgmtClusterName
	}
	return a.cluster.Namespace
}

func (a *CAPRKE2Adapter) ClusterObject() (*unstructured.Unstructured, error) {
	ustr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(a.cluster)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: ustr}, nil
}

// ETCDSnapshotS3 returns the location half of the RKE2ControlPlane's etcd S3 backup config.
//
// CAPRKE2 keeps the access key, secret key, and endpoint CA in secrets referenced from the backup
// config rather than in a Rancher cloud credential, and those references cannot be expressed in
// the shared ETCDSnapshotS3 type. They are resolved in ToS3ArgsEnvAndFiles instead, so a caller
// which only needs to know *where* snapshots go can use this, and a caller which needs to reach
// the bucket must go through ToS3ArgsEnvAndFiles.
func (a *CAPRKE2Adapter) ETCDSnapshotS3() *rkev1.ETCDSnapshotS3 {
	s3 := a.etcdBackupS3()
	if s3 == nil {
		return nil
	}

	return &rkev1.ETCDSnapshotS3{
		Endpoint: s3.Endpoint,
		Bucket:   s3.Bucket,
		Region:   s3.Region,
		Folder:   s3.Folder,
		// CAPRKE2 inverts the flag: verification is opt-in, and the zero value skips it.
		SkipSSLVerify: !s3.EnforceSSLVerify,
	}
}

// ToS3ArgsEnvAndFiles renders the S3 args/env/files for the location described by s3, falling back
// to the control plane's own backup config. Unlike CAPR and imported clusters there is no cloud
// credential to resolve: the access key and secret key come from the secret named by
// S3CredentialSecret (empty when the cluster relies on IAM authentication instead), and the
// endpoint CA from the secret named by EndpointCASecret, rendered to the path CAPRKE2 itself uses
// so the argument matches the file the bootstrap provider already wrote.
func (a *CAPRKE2Adapter) ToS3ArgsEnvAndFiles(_ *corev1.Secret, s3 *rkev1.ETCDSnapshotS3, prefix string, secretKeyInEnv bool) ([]string, []string, []plan.File, error) {
	clusterS3 := a.ETCDSnapshotS3()
	if s3 == nil {
		s3 = clusterS3
	}
	if !S3Enabled(s3) {
		return nil, nil, nil, nil
	}

	target := &S3Target{
		Bucket:        s3.Bucket,
		Endpoint:      s3.Endpoint,
		Region:        s3.Region,
		Folder:        s3.Folder,
		SkipSSLVerify: s3.SkipSSLVerify,
		Retention:     s3.Retention,
	}

	if clusterS3 != nil {
		if target.Bucket == "" {
			target.Bucket = clusterS3.Bucket
		}
		if target.Endpoint == "" {
			target.Endpoint = clusterS3.Endpoint
		}
		if target.Region == "" {
			target.Region = clusterS3.Region
		}
		if target.Folder == "" {
			target.Folder = clusterS3.Folder
		}
	}

	backup := a.etcdBackupS3()

	if backup != nil && backup.S3CredentialSecret != nil {
		secret, err := a.secretForReference(backup.S3CredentialSecret)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to lookup s3 credential secret for cluster %s/%s: %w", a.cluster.Namespace, a.cluster.Name, err)
		}
		target.AccessKey = string(secret.Data["aws_access_key_id"])
		target.SecretKey = string(secret.Data["aws_secret_access_key"])
	}

	switch {
	case backup != nil && backup.EndpointCASecret != nil:
		secret, err := a.secretForReference(backup.EndpointCASecret)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to lookup s3 endpoint CA secret for cluster %s/%s: %w", a.cluster.Namespace, a.cluster.Name, err)
		}
		ca, ok := secret.Data["ca.pem"]
		if !ok {
			return nil, nil, nil, fmt.Errorf("s3 endpoint CA secret %s/%s for cluster %s/%s is missing the ca.pem key", secret.Namespace, secret.Name, a.cluster.Namespace, a.cluster.Name)
		}
		target.EndpointCAPath = CAPRKE2EndpointCAPath
		target.EndpointCAContent = base64.StdEncoding.EncodeToString(ca)
	case strings.HasSuffix(s3.EndpointCA, ".crt"), strings.HasSuffix(s3.EndpointCA, ".pem"):
		// The snapshot recorded the path of a CA placed on the node outside of CAPRKE2's control.
		// Reference it, but leave the file to whoever put it there.
		target.EndpointCAPath = s3.EndpointCA
	}

	args, env, files := RenderS3(target, prefix, secretKeyInEnv)
	return args, env, files, nil
}

// etcdBackupS3 returns the RKE2ControlPlane's etcd S3 backup config, if any.
func (a *CAPRKE2Adapter) etcdBackupS3() *controlplanev1beta2.EtcdS3 {
	return a.controlPlane.Spec.ServerConfig.Etcd.BackupConfig.S3
}

// secretForReference resolves an object reference to a secret, defaulting an empty namespace to
// the CAPI cluster's namespace.
func (a *CAPRKE2Adapter) secretForReference(ref *corev1.ObjectReference) (*corev1.Secret, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = a.cluster.Namespace
	}
	return a.clients.Core.Secret().Cache().Get(namespace, ref.Name)
}

// LoopbackAddress returns the loopback host used when constructing probes. CAPRKE2 has no
// stack-preference field so we always use IPv4. If/when CAPRKE2 adds an analogous field, mirror
// CAPRAdapter.LoopbackAddress's stack-preference handling.
func (a *CAPRKE2Adapter) LoopbackAddress(_ *corev1.Secret) string {
	return "127.0.0.1"
}

// ConfigFile returns the runtime config file on the host.
func (a *CAPRKE2Adapter) ConfigFile(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/config.yaml"
}

// ConfigDirectory returns the runtime config drop-in directory on the host.
func (a *CAPRKE2Adapter) ConfigDirectory(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/config.yaml.d"
}

// GetServerURL returns the internal/external IP of the CAPI Machine backing the given
// machine-plan secret. Mirrors CAPRAdapter.GetServerURL — see pkg/operations/capr.go:77-113.
func (a *CAPRKE2Adapter) GetServerURL(secret *corev1.Secret) string {
	if secret == nil {
		return ""
	}

	if !planv1alpha1.HasMachineLifecycleLabels(secret) {
		return ""
	}

	ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(secret, secret.Namespace, a.clients.RESTMapper)
	if err != nil {
		logrus.Errorf("error getting reference for machine lifecycle labels: %v", err)
		return ""
	}

	machine, err := a.clients.CAPI.Machine().Cache().Get(ref.Namespace, ref.Name)
	if err != nil {
		logrus.Errorf("error getting machine %s for machine lifecycle: %v", ref.Name, err)
		return ""
	}

	if len(machine.Status.Addresses) == 0 {
		return ""
	}

	// Prefer InternalIP when present; fall back to ExternalIP. Matches CAPRAdapter's behaviour.
	var address string
	for _, addr := range machine.Status.Addresses {
		if addr.Type == capiv1beta2.MachineExternalIP && address == "" {
			address = addr.Address
		} else if addr.Type == capiv1beta2.MachineInternalIP {
			address = addr.Address
		}
	}
	return address
}

// GetSupervisorPort returns the RKE2 supervisor port. CAPRKE2 is rke2-only so this is constant.
func (a *CAPRKE2Adapter) GetSupervisorPort(_ *corev1.Secret) string {
	return "9345"
}

// WaitForRegister waits for every CAPI Machine in the cluster to have a corresponding
// machine-plan secret, indicating the system-agent has registered for that machine. Mirrors
// CAPRAdapter.WaitForRegister — see pkg/operations/capr.go:122-175. Labels are identical because
// the system-agent's plan-secret labeling is operation-package-agnostic.
func (a *CAPRKE2Adapter) WaitForRegister() (bool, error) {
	labelSelector := fmt.Sprintf("%s=%s,%s=%s,%s=%s",
		planv1alpha1.ClusterLifecycleGroupLabel, capiv1beta2.GroupVersion.Group,
		planv1alpha1.ClusterLifecycleKindLabel, "Cluster",
		planv1alpha1.ClusterLifecycleNameLabel, a.cluster.Name)
	secretList, err := a.clients.Core.Secret().List(a.cluster.Namespace, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	if err != nil {
		return false, err
	}

	secrets := secretList.Items

	machines, err := a.clients.CAPI.Machine().Cache().List(a.cluster.Namespace, labels.SelectorFromSet(labels.Set{
		capiv1beta2.ClusterNameLabel: a.cluster.Name,
	}))
	if err != nil {
		return false, err
	}

	if len(secrets) != len(machines) {
		return false, nil
	}

	// Build the expected machine-name set, then verify every secret matches a unique machine.
	expectedMachines := make(map[string]bool, len(machines))
	for _, machine := range machines {
		expectedMachines[machine.Name] = true
	}

	for _, secret := range secrets {
		if secret.Labels == nil {
			return false, nil
		}
		machineName, exists := secret.Labels[planv1alpha1.MachineLifecycleNameLabel]
		if !exists || !expectedMachines[machineName] {
			return false, nil
		}
		delete(expectedMachines, machineName)
	}

	return len(expectedMachines) == 0, nil
}

// RuntimeCommand returns the runtime command used on each node. CAPRKE2 is rke2-only.
func (a *CAPRKE2Adapter) RuntimeCommand() string {
	return capr.RuntimeRKE2
}

// ServerUnit returns the systemd unit name for the RKE2 server.
func (a *CAPRKE2Adapter) ServerUnit() string {
	return "rke2-server"
}

// extraArgsFor returns the ExtraArgs slice for the named control-plane component, or nil when
// the component is unset on the RKE2ControlPlane spec. The result is passed into
// renderSecureProbe (which accepts `any`) to drive --secure-port / --tls-cert-file / --cert-dir
// extraction.
func (a *CAPRKE2Adapter) extraArgsFor(component string) []string {
	cfg := &a.controlPlane.Spec.ServerConfig
	switch component {
	case KubeAPIServerProbeName:
		if cfg.KubeAPIServer != nil {
			return cfg.KubeAPIServer.ExtraArgs
		}
	case KubeControllerManagerProbeName:
		if cfg.KubeControllerManager != nil {
			return cfg.KubeControllerManager.ExtraArgs
		}
	case KubeSchedulerProbeName:
		if cfg.KubeScheduler != nil {
			return cfg.KubeScheduler.ExtraArgs
		}
	}
	return nil
}

// RenderProbes renders the per-role probe set for a machine-plan secret. Mirrors the structure of
// CAPRAdapter.RenderProbes (pkg/operations/capr.go:193-261) but skips the renderConfig
// indirection — CAPRKE2 has no MachineGlobalConfig/MachineSelectorConfig.
//
// Probe selection rules:
//   - ETCD: always included for etcd-role secrets (runtime is always rke2).
//   - Kube apiserver/KCM/KS: included for control-plane-role secrets.
//   - Kubelet: included for everything (CAPRKE2 has no k3s etcd-only-without-kubelet case).
//   - Supervisor: included for etcd or control-plane secrets when `supervisor` is true.
//   - Calico: included only when ServerConfig.CNI is explicitly "calico" (CAPRKE2 default is
//     "canal" so usually skipped); never for Windows or etcd-only nodes.
func (a *CAPRKE2Adapter) RenderProbes(secret *corev1.Secret, supervisor bool) (map[string]plan.Probe, error) {
	var (
		probeNames []string
		probes     = map[string]plan.Probe{}
	)

	if IsEtcd(secret) {
		probeNames = append(probeNames, ETCDProbeName)
	}
	if IsControlPlane(secret) {
		probeNames = append(probeNames, KubeAPIServerProbeName, KubeControllerManagerProbeName, KubeSchedulerProbeName)
	}
	probeNames = append(probeNames, KubeletProbeName)

	if !And(IsEtcd, Not(IsControlPlane))(secret) && a.controlPlane.Spec.ServerConfig.CNI == "calico" && Not(IsWindows)(secret) {
		probeNames = append(probeNames, CalicoProbeName)
	}

	for _, probeName := range probeNames {
		probes[probeName] = AllProbes[probeName]
	}

	dataDir := a.DistroDataDirectory(secret)
	loopbackAddress := a.LoopbackAddress(secret)

	// Supervisor probe has a multi-arg URL format; build separately so the standard format
	// substitution doesn't break it. RKE2 supervisor port is always 9345.
	if supervisor && (IsEtcd(secret) || IsControlPlane(secret)) {
		supervisorProbe := AllProbes[SupervisorProbeName]
		supervisorProbe.HTTPGetAction.URL = fmt.Sprintf(supervisorProbe.HTTPGetAction.URL, loopbackAddress, 9345, capr.RuntimeRKE2)
		probes[SupervisorProbeName] = supervisorProbe
	}

	probes = InsertDataDirForProbes(dataDir, probes)

	if IsControlPlane(secret) {
		kcmProbe, err := renderSecureProbe(a.extraArgsFor(KubeControllerManagerProbeName), probes[KubeControllerManagerProbeName], dataDir, loopbackAddress, DefaultKubeControllerManagerPort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeControllerManagerProbeName] = kcmProbe

		ksProbe, err := renderSecureProbe(a.extraArgsFor(KubeSchedulerProbeName), probes[KubeSchedulerProbeName], dataDir, loopbackAddress, DefaultKubeSchedulerPort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeSchedulerProbeName] = ksProbe
	}

	probes = ReplaceURLForProbes(probes, loopbackAddress)
	return probes, nil
}

// isSuitableLeader returns true when the CAPI Machine backing the plan secret exists, is not
// deleting, has a NodeRef, and is Ready. Mirrors CAPRAdapter.isSuitableLeader.
func (a *CAPRKE2Adapter) isSuitableLeader(s *corev1.Secret) (bool, error) {
	machineName := MachineName(s)
	machine, err := a.clients.CAPI.Machine().Cache().Get(a.controlPlane.Namespace, machineName)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return machine.DeletionTimestamp == nil &&
		machine.Status.NodeRef.IsDefined() &&
		capr.Ready.IsTrue(machine), nil
}

// FindOrElectLeader picks the machine-plan secret that should lead the given operation. The
// algorithm matches CAPRAdapter.FindOrElectLeader (pkg/operations/capr.go:284-345): reuse an
// existing annotated leader if still suitable, otherwise prefer the init node, otherwise fall
// back to the first sorted candidate.
func (a *CAPRKE2Adapter) FindOrElectLeader(operation string, filter Filter) (*corev1.Secret, error) {
	secrets := a.clients.Core.Secret()
	candidates, err := plan.NewCollector(secrets, a.controlPlane, a.controlPlane.Namespace).
		WithFilter(plan.FilterFunc(filter)).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if err != nil {
		return nil, err
	}

	var (
		marked        *corev1.Secret
		markedCount   int
		markedReady   bool
		initCandidate *corev1.Secret
		fallback      *corev1.Secret
	)
	for _, secret := range candidates {
		if secret.Annotations[OperationLeaderAnnotation] == operation {
			marked = secret
			markedCount++
			if markedCount > 1 {
				return nil, fmt.Errorf("multiple machine-plan secrets marked as operation leader for %s", operation)
			}
		}

		ok, err := a.isSuitableLeader(secret)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		if marked != nil && secret.Namespace == marked.Namespace && secret.Name == marked.Name {
			markedReady = true
		}
		if initCandidate == nil && IsInitNode(secret) {
			initCandidate = secret
		}
		if fallback == nil {
			fallback = secret
		}
	}

	if marked != nil {
		if markedReady {
			return marked, nil
		}
		logrus.Warnf("[operations] %s/%s: elected leader %s is no longer suitable, re-electing", a.controlPlane.Namespace, a.controlPlane.Name, marked.Name)
		if err := a.clearLeaderAnnotation(marked, operation); err != nil {
			return nil, err
		}
	}
	if initCandidate != nil {
		return a.markLeader(initCandidate, operation)
	}
	if fallback != nil {
		return a.markLeader(fallback, operation)
	}
	return nil, nil
}

// markLeader writes the OperationLeaderAnnotation on the given secret with retry-on-conflict.
// Idempotent: if the secret is already marked for this operation, returns it unmodified.
func (a *CAPRKE2Adapter) markLeader(secret *corev1.Secret, operation string) (*corev1.Secret, error) {
	var updated *corev1.Secret
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := a.clients.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Annotations[OperationLeaderAnnotation] == operation {
			updated = s
			return nil
		}
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations[OperationLeaderAnnotation] = operation
		updated, err = a.clients.Core.Secret().Update(s)
		return err
	})
	return updated, err
}

// clearLeaderAnnotation removes the OperationLeaderAnnotation when it points at the given
// operation. No-op when it points at a different operation or is absent.
func (a *CAPRKE2Adapter) clearLeaderAnnotation(secret *corev1.Secret, operation string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := a.clients.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Annotations == nil || s.Annotations[OperationLeaderAnnotation] != operation {
			return nil
		}
		delete(s.Annotations, OperationLeaderAnnotation)
		_, err = a.clients.Core.Secret().Update(s)
		return err
	})
}

// DistroDataDirectory returns the on-host RKE2 data directory for the node backing the given
// machine-plan secret.
//
// CAPRKE2 stores the per-machine data-dir on the bootstrap object, NOT on the control-plane
// spec. Worker machines and control-plane machines reach their data-dir through different paths
// (a worker MachineDeployment can use a separate RKE2ConfigTemplate from the control plane's
// inline RKE2ConfigSpec), so we resolve it per-secret:
//
//  1. Read the machine-lifecycle labels on the secret to find the CAPI Machine.
//  2. Read the Machine's bootstrap configRef to find the bootstrap RKE2Config object.
//  3. Read `RKE2Config.Spec.AgentConfig.DataDir`.
//
// If any step fails (machine still bootstrapping, configRef not yet populated, etc.) we log and
// fall back to the runtime default `/var/lib/rancher/rke2`. The interface signature doesn't
// allow returning an error, and returning the default is the only safe behaviour for callers
// that may invoke this method during reconciles where the cluster is still settling.
func (a *CAPRKE2Adapter) DistroDataDirectory(secret *corev1.Secret) string {
	if dir := a.bootstrapDataDir(secret); dir != "" {
		return dir
	}
	return path.Join("/var/lib/rancher", capr.RuntimeRKE2)
}

// bootstrapDataDir resolves Secret → CAPI Machine → RKE2Config → AgentConfig.DataDir. Returns
// the empty string on any miss; callers should fall back to the runtime default.
func (a *CAPRKE2Adapter) bootstrapDataDir(secret *corev1.Secret) string {
	if secret == nil {
		return ""
	}
	if !planv1alpha1.HasMachineLifecycleLabels(secret) {
		return ""
	}
	ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(secret, secret.Namespace, a.clients.RESTMapper)
	if err != nil {
		logrus.Errorf("[caprke2] error resolving machine lifecycle labels on secret %s/%s: %v", secret.Namespace, secret.Name, err)
		return ""
	}

	machine, err := a.clients.CAPI.Machine().Cache().Get(ref.Namespace, ref.Name)
	if err != nil {
		// During early bootstrap the machine may not yet exist in cache; suppress NotFound
		// from logs so cold-start reconciles aren't noisy.
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("[caprke2] error fetching CAPI Machine %s/%s: %v", ref.Namespace, ref.Name, err)
		}
		return ""
	}

	configRef := machine.Spec.Bootstrap.ConfigRef
	if configRef.Name == "" || configRef.Kind == "" || configRef.APIGroup == "" {
		// Machine has not been linked to a bootstrap object yet.
		return ""
	}
	// CAPRKE2's bootstrap kind is "RKE2Config" under bootstrap.cluster.x-k8s.io. If a future
	// CAPI version returns a different kind for this machine, bail out — we cannot know how to
	// read DataDir from an arbitrary bootstrap object.
	if configRef.APIGroup != bootstrapv1beta2.GroupVersion.Group || configRef.Kind != "RKE2Config" {
		logrus.Debugf("[caprke2] machine %s/%s bootstrap configRef is %s/%s (not RKE2Config), falling back to default data-dir",
			machine.Namespace, machine.Name, configRef.APIGroup, configRef.Kind)
		return ""
	}

	// The bootstrap RKE2Config is namespaced; ContractVersionedObjectReference omits namespace
	// (assumed to be the machine's namespace per CAPI conventions).
	gvk := bootstrapv1beta2.GroupVersion.WithKind("RKE2Config")
	obj, err := a.clients.Dynamic.Get(gvk, machine.Namespace, configRef.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("[caprke2] error fetching bootstrap RKE2Config %s/%s: %v", machine.Namespace, configRef.Name, err)
		}
		return ""
	}
	ustr, ok := obj.(*unstructured.Unstructured)
	if !ok {
		logrus.Errorf("[caprke2] expected *unstructured.Unstructured for RKE2Config %s/%s, got %T", machine.Namespace, configRef.Name, obj)
		return ""
	}
	rke2Config := &bootstrapv1beta2.RKE2Config{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(ustr.Object, rke2Config); err != nil {
		logrus.Errorf("[caprke2] converting RKE2Config %s/%s from unstructured: %v", machine.Namespace, configRef.Name, err)
		return ""
	}
	return rke2Config.Spec.AgentConfig.DataDir
}

// ProvisioningDataDirectory returns the per-operation data directory. CAPRKE2 has no field for
// this on the RKE2ControlPlane spec — every CAPRKE2 cluster uses the default `/var/lib/rancher/capr`.
func (a *CAPRKE2Adapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	return "/var/lib/rancher/capr"
}

// KubectlPath returns the kubectl binary path for this cluster's runtime — RKE2 ships kubectl
// under the data-dir's bin/ subdirectory.
func (a *CAPRKE2Adapter) KubectlPath(secret *corev1.Secret) string {
	return path.Join(a.DistroDataDirectory(secret), "bin", "kubectl")
}

// KubeconfigPath returns the on-host admin kubeconfig path.
func (a *CAPRKE2Adapter) KubeconfigPath(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/rke2.yaml"
}

// PauseCluster toggles Spec.Paused on the CAPI Cluster that owns this RKE2ControlPlane. CAPI's
// Cluster name matches the RKE2ControlPlane name by convention. Mirrors CAPRAdapter.PauseCluster.
func (a *CAPRKE2Adapter) PauseCluster(pause bool) error {
	cluster, err := a.clients.CAPI.Cluster().Cache().Get(a.controlPlane.Namespace, a.controlPlane.Name)
	if err != nil {
		return err
	}
	if ptr.Equal(cluster.Spec.Paused, &pause) {
		return nil
	}
	cluster = cluster.DeepCopy()
	cluster.Spec.Paused = &pause
	_, err = a.clients.CAPI.Cluster().Update(cluster)
	return err
}
