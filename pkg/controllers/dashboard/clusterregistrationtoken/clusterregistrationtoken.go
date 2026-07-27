package clusterregistrationtoken

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	v32 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbaccontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

const (
	crtTokenReaderRole = "crt-token-reader"
	// MinTTLMinutes is the minimum allowed TTL for CRT tokens in minutes
	MinTTLMinutes = 30 // 30 minutes
	// MinGracePeriodMinutes is the minimum allowed grace period for CRT token rotation in minutes
	MinGracePeriodMinutes = 10 // 10 minutes
)

type handler struct {
	clusterRegistrationTokenCache      v32.ClusterRegistrationTokenCache
	clusterRegistrationTokenController v32.ClusterRegistrationTokenController
	clusters                           v32.ClusterCache
	secrets                            corecontrollers.SecretClient
	secretCache                        corecontrollers.SecretCache
	roles                              rbaccontrollers.RoleClient
	roleCache                          rbaccontrollers.RoleCache
	namespaceCache                     corecontrollers.NamespaceCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterRegistrationTokenController: clients.Mgmt.ClusterRegistrationToken(),
		clusterRegistrationTokenCache:      clients.Mgmt.ClusterRegistrationToken().Cache(),
		clusters:                           clients.Mgmt.Cluster().Cache(),
		secrets:                            clients.Core.Secret(),
		secretCache:                        clients.Core.Secret().Cache(),
		roles:                              clients.RBAC.Role(),
		roleCache:                          clients.RBAC.Role().Cache(),
		namespaceCache:                     clients.Core.Namespace().Cache(),
	}
	clients.Mgmt.ClusterRegistrationToken().OnChange(ctx, "cluster-registration-token", h.onChange)
	clients.Mgmt.ClusterRegistrationToken().OnRemove(ctx, "cluster-registration-token-cleanup", h.onRemove)
	clients.Mgmt.Cluster().OnChange(ctx, "cluster-registration-token-trigger", h.onClusterChange)
	clients.Mgmt.Feature().OnChange(ctx, "crt-ttl-feature-trigger", h.onFeatureChange)
	clients.Mgmt.Setting().OnChange(ctx, "crt-ttl-setting-trigger", h.onSettingChange)
}

func (h *handler) onClusterChange(key string, obj *v3.Cluster) (*v3.Cluster, error) {
	if obj == nil {
		return obj, nil
	}

	crts, err := h.clusterRegistrationTokenCache.List(obj.Name, labels.Everything())
	if err != nil {
		return obj, nil
	}

	for _, crt := range crts {
		h.clusterRegistrationTokenController.Enqueue(crt.Namespace, crt.Name)
	}

	return obj, nil
}

// onFeatureChange re-enqueues all CRTs without an ExpiresAt when the
// CRTTokenTTLRotation feature flag changes, so they pick up the new state.
func (h *handler) onFeatureChange(_ string, obj *v3.Feature) (*v3.Feature, error) {
	if obj == nil || obj.Name != features.CRTTokenTTLRotation.Name() {
		return obj, nil
	}
	return obj, h.enqueueCRTsWithoutExpiry()
}

// onSettingChange re-enqueues all CRTs without an ExpiresAt when the default
// CRT TTL setting changes, so they pick up the newly configured TTL.
func (h *handler) onSettingChange(_ string, obj *v3.Setting) (*v3.Setting, error) {
	if obj == nil || obj.Name != settings.CRTDefaultTTL.Name {
		return obj, nil
	}
	return obj, h.enqueueCRTsWithoutExpiry()
}

// enqueueCRTsWithoutExpiry enqueues all CRTs that have no ExpiresAt set.
// This is used when the feature flag or TTL setting changes, to ensure existing
// CRTs without a scheduled expiry are reconciled and get ExpiresAt assigned.
// No-ops if the TTL rotation feature flag is not enabled.
func (h *handler) enqueueCRTsWithoutExpiry() error {
	if !h.isTTLRotationEnabled() {
		return nil
	}
	crts, err := h.clusterRegistrationTokenCache.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, crt := range crts {
		if crt.Status.ExpiresAt == "" {
			h.clusterRegistrationTokenController.Enqueue(crt.Namespace, crt.Name)
		}
	}
	return nil
}

// If no CRT Secrets remain in the namespace, the Role is deleted.
func (h *handler) onRemove(_ string, obj *v3.ClusterRegistrationToken) (*v3.ClusterRegistrationToken, error) {
	if obj == nil {
		return obj, nil
	}

	if err := h.ensureCRTTokenReaderRole(obj.Namespace); err != nil {
		return nil, err
	}

	return obj, nil
}

