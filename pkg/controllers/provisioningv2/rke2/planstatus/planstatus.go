package planstatus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
)

type handler struct {
	secrets corecontrollers.SecretClient
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets: clients.Core.Secret(),
	}
	clients.Core.Secret().OnChange(ctx, "plan-status", h.OnChange)
}

func (h *handler) OnChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != rke2.SecretTypeMachinePlan || len(secret.Data) == 0 {
		return secret, nil
	}

	if len(secret.Data) == 0 {
		return secret, nil
	}

	appliedChecksum := string(secret.Data["applied-checksum"])
	plan := secret.Data["plan"]
	appliedPlan := secret.Data["appliedPlan"]

	if appliedChecksum == hash(plan) {
		if !bytes.Equal(plan, appliedPlan) {
			secret = secret.DeepCopy()
			secret.Data["appliedPlan"] = plan
			return h.secrets.Update(secret)
		}
	}

	return secret, nil
}

func hash(plan []byte) string {
	result := sha256.Sum256(plan)
	return hex.EncodeToString(result[:])
}
