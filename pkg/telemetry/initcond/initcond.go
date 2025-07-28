package initcond

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/telemetry/consts"
	"github.com/rancher/rancher/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	ManagedByAnnotation = "app.kubernetes.io/managed-by"
	PartOfAnnotation    = "app.kubernetes.io/part-of"

	BackupLabel = "resources.cattle.io/backup"
)

var (
	InitRetryDuration = time.Second * 15
)

type InitInfo struct {
	ClusterUUID    string
	InstallUUID    string
	ServerURL      string
	RancherVersion string
	GitHash        string
	hasTelemetryNs bool
}

func (i InitInfo) isReady() bool {
	return i.hasTelemetryNs && i.ClusterUUID != "" && i.ServerURL != "" && i.InstallUUID != "" && i.RancherVersion != ""
}

func getInitInfo(wContext *wrangler.Context) InitInfo {
	namespaces := wContext.Core.Namespace()
	serverURL := settings.ServerURL.Get()
	installUUID := settings.InstallUUID.Get()
	rancherVersion := settings.ServerVersion.Get()
	clusterUUID := ""

	kubeSystenNs, err := namespaces.Get("kube-system", metav1.GetOptions{})
	if err == nil {
		clusterUUID = string(kubeSystenNs.UID)
	}

	hasTelemetryNs := false
	telemetryNs, err := namespaces.Get(consts.TelemetrySecretNamespace, metav1.GetOptions{})
	if err == nil && telemetryNs != nil {
		hasTelemetryNs = true
	}

	return InitInfo{
		ClusterUUID:    clusterUUID,
		ServerURL:      serverURL,
		InstallUUID:    installUUID,
		RancherVersion: rancherVersion,
		GitHash:        version.GitCommit,
		hasTelemetryNs: hasTelemetryNs,
	}
}

func WaitForInfo(wContext *wrangler.Context, initInfo *InitInfo, done chan struct{}) {
	wait.Until(func() {
		logrus.Info("initializing required info for telemetry manager...")
		gotInitInfo := getInitInfo(wContext)
		if gotInitInfo.isReady() {
			logrus.Info("initialized required info for telemetry manager")
			initInfo.ServerURL = gotInitInfo.ServerURL
			initInfo.ClusterUUID = gotInitInfo.ClusterUUID
			initInfo.InstallUUID = gotInitInfo.InstallUUID
			initInfo.RancherVersion = gotInitInfo.RancherVersion
			initInfo.GitHash = gotInitInfo.GitHash
			initInfo.hasTelemetryNs = gotInitInfo.hasTelemetryNs
			close(done)
		}
		logrus.Info("telemetry manager info not available yet, re-queing check...")
	}, InitRetryDuration, done)
}

func CreateTelemetryNamespace(ctx context.Context, wContext *wrangler.Context) (*corev1.Namespace, error) {
	namespaces := wContext.Core.Namespace()

	existing, err := namespaces.Get(consts.TelemetrySecretNamespace, metav1.GetOptions{})
	if err == nil {
		return existing, nil
	}

	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("unexpected error while creating telemetry namespace: %w", err)
	}

	return namespaces.Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: consts.TelemetrySecretNamespace,
			Annotations: map[string]string{
				ManagedByAnnotation: "rancher",
				PartOfAnnotation:    "rancher-telemetry",
			},
			Labels: map[string]string{
				BackupLabel: "false",
			},
		},
	})
}
