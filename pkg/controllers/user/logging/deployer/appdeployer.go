package deployer

import (
	"context"
	"fmt"
	"time"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/apis/core/v1"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"

	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for deploy app
const (
	creatorIDAnn = "field.cattle.io/creatorId"
)

const (
	waitTimeout   = 30 * time.Second
	checkInterval = 2 * time.Second
)

type AppDeployer struct {
	AppsGetter projectv3.AppsGetter
	Namespaces v1.NamespaceInterface
	Pods       v1.PodInterface
}

func (d *AppDeployer) initNamespace(name string) error {
	if _, err := d.Namespaces.Controller().Lister().Get(metav1.NamespaceAll, name); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		initNamespace := k8scorev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		if _, err := d.Namespaces.Create(&initNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (d *AppDeployer) cleanup(appName, appTargetNamespace, projectID string) error {
	_, projectName := ref.Parse(projectID)

	if err := d.AppsGetter.Apps(projectName).Delete(appName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (d *AppDeployer) deploy(app *projectv3.App) error {
	if err := d.initNamespace(app.Spec.TargetNamespace); err != nil {
		return err
	}

	current, err := d.AppsGetter.Apps(app.Namespace).Get(app.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to query app %s:%s", app.Namespace, app.Name)
		}
		if _, err := d.AppsGetter.Apps(app.Namespace).Create(app); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create app %s:%s", app.Namespace, app.Name)
		}
	}

	if current.DeletionTimestamp != nil {
		return errors.New("stale app " + app.Namespace + ":" + app.Name + " still on terminating")
	}

	return nil
}

func (d *AppDeployer) isDeploySuccess(targetNamespace string, selector map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	var err error
	for range ticker.Context(ctx, checkInterval) {
		var pods *v1.PodList

		opt := metav1.ListOptions{
			LabelSelector: labels.Set(selector).String(),
			FieldSelector: fields.Set{"metadata.namespace": targetNamespace}.String(),
		}
		pods, err = d.Pods.List(opt)
		if err != nil {
			return errors.Wrap(err, "list pods failed in check app deploy")
		}

		if len(pods.Items) == 0 {
			continue
		}

		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case k8scorev1.PodFailed:
				return errors.New("get failed status from pod, please the check logs for " + pod.Namespace + ":" + pod.Name)
			case k8scorev1.PodRunning, k8scorev1.PodSucceeded:
				return nil
			}
		}
	}
	return fmt.Errorf("timeout check deploy app status, namespace: %s, labels: %v", targetNamespace, selector)
}

func rancherLoggingApp(systemProjectCreator, systemProjectID, catalogID, driverDir string) *projectv3.App {
	appName := loggingconfig.AppName
	namepspace := loggingconfig.LoggingNamespace
	_, systemProjectName := ref.Parse(systemProjectID)

	return &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: systemProjectCreator,
			},
			Name:      appName,
			Namespace: systemProjectName,
		},
		Spec: projectv3.AppSpec{
			Answers: map[string]string{
				"fluentd.enabled":              "true",
				"log-aggregator.enabled":       "true",
				"log-aggregator.flexVolumeDir": driverDir,
			},
			Description:     "Rancher Logging for collect logs",
			ExternalID:      catalogID,
			ProjectName:     systemProjectID,
			TargetNamespace: namepspace,
		},
	}
}

func loggingTesterApp(systemProjectCreator, systemProjectID, catalogID, clusterConfig, projectConfig string) *projectv3.App {
	appName := loggingconfig.TesterAppName
	namepspace := loggingconfig.LoggingNamespace
	_, systemProjectName := ref.Parse(systemProjectID)

	return &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: systemProjectCreator,
			},
			Name:      appName,
			Namespace: systemProjectName,
		},
		Spec: projectv3.AppSpec{
			Answers: map[string]string{
				"fluentd-tester.enabled": "true",
			},
			Description:     "Dry run to validate the config",
			ExternalID:      catalogID,
			ProjectName:     systemProjectID,
			TargetNamespace: namepspace,
		},
	}
}
