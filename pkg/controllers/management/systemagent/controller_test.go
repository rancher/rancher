package systemagent

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbaccontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDisableBeaconWait(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		beacon   *planv1alpha1.Beacon
		wantWait bool
		wantMsg  string
	}{
		{
			name: "no beacon",
		},
		{
			name: "owned by disable",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{planv1alpha1.BeaconOwnerLabel: disableBeaconOwnerKey},
				},
			},
		},
		{
			name: "owned by another controller",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{planv1alpha1.BeaconOwnerLabel: "etcd-snapshot-save"},
				},
			},
			wantWait: true,
			wantMsg:  "waiting for beacon release from \"etcd-snapshot-save\"",
		},
		{
			name: "active without owner does not block disable",
			beacon: &planv1alpha1.Beacon{
				Status: planv1alpha1.BeaconStatus{Active: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wait, message := disableBeaconWait(tt.beacon)
			if wait != tt.wantWait {
				t.Fatalf("expected wait=%t, got %t", tt.wantWait, wait)
			}
			if message != tt.wantMsg {
				t.Fatalf("expected message %q, got %q", tt.wantMsg, message)
			}
		})
	}
}

func TestShouldReconcileImportedDisable(t *testing.T) {
	t.Parallel()

	if shouldReconcileImportedDisable(nil) {
		t.Fatalf("did not expect disable reconciliation for nil annotations")
	}
	if shouldReconcileImportedDisable(map[string]string{}) {
		t.Fatalf("did not expect disable reconciliation for missing ops-enabled annotation")
	}
	if !shouldReconcileImportedDisable(map[string]string{
		day2OpsEnabledAnnotation: "false",
	}) {
		t.Fatalf("expected explicit disable annotation to trigger reconciliation")
	}
	if !shouldReconcileImportedDisable(map[string]string{
		day2OpsEnabledAnnotation:        "true",
		importedCleaningStateAnnotation: apimgmtv3.ImportedDay2OpsCleaningStateOperations,
	}) {
		t.Fatalf("expected cleaning state to keep disable reconciliation sticky")
	}
}

func TestInstallerSetsUninstallFalseEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "UNINSTALL", Value: "false"},
					{Name: "STRICT_VERIFY", Value: "custom-strict"},
				},
			},
		},
	}

	plans := renderedPlans(installer(cluster))
	if len(plans) != 2 {
		t.Fatalf("expected 2 rendered plans, got %d", len(plans))
	}
	for _, plan := range plans {
		value, ok := envVar(plan.Spec.Upgrade.Env, "UNINSTALL")
		if !ok || value != "false" {
			t.Fatalf("expected UNINSTALL=false in install plan %s", plan.Name)
		}
		if len(plan.Spec.Upgrade.Env) == 0 || plan.Spec.Upgrade.Env[0].Name != "UNINSTALL" || plan.Spec.Upgrade.Env[0].Value != "false" {
			t.Fatalf("expected UNINSTALL=false to be the first env in install plan %s", plan.Name)
		}
		uninstallCount := 0
		for _, env := range plan.Spec.Upgrade.Env {
			if env.Name == "UNINSTALL" {
				uninstallCount++
			}
		}
		if uninstallCount != 1 {
			t.Fatalf("expected exactly one UNINSTALL env in install plan %s, got %d", plan.Name, uninstallCount)
		}
		roleNoneValue, ok := envVar(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE")
		if !ok || roleNoneValue != "true" {
			t.Fatalf("expected CATTLE_ROLE_NONE=true in install plan %s", plan.Name)
		}
		if roleNoneCount := envVarCount(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
			t.Fatalf("expected exactly one CATTLE_ROLE_NONE env in install plan %s, got %d", plan.Name, roleNoneCount)
		}
		if strictValue, ok := envVar(plan.Spec.Upgrade.Env, "STRICT_VERIFY"); !ok || strictValue != "custom-strict" {
			t.Fatalf("expected STRICT_VERIFY=custom-strict in install plan %s", plan.Name)
		}
	}
	if !hasPlanName(plans, SystemAgentUpgraderPlanName) {
		t.Fatalf("expected install plan %q to be rendered", SystemAgentUpgraderPlanName)
	}
	if !hasPlanName(plans, SystemAgentUpgraderWindowsPlanName) {
		t.Fatalf("expected install plan %q to be rendered", SystemAgentUpgraderWindowsPlanName)
	}
}

func TestUninstallerSetsUninstallEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "UNINSTALL", Value: "false"},
					{Name: "STRICT_VERIFY", Value: "custom-strict"},
				},
			},
		},
	}

	plans := renderedPlans(uninstaller(cluster))
	if len(plans) != 2 {
		t.Fatalf("expected 2 rendered plans, got %d", len(plans))
	}
	for _, plan := range plans {
		value, ok := envVar(plan.Spec.Upgrade.Env, "UNINSTALL")
		if !ok || value != "true" {
			t.Fatalf("expected UNINSTALL=true in uninstall plan %s", plan.Name)
		}
		if len(plan.Spec.Upgrade.Env) == 0 || plan.Spec.Upgrade.Env[0].Name != "UNINSTALL" || plan.Spec.Upgrade.Env[0].Value != "true" {
			t.Fatalf("expected UNINSTALL=true to be the first env in uninstall plan %s", plan.Name)
		}
		uninstallCount := 0
		for _, env := range plan.Spec.Upgrade.Env {
			if env.Name == "UNINSTALL" {
				uninstallCount++
			}
		}
		if uninstallCount != 1 {
			t.Fatalf("expected exactly one UNINSTALL env in uninstall plan %s, got %d", plan.Name, uninstallCount)
		}
		roleNoneValue, ok := envVar(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE")
		if !ok || roleNoneValue != "true" {
			t.Fatalf("expected CATTLE_ROLE_NONE=true in uninstall plan %s", plan.Name)
		}
		if roleNoneCount := envVarCount(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
			t.Fatalf("expected exactly one CATTLE_ROLE_NONE env in uninstall plan %s, got %d", plan.Name, roleNoneCount)
		}
		if strictValue, ok := envVar(plan.Spec.Upgrade.Env, "STRICT_VERIFY"); !ok || strictValue != "custom-strict" {
			t.Fatalf("expected STRICT_VERIFY=custom-strict in uninstall plan %s", plan.Name)
		}
	}
	if !hasPlanName(plans, SystemAgentUpgraderPlanName) {
		t.Fatalf("expected uninstall plan %q to be rendered", SystemAgentUpgraderPlanName)
	}
	if !hasPlanName(plans, SystemAgentUpgraderWindowsPlanName) {
		t.Fatalf("expected uninstall plan %q to be rendered", SystemAgentUpgraderWindowsPlanName)
	}
}

func TestInstallerForcesRoleNoneEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "CATTLE_ROLE_NONE", Value: "false"},
				},
			},
		},
	}

	plans := renderedPlans(installer(cluster))
	if len(plans) != 2 {
		t.Fatalf("expected 2 rendered plans, got %d", len(plans))
	}
	for _, plan := range plans {
		roleNoneValue, ok := envVar(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE")
		if !ok || roleNoneValue != "true" {
			t.Fatalf("expected CATTLE_ROLE_NONE=true in install plan %s", plan.Name)
		}
		if roleNoneCount := envVarCount(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
			t.Fatalf("expected exactly one CATTLE_ROLE_NONE env in install plan %s, got %d", plan.Name, roleNoneCount)
		}
	}
}

func TestUninstallerIncludesSharedSUCObjects(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
	}

	objs := uninstaller(cluster)

	hasServiceAccount := false
	hasClusterRole := false
	hasClusterRoleBinding := false
	for _, obj := range objs {
		switch o := obj.(type) {
		case *corev1.ServiceAccount:
			hasServiceAccount = hasServiceAccount || (o.Name == SystemAgentUpgraderServiceAccountName && o.Namespace == namespaces.System)
		case *rbacv1.ClusterRole:
			hasClusterRole = hasClusterRole || (o.Name == SystemAgentUpgraderClusterRoleName)
		case *rbacv1.ClusterRoleBinding:
			hasClusterRoleBinding = hasClusterRoleBinding || (o.Name == SystemAgentUpgraderClusterRoleBindingName)
		}
	}

	if !hasServiceAccount {
		t.Fatalf("expected uninstaller to include service account %q", SystemAgentUpgraderServiceAccountName)
	}
	if !hasClusterRole {
		t.Fatalf("expected uninstaller to include cluster role %q", SystemAgentUpgraderClusterRoleName)
	}
	if !hasClusterRoleBinding {
		t.Fatalf("expected uninstaller to include cluster role binding %q", SystemAgentUpgraderClusterRoleBindingName)
	}
}

