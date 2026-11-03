package managedchart

import (
	tar2 "archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/fleet/pkg/helmvalues"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	fleetcontrollers "github.com/rancher/rancher/pkg/generated/controllers/fleet.cattle.io/v1alpha1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	chartByRepo = "chartByRepo"
)

// chartGetter abstracts chart retrieval so the handler can be unit-tested
// without a real catalog stack.
type chartGetter interface {
	Chart(namespace, name, chartName, version string, skipFilter bool) (io.ReadCloser, error)
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		charts: content.NewManager(
			clients.K8s.Discovery(),
			clients.Core.ConfigMap().Cache(),
			clients.Core.Secret().Cache(),
			clients.Catalog.ClusterRepo().Cache()),
		mccCache:      clients.Mgmt.ManagedChart().Cache(),
		mccController: clients.Mgmt.ManagedChart(),
		bundleCache:   clients.Fleet.Bundle().Cache(),
		secrets:       clients.Core.Secret(),
	}

	clients.Catalog.ClusterRepo().OnChange(ctx, "mcc-repo", h.OnRepoChange)
	relatedresource.Watch(ctx,
		"mcc-from-bundle-trigger",
		relatedresource.OwnerResolver(true, v3.SchemeGroupVersion.String(), "ManagedChart"),
		clients.Mgmt.ManagedChart(),
		clients.Fleet.Bundle())
	mgmtcontrollers.RegisterManagedChartGeneratingHandler(ctx,
		clients.Mgmt.ManagedChart(),
		clients.Apply.
			WithSetOwnerReference(true, true).
			WithCacheTypes(
				clients.Mgmt.ManagedChart(),
				clients.Fleet.Bundle(),
				clients.Core.Secret()),
		"Defined",
		"mcc-bundle",
		h.OnChange,
		nil)
	clients.Mgmt.ManagedChart().Cache().AddIndexer(chartByRepo, func(obj *v3.ManagedChart) ([]string, error) {
		return []string{obj.Spec.RepoName}, nil
	})
}

type handler struct {
	charts        chartGetter
	mccCache      mgmtcontrollers.ManagedChartCache
	mccController mgmtcontrollers.ManagedChartController
	bundleCache   fleetcontrollers.BundleCache
	secrets       corecontrollers.SecretClient
}

func (h *handler) OnRepoChange(key string, _ *v1.ClusterRepo) (*v1.ClusterRepo, error) {
	mccs, err := h.mccCache.GetByIndex(chartByRepo, key)
	if err != nil {
		return nil, err
	}

	for _, mcc := range mccs {
		h.mccController.Enqueue(mcc.Namespace, mcc.Name)
	}

	return nil, nil
}