func (h *handler) onChange(key string, obj *v3.ClusterRegistrationToken) (_ *v3.ClusterRegistrationToken, err error) {
	if obj == nil {
		return obj, nil
	}

	// Case 1: New CRT - TokenSecretName not set, generate token
	if obj.Status.TokenSecretName == "" {
		if obj.Status.Token != "" {
			logrus.Warnf("CRT %s/%s has Status.Token set but no TokenSecretName - this should have been migrated. Migrating now as fallback.",
				obj.Namespace, obj.Name)
			return h.migrateCRTFallback(obj)
		}

		// New CRT - generate token and create secret
		return h.createNewCRT(obj)
	}

	// Handle TTL rotation and status updates
	return h.reconcileExistingCRT(obj)
}

// migrateCRTFallback handles the case where migration didn't run or failed
// This is a safety net - normal migration happens in migrations.go
func (h *handler) migrateCRTFallback(obj *v3.ClusterRegistrationToken) (*v3.ClusterRegistrationToken, error) {
	obj = obj.DeepCopy()

	var expiresAt string
	ttl := h.getTTL(obj)
	if ttl > 0 && h.isTTLRotationEnabled() {
		ttlDuration := time.Duration(ttl) * time.Minute
		jitter := computeJitter(ttlDuration)

		candidate := obj.CreationTimestamp.Time.Add(ttlDuration).Add(jitter)
		now := time.Now()

		if candidate.After(now) {
			expiresAt = candidate.UTC().Format(time.RFC3339)
		} else {
			expiresAt = now.Add(jitter).UTC().Format(time.RFC3339)
		}
	}

	if err := h.ensureTokenSecret(obj, obj.Status.Token, expiresAt); err != nil {
		return nil, err
	}

	obj.Status.Token = ""
	obj.Status.TokenSecretName = SecretName(obj.Name)

	newStatus, err := h.assignStatus(obj)
	if err != nil {
		return nil, err
	}
	obj.Status = newStatus

	if expiresAt != "" {
		obj.Status.ExpiresAt = expiresAt
	}

	logrus.Infof("Fallback migration completed for CRT %s/%s", obj.Namespace, obj.Name)
	return h.clusterRegistrationTokenController.Update(obj)
}

// createNewCRT generates a token for a new CRT and stores it in a secret
func (h *handler) createNewCRT(obj *v3.ClusterRegistrationToken) (*v3.ClusterRegistrationToken, error) {
	token, err := randomtoken.Generate()
	if err != nil {
		return nil, err
	}

	obj = obj.DeepCopy()

	// Calculate expiresAt once for consistency between secret and status
	var expiresAt string
	ttl := h.getTTL(obj)
	if ttl > 0 && h.isTTLRotationEnabled() {
		expiresAt = time.Now().Add(time.Duration(ttl) * time.Minute).UTC().Format(time.RFC3339)
	}

	if err := h.ensureTokenSecret(obj, token, expiresAt); err != nil {
		return nil, err
	}

	obj.Status.Token = ""

	obj.Status.TokenSecretName = SecretName(obj.Name)

	newStatus, err := h.assignStatus(obj)
	if err != nil {
		return nil, err
	}
	obj.Status = newStatus

	if expiresAt != "" {
		obj.Status.ExpiresAt = expiresAt
	}

	return h.clusterRegistrationTokenController.Update(obj)
}

// reconcileExistingCRT handles TTL rotation and status updates for existing CRTs
func (h *handler) reconcileExistingCRT(obj *v3.ClusterRegistrationToken) (*v3.ClusterRegistrationToken, error) {
	if err := h.ensureCRTTokenReaderRole(obj.Namespace); err != nil {
		return nil, err
	}

	obj = obj.DeepCopy()
	original := obj.DeepCopy()

	if err := h.handleTTL(obj); err != nil {
		return nil, err
	}

	newStatus, err := h.assignStatus(obj)
	if err != nil {
		return nil, err
	}

	if equality.Semantic.DeepEqual(original.Status, newStatus) {
		return obj, nil
	}
	obj.Status = newStatus

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := h.clusterRegistrationTokenController.Get(obj.Namespace, obj.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		latest = latest.DeepCopy()
		latest.Status = obj.Status

		updated, err := h.clusterRegistrationTokenController.Update(latest)
		if err == nil {
			obj = updated
		}
		return err
	})

	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (h *handler) handleTTL(obj *v3.ClusterRegistrationToken) error {
	now := time.Now()

	// Handle grace period cleanup FIRST - always enforce grace period deadline
	// even when feature flag is disabled. This ensures previousToken expires
	// at the promised time.
	if obj.Status.GracePeriodExpiresAt != "" {
		gpExpiry, err := time.Parse(time.RFC3339, obj.Status.GracePeriodExpiresAt)
		if err != nil {
			return err
		}
		if now.Before(gpExpiry) {
			h.clusterRegistrationTokenController.EnqueueAfter(obj.Namespace, obj.Name, time.Until(gpExpiry))
			return nil
		}
		return h.cleanupGracePeriod(obj)
	}

	if !h.isTTLRotationEnabled() {
		return nil
	}

	ttl := h.getTTL(obj)
	if ttl <= 0 {
		return nil
	}

	if obj.Status.ExpiresAt == "" {
		return h.setExpiresAt(obj, ttl)
	}

	expiry, err := time.Parse(time.RFC3339, obj.Status.ExpiresAt)
	if err != nil {
		return err
	}

	if now.Before(expiry) {
		h.clusterRegistrationTokenController.EnqueueAfter(obj.Namespace, obj.Name, time.Until(expiry))
		return nil
	}

	return h.rotateToken(obj, ttl)
}