func TestPlanEnvForcesRoleNoneAndPreservesStrictVerify(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "STRICT_VERIFY", Value: "custom-strict"},
					{Name: "CATTLE_ROLE_NONE", Value: "false"},
				},
			},
		},
	}

	env := planEnv(cluster)
	if roleNoneValue, ok := envVar(env, "CATTLE_ROLE_NONE"); !ok || roleNoneValue != "true" {
		t.Fatalf("expected forced CATTLE_ROLE_NONE=true, got %q", roleNoneValue)
	}
	if roleNoneCount := envVarCount(env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
		t.Fatalf("expected exactly one CATTLE_ROLE_NONE env, got %d", roleNoneCount)
	}
	if strictValue, ok := envVar(env, "STRICT_VERIFY"); !ok || strictValue != "custom-strict" {
		t.Fatalf("expected STRICT_VERIFY=custom-strict, got %q", strictValue)
	}
}

func TestHasOperationsReturnsTrueForClusterNamespace(t *testing.T) {
	t.Parallel()

	saveCache := &fakeETCDSnapshotSaveCache{
		items: []*opv1alpha1.ETCDSnapshotSave{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-save"}},
		},
	}
	h := &handler{
		etcdSnapshotSaveCache: saveCache,
	}

	hasOps, err := h.hasOperations("c-m-test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !hasOps {
		t.Fatalf("expected hasOperations to return true")
	}
	if saveCache.lastNamespace != "c-m-test" {
		t.Fatalf("expected namespace-scoped list in c-m-test, got %q", saveCache.lastNamespace)
	}
}

func TestDeleteOperationsDeletesNamespacedOperationsAndSkipsDeleting(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	saveCache := &fakeETCDSnapshotSaveCache{
		items: []*opv1alpha1.ETCDSnapshotSave{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-save"}},
		},
	}
	restoreCache := &fakeETCDSnapshotRestoreCache{
		items: []*opv1alpha1.ETCDSnapshotRestore{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-restore", DeletionTimestamp: &now}},
		},
	}
	rotationCache := &fakeEncryptionKeyRotationCache{
		items: []*opv1alpha1.EncryptionKeyRotation{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-rotation"}},
		},
	}

	saveClient := &fakeETCDSnapshotSaveClient{}
	restoreClient := &fakeETCDSnapshotRestoreClient{}
	rotationClient := &fakeEncryptionKeyRotationClient{}

	h := &handler{
		etcdSnapshotSaveCache:    saveCache,
		etcdSnapshotRestoreCache: restoreCache,
		encryptionRotationCache:  rotationCache,
		etcdSnapshotSaves:        saveClient,
		etcdSnapshotRestores:     restoreClient,
		encryptionRotations:      rotationClient,
	}

	remaining, err := h.deleteOperations("c-m-test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !remaining {
		t.Fatalf("expected remaining=true while operations still exist or are deleting")
	}
	if len(saveClient.deleted) != 1 || saveClient.deleted[0] != (namespacedName{namespace: "c-m-test", name: "op-save"}) {
		t.Fatalf("expected save delete for c-m-test/op-save, got %+v", saveClient.deleted)
	}
	if len(rotationClient.deleted) != 1 || rotationClient.deleted[0] != (namespacedName{namespace: "c-m-test", name: "op-rotation"}) {
		t.Fatalf("expected rotation delete for c-m-test/op-rotation, got %+v", rotationClient.deleted)
	}
	if len(restoreClient.deleted) != 0 {
		t.Fatalf("expected deleting restore to be skipped, got deletes %+v", restoreClient.deleted)
	}
	if saveCache.lastNamespace != "c-m-test" || restoreCache.lastNamespace != "c-m-test" || rotationCache.lastNamespace != "c-m-test" {
		t.Fatalf("expected namespace-scoped list for all operation kinds")
	}
}

