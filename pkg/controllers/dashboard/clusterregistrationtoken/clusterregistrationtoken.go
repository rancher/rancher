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

// enqueueCRTsWithoutExpiry re-enqueues CRTs whose token Secret has no expiresAt yet.
// Called on TTL flag/setting changes so they pick up a schedule.
func (h *handler) enqueueCRTsWithoutExpiry() error {
	if !isTTLRotationEnabled() {
		return nil
	}
	crts, err := h.clusterRegistrationTokenCache.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, crt := range crts {
		if crt.Status.TokenSecretName == "" {
			continue
		}
		secret, err := h.secretCache.Get(crt.Namespace, crt.Status.TokenSecretName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logrus.Warnf("CRT %s/%s: failed to get token secret %s: %v", crt.Namespace, crt.Name, crt.Status.TokenSecretName, err)
			}
			continue
		}
		if expiresAt, ok := secret.Data[expiresAtDataKey]; !ok || len(expiresAt) == 0 {
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
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	obj = obj.DeepCopy()
	original := obj.DeepCopy()
	var requeueAfter time.Duration

	// Runs only once, on first creation (or as fallback migration for a
	// pre-Secret CRT). The Status update below re-triggers onChange with
	// TokenSecretName set, entering the else branch for TTL handling.
	if obj.Status.TokenSecretName == "" {
		token := obj.Status.Token
		if token != "" {
			logrus.Warnf("CRT %s/%s has Status.Token set but no TokenSecretName - this should have been migrated. Migrating now as fallback.",
				obj.Namespace, obj.Name)
		} else {
			token, err = randomtoken.Generate()
			if err != nil {
				return nil, err
			}
		}

		var expiresAt string
		ttl := getTTL(obj)
		if ttl > 0 && isTTLRotationEnabled() {
			expiresAt = computeExpiresAt(obj.CreationTimestamp.Time, ttl, time.Now())
		}

		if err := h.ensureTokenSecret(obj, token, expiresAt); err != nil {
			return nil, err
		}

		obj.Status.Token = ""
		obj.Status.TokenSecretName = SecretName(obj.Name)
		obj.Status.ExpiresAt = expiresAt
		obj.Status.GracePeriodExpiresAt = ""
	} else {
		if err := h.ensureCRTTokenReaderRole(obj.Namespace); err != nil {
			return nil, err
		}

		requeueAfter, err = h.handleTTL(obj)
		if err != nil {
			return nil, err
		}
	}

	newStatus, err := h.assignStatus(obj)
	if err != nil {
		return nil, err
	}

	if !equality.Semantic.DeepEqual(original.Status, newStatus) {
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
	}

	if requeueAfter > 0 {
		h.clusterRegistrationTokenController.EnqueueAfter(obj.Namespace, obj.Name, requeueAfter)
	}

	return obj, nil
}

type ttlDecision struct {
	setData              map[string][]byte
	deleteKeys           []string
	expiresAt            string
	gracePeriodExpiresAt string
	requeueAfter         time.Duration
}

func (d ttlDecision) writesSecret() bool {
	return len(d.setData) > 0 || len(d.deleteKeys) > 0
}

func computeTTLDecision(now time.Time, obj *v3.ClusterRegistrationToken, data map[string][]byte) (ttlDecision, error) {
	ttl := getTTL(obj)
	gracePeriod := getGracePeriod(obj, ttl)
	ttlEnabled := isTTLRotationEnabled()
	creationTime := obj.CreationTimestamp.Time
	currentExpiresAt := obj.Status.ExpiresAt
	currentGracePeriodExpiresAt := obj.Status.GracePeriodExpiresAt

	d := ttlDecision{
		expiresAt:            currentExpiresAt,
		gracePeriodExpiresAt: currentGracePeriodExpiresAt,
	}

	// Grace period is checked first, even if TTL is disabled: a promised
	// previousToken expiry must still fire regardless of the feature flag.
	if gp, ok := data[gracePeriodExpiresAtDataKey]; ok && len(gp) > 0 {
		gpExpiry, err := time.Parse(time.RFC3339, string(gp))
		if err != nil {
			return ttlDecision{}, err
		}

		if now.Before(gpExpiry) {
			d.gracePeriodExpiresAt = string(gp)
			if ea, ok := data[expiresAtDataKey]; ok && len(ea) > 0 {
				d.expiresAt = string(ea)
			}
			d.requeueAfter = gpExpiry.Sub(now)
			return d, nil
		}

		d.gracePeriodExpiresAt = ""
		d.deleteKeys = []string{gracePeriodExpiresAtDataKey}
		if _, hasPrev := data[previousTokenDataKey]; hasPrev {
			d.deleteKeys = append(d.deleteKeys, previousTokenDataKey)
		}
		return d, nil
	}

	if !ttlEnabled || ttl <= 0 {
		return d, nil
	}

	ea, ok := data[expiresAtDataKey]
	if !ok || len(ea) == 0 {
		expiresAt := computeExpiresAt(creationTime, ttl, now)
		d.setData = map[string][]byte{expiresAtDataKey: []byte(expiresAt)}
		d.expiresAt = expiresAt
		return d, nil
	}

	expiry, err := time.Parse(time.RFC3339, string(ea))
	if err != nil {
		return ttlDecision{}, err
	}

	if now.Before(expiry) {
		d.expiresAt = string(ea)
		d.requeueAfter = expiry.Sub(now)
		return d, nil
	}

	newToken, err := randomtoken.Generate()
	if err != nil {
		return ttlDecision{}, err
	}

	newExpiresAt := now.Add(time.Duration(ttl) * time.Minute).UTC().Format(time.RFC3339)
	newGracePeriodExpiresAt := now.Add(time.Duration(gracePeriod) * time.Minute).UTC().Format(time.RFC3339)

	d.setData = map[string][]byte{
		tokenDataKey:                []byte(newToken),
		expiresAtDataKey:            []byte(newExpiresAt),
		gracePeriodExpiresAtDataKey: []byte(newGracePeriodExpiresAt),
	}
	if oldToken, ok := data[tokenDataKey]; ok && len(oldToken) > 0 {
		d.setData[previousTokenDataKey] = oldToken
	} else {
		// tokenDataKey should always be populated once the token secret exists; reaching rotation without one
		// indicates the secret was corrupted.
		logrus.Warnf("CRT %s/%s: %s is missing a valid token at rotation time; proceeding without a previousToken",
			obj.Namespace, obj.Name, obj.Status.TokenSecretName)
	}
	d.expiresAt = newExpiresAt
	d.gracePeriodExpiresAt = newGracePeriodExpiresAt
	return d, nil
}

func (h *handler) handleTTL(obj *v3.ClusterRegistrationToken) (time.Duration, error) {
	secret, err := h.secretCache.Get(obj.Namespace, obj.Status.TokenSecretName)
	if err != nil {
		return 0, err
	}

	decision, err := computeTTLDecision(time.Now(), obj, secret.Data)
	if err != nil {
		return 0, err
	}

	if decision.writesSecret() {
		updated := secret.DeepCopy()
		if updated.Data == nil {
			updated.Data = make(map[string][]byte)
		}
		for k, v := range decision.setData {
			updated.Data[k] = v
		}
		for _, k := range decision.deleteKeys {
			delete(updated.Data, k)
		}
		if _, err := h.secrets.Update(updated); err != nil {
			return 0, err
		}
	}

	obj.Status.ExpiresAt = decision.expiresAt
	obj.Status.GracePeriodExpiresAt = decision.gracePeriodExpiresAt

	return decision.requeueAfter, nil
}

func getTTL(obj *v3.ClusterRegistrationToken) int64 {
	var ttl int64
	if obj.Spec.TTL == nil {
		ttl = int64(settings.CRTDefaultTTL.GetInt())
	} else {
		ttl = *obj.Spec.TTL
	}

	return ClampTTL(ttl, obj.Namespace, obj.Name)
}

// ClampTTL clamps the TTL to the minimum value and logs (at trace level, since this can fire on
// every reconcile) if clamping occurs. ttl <= 0 is treated as an explicit "never expire" sentinel
// and is left untouched.
// This is exported for use by migration code.
func ClampTTL(ttl int64, namespace, name string) int64 {
	if ttl <= 0 {
		return ttl
	}
	if ttl < MinTTLMinutes {
		logrus.Tracef("CRT %s/%s: TTL %d is below minimum %d, clamping to minimum",
			namespace, name, ttl, MinTTLMinutes)
		return MinTTLMinutes
	}
	return ttl
}

func isTTLRotationEnabled() bool {
	return features.CRTTokenTTLRotation.Enabled()
}

func getGracePeriod(obj *v3.ClusterRegistrationToken, ttl int64) int64 {
	var gracePeriod int64
	if obj.Spec.GracePeriod == nil {
		gracePeriod = int64(settings.CRTDefaultGracePeriod.GetInt())
	} else {
		gracePeriod = *obj.Spec.GracePeriod
	}

	return ClampGracePeriod(gracePeriod, ttl, obj.Namespace, obj.Name)
}

// ClampGracePeriod clamps the grace period to the minimum value and ensures it's less than TTL.
func ClampGracePeriod(gracePeriod, ttl int64, namespace, name string) int64 {
	if ttl <= 0 {
		return gracePeriod
	}

	// Clamp to minimum - defensive check in case setting validation is bypassed
	if gracePeriod < MinGracePeriodMinutes {
		logrus.Tracef("CRT %s/%s: grace period %d is below minimum %d, clamping to minimum",
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
		logrus.Tracef("CRT %s/%s: grace period %d must be less than TTL %d, clamping to %d",
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

	// Only reached during initial/legacy migration - a live token's Secret
	// should never be overwritten after that.
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

// computeExpiresAt computes a jittered expiry from creationTime + ttl,
// falling back to now + jitter if that candidate is already in the past.
func computeExpiresAt(creationTime time.Time, ttl int64, now time.Time) string {
	ttlDuration := time.Duration(ttl) * time.Minute
	jitter := ComputeJitter(ttlDuration)

	candidate := creationTime.Add(ttlDuration).Add(jitter)
	if candidate.After(now) {
		return candidate.UTC().Format(time.RFC3339)
	}
	return now.Add(jitter).UTC().Format(time.RFC3339)
}

// ComputeJitter returns a random duration up to 10% of ttl (capped at 24h),
// used to avoid synchronized expiry/rotation across CRTs.
func ComputeJitter(ttl time.Duration) time.Duration {
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
