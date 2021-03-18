package apiservice

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
)

func (h *handler) SetupInternalServerURL(key string, setting *v3.Setting) (*v3.Setting, error) {
	if key != settings.ServerURL.Name {
		return setting, nil
	}

	internalURL, internalCA, err := h.getInternalServerAndURL()
	if err != nil {
		return nil, err
	}

	// purposely update CA before URL, because we only wait for internalURL != "" when checking if it's initialized
	if settings.InternalCACerts.Get() != internalCA {
		if err := settings.InternalCACerts.Set(internalCA); err != nil {
			return setting, err
		}
	}

	if settings.InternalServerURL.Get() != internalURL {
		if err := settings.InternalServerURL.Set(internalURL); err != nil {
			return setting, err
		}
	}

	return setting, nil
}

func (h *handler) getInternalServerAndURL() (string, string, error) {
	serverURL := settings.ServerURL.Get()
	ca := settings.CACerts.Get()

	tlsSecret, err := h.secrets.Get(namespace.System, "tls-rancher-internal-ca")
	if err != nil {
		return "", "", err
	}
	internalCA := string(tlsSecret.Data[corev1.TLSCertKey])

	if dp, err := h.deploymentCache.Get(namespace.System, "rancher"); err == nil && dp.Spec.Replicas != nil && *dp.Spec.Replicas != 0 {
		return fmt.Sprintf("https://rancher.%s", namespace.System), internalCA, nil
	}

	if _, err := h.daemonSetCache.Get(namespace.System, "rancher"); err == nil {
		return fmt.Sprintf("https://rancher.%s", namespace.System), internalCA, nil
	}

	return serverURL, ca, nil
}