func TestDeleteOperationsReturnsFalseWhenNoOperationsExist(t *testing.T) {
	t.Parallel()

	h := &handler{
		etcdSnapshotSaveCache:    &fakeETCDSnapshotSaveCache{},
		etcdSnapshotRestoreCache: &fakeETCDSnapshotRestoreCache{},
		encryptionRotationCache:  &fakeEncryptionKeyRotationCache{},
		etcdSnapshotSaves:        &fakeETCDSnapshotSaveClient{},
		etcdSnapshotRestores:     &fakeETCDSnapshotRestoreClient{},
		encryptionRotations:      &fakeEncryptionKeyRotationClient{},
	}

	remaining, err := h.deleteOperations("c-m-empty")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if remaining {
		t.Fatalf("expected remaining=false when no operations exist")
	}
}

func TestDisableNeededReturnsTrueForLeftoverImportedPlanIdentity(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}

	h := &handler{
		beaconCache:              &fakeBeaconCache{notFound: true},
		etcdSnapshotSaveCache:    &fakeETCDSnapshotSaveCache{},
		etcdSnapshotRestoreCache: &fakeETCDSnapshotRestoreCache{},
		encryptionRotationCache:  &fakeEncryptionKeyRotationCache{},
		secretCache: &fakeSecretCache{
			items: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "c-m-test",
						Name:      "leftover-machine-plan-token",
						Labels: map[string]string{
							serviceaccounttoken.ServiceAccountSecretLabel: "leftover-machine-plan",
						},
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
		},
		serviceAccountCache: &fakeServiceAccountCache{},
		roles: &fakeRoleClient{
			items: []rbacv1.Role{
				{ObjectMeta: metav1.ObjectMeta{Name: "leftover-machine-plan", Namespace: "c-m-test"}},
			},
		},
		roleBindings: &fakeRoleBindingClient{
			items: []rbacv1.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "leftover-machine-plan", Namespace: "c-m-test"},
					Subjects: []rbacv1.Subject{
						{Kind: "ServiceAccount", Name: "leftover-machine-plan", Namespace: "c-m-test"},
					},
				},
			},
		},
	}

	needed, err := h.disableNeeded(cluster)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !needed {
		t.Fatalf("expected disableNeeded to keep reconciling for leftover imported plan identity resources")
	}
}