func (h *handler) OnChange(mcc *v3.ManagedChart, status v3.ManagedChartStatus) ([]runtime.Object, v3.ManagedChartStatus, error) {
	chart, err := h.charts.Chart("", mcc.Spec.RepoName, mcc.Spec.Chart, mcc.Spec.Version, true)
	if err != nil {
		return nil, status, err
	}
	defer chart.Close()

	bundleName := capr.SafeConcatName(capr.MaxHelmReleaseNameLength, "mcc", mcc.Name)
	bundle := &v1alpha1.Bundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: mcc.Namespace,
		},
		Spec: v1alpha1.BundleSpec{
			BundleDeploymentOptions: v1alpha1.BundleDeploymentOptions{
				DefaultNamespace: mcc.Spec.DefaultNamespace,
				TargetNamespace:  mcc.Spec.TargetNamespace,
				Helm: &v1alpha1.HelmOptions{
					ReleaseName:    name.Limit(mcc.Spec.ReleaseName, capr.MaxHelmReleaseNameLength),
					Version:        mcc.Spec.Version,
					TimeoutSeconds: mcc.Spec.TimeoutSeconds,
					Values:         mcc.Spec.Values,
					Force:          mcc.Spec.Force,
					TakeOwnership:  mcc.Spec.TakeOwnership,
					MaxHistory:     mcc.Spec.MaxHistory,
				},
				ServiceAccount: mcc.Spec.ServiceAccount,
				Diff:           mcc.Spec.Diff,
			},
			Paused:          mcc.Spec.Paused,
			RolloutStrategy: mcc.Spec.RolloutStrategy,
			Targets:         mcc.Spec.Targets,
		},
	}

	gz, err := gzip.NewReader(chart)
	if err != nil {
		return nil, status, err
	}

	tar := tar2.NewReader(gz)
	for {
		next, err := tar.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, status, err
		}

		if next.Typeflag != tar2.TypeReg {
			continue
		}

		buf := &bytes.Buffer{}
		gz := gzip.NewWriter(buf)
		if _, err := io.Copy(gz, tar); err != nil {
			return nil, status, err
		}
		if err := gz.Close(); err != nil {
			return nil, status, err
		}

		name := strings.TrimPrefix(next.Name, mcc.Spec.Chart+"/")
		bundle.Spec.Resources = append(bundle.Spec.Resources, v1alpha1.BundleResource{
			Name:     name,
			Content:  base64.StdEncoding.EncodeToString(buf.Bytes()),
			Encoding: "base64+gz",
		})
	}

	sort.Slice(bundle.Spec.Resources, func(i, j int) bool {
		return bundle.Spec.Resources[i].Name < bundle.Spec.Resources[j].Name
	})

	// Move values out of the Bundle spec into a dedicated Secret so they are
	// not stored in plain text. Fleet's bundle controller will load them back
	// from the Secret when it needs to render the chart.
	hash, data, err := helmvalues.ExtractValues(bundle)
	if err != nil {
		return nil, status, fmt.Errorf("extracting managed chart helm values: %w", err)
	}

	// Apply the Secret FIRST before setting ValuesHash on the Bundle.
	// This ensures the Secret exists before Fleet tries to load it.
	if hash != "" {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bundleName,
				Namespace: mcc.Namespace,
			},
			Type: v1alpha1.SecretTypeBundleValues,
			Data: data,
		}

		// Try to create or update the secret
		existing, err := h.secrets.Get(mcc.Namespace, bundleName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			// Secret doesn't exist, create it
			if _, err := h.secrets.Create(secret); err != nil {
				return nil, status, fmt.Errorf("creating values secret: %w", err)
			}
		} else if err != nil {
			return nil, status, fmt.Errorf("getting values secret: %w", err)
		} else {
			// Secret exists, update it
			// Note: Type field is immutable, so if it's wrong we can't fix it here.
			// This will fail if there's a pre-existing secret with a different Type.
			if existing.Type != v1alpha1.SecretTypeBundleValues {
				return nil, status, fmt.Errorf("existing secret %s/%s has wrong type %q (expected %q); delete it manually to proceed",
					mcc.Namespace, bundleName, existing.Type, v1alpha1.SecretTypeBundleValues)
			}
			existing.Data = data
			if _, err := h.secrets.Update(existing); err != nil {
				return nil, status, fmt.Errorf("updating values secret: %w", err)
			}
		}

		// Only set ValuesHash AFTER the secret has been successfully applied
		helmvalues.ClearValues(bundle)
		bundle.Spec.ValuesHash = hash
	}

	status, err = h.updateStatus(status, bundle)
	return []runtime.Object{bundle}, status, err
}

func (h *handler) updateStatus(status v3.ManagedChartStatus, bundle *v1alpha1.Bundle) (v3.ManagedChartStatus, error) {
	bundle, err := h.bundleCache.Get(bundle.Namespace, bundle.Name)
	if apierrors.IsNotFound(err) {
		return status, nil
	} else if err != nil {
		return status, err
	}

	status.BundleStatus = bundle.Status
	return status, nil
}