func (h *handler) setExpiresAt(obj *v3.ClusterRegistrationToken, ttl int64) error {
	ttlDuration := time.Duration(ttl) * time.Minute
	jitter := computeJitter(ttlDuration)

	candidate := obj.CreationTimestamp.Time.Add(ttlDuration).Add(jitter)
	now := time.Now()

	if candidate.After(now) {
		obj.Status.ExpiresAt = candidate.UTC().Format(time.RFC3339)
	} else {
		obj.Status.ExpiresAt = now.Add(jitter).UTC().Format(time.RFC3339)
	}

	logrus.Infof("CRT %s/%s: ExpiresAt empty, set to %s", obj.Namespace, obj.Name, obj.Status.ExpiresAt)
	return nil
}

func (h *handler) rotateToken(obj *v3.ClusterRegistrationToken, ttl int64) error {
	secretName := SecretName(obj.Name)
	existing, err := h.secretCache.Get(obj.Namespace, secretName)
	if err != nil {
		return err
	}

	if expiresAtBytes, ok := existing.Data[expiresAtDataKey]; ok {
		expiresAt, err := time.Parse(time.RFC3339, string(expiresAtBytes))
		if err == nil && expiresAt.After(time.Now()) {
			obj.Status.ExpiresAt = string(expiresAtBytes)
			if _, hasPrev := existing.Data[previousTokenDataKey]; hasPrev {
				if gp, ok := existing.Data[gracePeriodExpiresAtDataKey]; ok && len(gp) > 0 {
					obj.Status.GracePeriodExpiresAt = string(gp)
				}
			}
			return nil
		}
	}

	now := time.Now()
	gracePeriod := h.getGracePeriod(obj)
	newExpiresAt := now.Add(time.Duration(ttl) * time.Minute).UTC().Format(time.RFC3339)
	newGracePeriodExpiresAt := now.Add(time.Duration(gracePeriod) * time.Minute).UTC().Format(time.RFC3339)

	newToken, err := randomtoken.Generate()
	if err != nil {
		return err
	}

	updated := existing.DeepCopy()
	updated.Data[previousTokenDataKey] = existing.Data[tokenDataKey]
	updated.Data[tokenDataKey] = []byte(newToken)
	updated.Data[expiresAtDataKey] = []byte(newExpiresAt)
	updated.Data[gracePeriodExpiresAtDataKey] = []byte(newGracePeriodExpiresAt)

	if _, err = h.secrets.Update(updated); err != nil {
		return err
	}

	obj.Status.ExpiresAt = newExpiresAt
	obj.Status.GracePeriodExpiresAt = newGracePeriodExpiresAt
	return nil
}

func (h *handler) cleanupGracePeriod(obj *v3.ClusterRegistrationToken) error {
	secretName := SecretName(obj.Name)
	existing, err := h.secretCache.Get(obj.Namespace, secretName)
	if err != nil {
		return err
	}

	if _, hasPrev := existing.Data[previousTokenDataKey]; hasPrev {
		updated := existing.DeepCopy()
		delete(updated.Data, previousTokenDataKey)
		delete(updated.Data, gracePeriodExpiresAtDataKey)
		if _, err := h.secrets.Update(updated); err != nil {
			return err
		}
	}

	obj.Status.GracePeriodExpiresAt = ""

	logrus.Infof("CRT %s/%s: grace period expired, previous token removed", obj.Namespace, obj.Name)
	return nil
}

func (h *handler) getTTL(obj *v3.ClusterRegistrationToken) int64 {
	var ttl int64
	if obj.Spec.TTL == nil {
		ttl = int64(settings.CRTDefaultTTL.GetInt())
	} else {
		ttl = *obj.Spec.TTL
	}

	return ClampTTL(ttl, obj.Namespace, obj.Name)
}

// ClampTTL clamps the TTL to the minimum value and logs a warning if clamping occurs.
// This is exported for use by migration code.
func ClampTTL(ttl int64, namespace, name string) int64 {
	if ttl < MinTTLMinutes {
		logrus.Warnf("CRT %s/%s: TTL %d is below minimum %d, clamping to minimum",
			namespace, name, ttl, MinTTLMinutes)
		return MinTTLMinutes
	}
	return ttl
}