func TestDeleteImportedPlanIdentityDeletesOnlyServiceAccountTokenSecrets(t *testing.T) {
	t.Parallel()

	h := &handler{
		secretCache: &fakeSecretCache{
			items: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "c-m-test",
						Name:      "plan-token",
						Labels: map[string]string{
							serviceaccounttoken.ServiceAccountSecretLabel: "node-machine-plan",
						},
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "c-m-test",
						Name:      "plan-opaque",
						Labels: map[string]string{
							serviceaccounttoken.ServiceAccountSecretLabel: "node-machine-plan",
						},
					},
					Type: corev1.SecretTypeOpaque,
				},
			},
		},
		secrets:         &fakeSecretClient{},
		roleBindings:    &fakeRoleBindingClient{},
		roles:           &fakeRoleClient{},
		serviceAccounts: &fakeServiceAccountClient{},
	}

	err := h.deleteImportedPlanIdentity(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-machine-plan",
			Namespace: "c-m-test",
			Labels: map[string]string{
				capr.RoleLabel:        capr.RolePlan,
				capr.ClusterNameLabel: "c-m-test",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	secretClient := h.secrets.(*fakeSecretClient)
	if len(secretClient.deleted) != 1 || secretClient.deleted[0] != (namespacedName{namespace: "c-m-test", name: "plan-token"}) {
		t.Fatalf("expected only service-account token secret to be deleted, got %+v", secretClient.deleted)
	}
}

func renderedPlans(objs []runtime.Object) []*upgradev1.Plan {
	plans := make([]*upgradev1.Plan, 0, len(objs))
	for i := range objs {
		plan, ok := objs[i].(*upgradev1.Plan)
		if ok {
			plans = append(plans, plan)
		}
	}
	return plans
}

func envVar(env []corev1.EnvVar, name string) (string, bool) {
	for i := range env {
		if env[i].Name == name {
			return env[i].Value, true
		}
	}
	return "", false
}

func envVarCount(env []corev1.EnvVar, name string) int {
	count := 0
	for i := range env {
		if env[i].Name == name {
			count++
		}
	}
	return count
}

func hasPlanName(plans []*upgradev1.Plan, name string) bool {
	for i := range plans {
		if plans[i].Name == name {
			return true
		}
	}
	return false
}

type namespacedName struct {
	namespace string
	name      string
}

type fakeETCDSnapshotSaveCache struct {
	operationcontrollers.ETCDSnapshotSaveCache
	items           []*opv1alpha1.ETCDSnapshotSave
	err             error
	lastNamespace   string
	lastHasSelector bool
}

func (f *fakeETCDSnapshotSaveCache) List(namespace string, selector labels.Selector) ([]*opv1alpha1.ETCDSnapshotSave, error) {
	f.lastNamespace = namespace
	f.lastHasSelector = selector != nil
	return f.items, f.err
}

type fakeETCDSnapshotRestoreCache struct {
	operationcontrollers.ETCDSnapshotRestoreCache
	items           []*opv1alpha1.ETCDSnapshotRestore
	err             error
	lastNamespace   string
	lastHasSelector bool
}

func (f *fakeETCDSnapshotRestoreCache) List(namespace string, selector labels.Selector) ([]*opv1alpha1.ETCDSnapshotRestore, error) {
	f.lastNamespace = namespace
	f.lastHasSelector = selector != nil
	return f.items, f.err
}

type fakeEncryptionKeyRotationCache struct {
	operationcontrollers.EncryptionKeyRotationCache
	items           []*opv1alpha1.EncryptionKeyRotation
	err             error
	lastNamespace   string
	lastHasSelector bool
}

func (f *fakeEncryptionKeyRotationCache) List(namespace string, selector labels.Selector) ([]*opv1alpha1.EncryptionKeyRotation, error) {
	f.lastNamespace = namespace
	f.lastHasSelector = selector != nil
	return f.items, f.err
}

type fakeETCDSnapshotSaveClient struct {
	operationcontrollers.ETCDSnapshotSaveClient
	deleted []namespacedName
	err     error
}

func (f *fakeETCDSnapshotSaveClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeETCDSnapshotRestoreClient struct {
	operationcontrollers.ETCDSnapshotRestoreClient
	deleted []namespacedName
	err     error
}

func (f *fakeETCDSnapshotRestoreClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeEncryptionKeyRotationClient struct {
	operationcontrollers.EncryptionKeyRotationClient
	deleted []namespacedName
	err     error
}

func (f *fakeEncryptionKeyRotationClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeBeaconCache struct {
	plancontrollers.BeaconCache
	beacon    *planv1alpha1.Beacon
	err       error
	notFound  bool
	namespace string
	name      string
}

func (f *fakeBeaconCache) Get(namespace, name string) (*planv1alpha1.Beacon, error) {
	f.namespace = namespace
	f.name = name
	if f.notFound {
		return nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    planv1alpha1.SchemeGroupVersion.Group,
			Resource: "beacons",
		}, name)
	}
	return f.beacon, f.err
}

type fakeSecretCache struct {
	corecontrollers.SecretCache
	items []*corev1.Secret
	err   error
}

func (f *fakeSecretCache) List(string, labels.Selector) ([]*corev1.Secret, error) {
	return f.items, f.err
}

type fakeSecretClient struct {
	corecontrollers.SecretClient
	deleted []namespacedName
	err     error
}

func (f *fakeSecretClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeServiceAccountCache struct {
	corecontrollers.ServiceAccountCache
	items []*corev1.ServiceAccount
	err   error
}

func (f *fakeServiceAccountCache) List(string, labels.Selector) ([]*corev1.ServiceAccount, error) {
	return f.items, f.err
}

type fakeServiceAccountClient struct {
	corecontrollers.ServiceAccountClient
	deleted []namespacedName
	err     error
}

func (f *fakeServiceAccountClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeRoleClient struct {
	rbaccontrollers.RoleClient
	items   []rbacv1.Role
	deleted []namespacedName
	err     error
}

func (f *fakeRoleClient) List(string, metav1.ListOptions) (*rbacv1.RoleList, error) {
	return &rbacv1.RoleList{Items: f.items}, f.err
}

func (f *fakeRoleClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeRoleBindingClient struct {
	rbaccontrollers.RoleBindingClient
	items   []rbacv1.RoleBinding
	deleted []namespacedName
	err     error
}

func (f *fakeRoleBindingClient) List(string, metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
	return &rbacv1.RoleBindingList{Items: f.items}, f.err
}

func (f *fakeRoleBindingClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}