func (h *handler) isTTLRotationEnabled() bool {
	return features.CRTTokenTTLRotation.Enabled()
}

func (h *handler) getGracePeriod(obj *v3.ClusterRegistrationToken) int64 {
	var gracePeriod int64
	if obj.Spec.GracePeriod == nil {
		gracePeriod = int64(settings.CRTDefaultGracePeriod.GetInt())
	} else {
		gracePeriod = *obj.Spec.GracePeriod
	}

	ttl := h.getTTL(obj)
	return ClampGracePeriod(gracePeriod, ttl, obj.Namespace, obj.Name)
}

// ClampGracePeriod clamps the grace period to the minimum value and ensures it's less than TTL.
// This is exported for use by migration code.
func ClampGracePeriod(gracePeriod, ttl int64, namespace, name string) int64 {
	// Clamp to minimum - defensive check in case setting validation is bypassed
	if gracePeriod < MinGracePeriodMinutes {
		logrus.Warnf("CRT %s/%s: grace period %d is below minimum %d, clamping to minimum",
			namespace, name, gracePeriod, MinGracePeriodMinutes)
		gracePeriod = MinGracePeriodMinutes
	}

	// Ensure grace period is less than TTL to prevent token accumulation
	if gracePeriod >= ttl {
		// Set grace period to 80% of TTL, or minimum, whichever is larger
		newGracePeriod := int64(float64(ttl) * 0.8)
		if newGracePeriod < MinGracePeriodMinutes {
			newGracePeriod = MinGracePeriodMinutes
		}
		logrus.Warnf("CRT %s/%s: grace period %d must be less than TTL %d, clamping to %d",
			namespace, name, gracePeriod, ttl, newGracePeriod)
		gracePeriod = newGracePeriod
	}

	return gracePeriod
}

func (h *handler) ensureTokenSecret(crt *v3.ClusterRegistrationToken, token string, expiresAt string) error {
	secretName := SecretName(crt.Name)
	existing, err := h.secretCache.Get(crt.Namespace, secretName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		secret := NewTokenSecret(crt, token, expiresAt)
		if _, err = h.secrets.Create(secret); err != nil {
			return err
		}
		// Ensure the crt-token-reader Role includes this Secret
		return h.ensureCRTTokenReaderRole(crt.Namespace)
	}

	if string(existing.Data[tokenDataKey]) != token {
		updated := existing.DeepCopy()
		if updated.Data == nil {
			updated.Data = make(map[string][]byte)
		}
		updated.Data[tokenDataKey] = []byte(token)
		_, err = h.secrets.Update(updated)
		return err
	}

	return nil
}

func computeJitter(ttl time.Duration) time.Duration {
	tenPercent := ttl / 10
	maxJitter := 24 * time.Hour
	if tenPercent > maxJitter {
		tenPercent = maxJitter
	}
	if tenPercent <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(tenPercent)))
}

// ensureCRTTokenReaderRole creates or updates the crt-token-reader Role in the given namespace.
// The Role grants get access to all crt-token-* Secrets in the namespace via resourceNames.
func (h *handler) ensureCRTTokenReaderRole(namespace string) error {
	ns, err := h.namespaceCache.Get(namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Namespace already deleted, nothing to do
			return nil
		}
		return fmt.Errorf("couldn't get namespace %v: %w", namespace, err)
	}
	if ns.DeletionTimestamp != nil || ns.Status.Phase == corev1.NamespaceTerminating {
		logrus.Debugf("Namespace %v is terminating, skipping crt-token-reader role reconciliation", namespace)
		return nil
	}

	// List all CRTs in this namespace to compute expected Secret names
	crts, err := h.clusterRegistrationTokenCache.List(namespace, labels.Everything())
	if err != nil {
		return err
	}

	var secretNames []string
	for _, crt := range crts {
		secretNames = append(secretNames, SecretName(crt.Name))
	}

	// If no CRT Secrets exist, delete the Role if it exists
	if len(secretNames) == 0 {
		err := h.roles.Delete(namespace, crtTokenReaderRole, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	desiredRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crtTokenReaderRole,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: secretNames,
				Verbs:         []string{"get"},
			},
		},
	}

	existing, err := h.roleCache.Get(namespace, crtTokenReaderRole)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		_, err = h.roles.Create(desiredRole)
		return err
	}

	if !equality.Semantic.DeepEqual(existing.Rules, desiredRole.Rules) {
		updated := existing.DeepCopy()
		updated.Rules = desiredRole.Rules
		_, err = h.roles.Update(updated)
		return err
	}

	return nil
}
