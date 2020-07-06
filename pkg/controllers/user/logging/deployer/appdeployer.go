package deployer

import (
	"fmt"
	"reflect"
	"time"

	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/ref"
	v1 "github.com/rancher/types/apis/core/v1"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"

	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	AppsLister projectv3.AppLister
	Namespaces v1.NamespaceInterface
	PodLister  v1.PodLister
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

	if _, err := d.AppsLister.Get(projectName, appName); err != nil {
		if apierrors.IsNotFound(err) {
			// the app doesn't exist
			return nil
		}
		return err
	}

	if err := d.AppsGetter.Apps(projectName).Delete(appName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (d *AppDeployer) deploy(app *projectv3.App, systemWriteKeys []string) error {
	if err := d.initNamespace(app.Spec.TargetNamespace); err != nil {
		return err
	}

	current, err := d.AppsLister.Get(app.Namespace, app.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to query app %s:%s", app.Namespace, app.Name)
		}
		current, err = d.AppsGetter.Apps(app.Namespace).Create(app)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create app %s:%s", app.Namespace, app.Name)
		}
	}

	if current.DeletionTimestamp != nil {
		return errors.New("stale app " + app.Namespace + ":" + app.Name + " still on terminating")
	}

	new := current.DeepCopy()
	if len(systemWriteKeys) != 0 {
		if new.Spec.Answers == nil {
			new.Spec.Answers = make(map[string]string)
		}

		for _, v := range systemWriteKeys {
			new.Spec.Answers[v] = app.Spec.Answers[v]
		}
	}

	if !reflect.DeepEqual(current.Spec, new.Spec) {
		_, err = d.AppsGetter.Apps(app.Namespace).Update(new)
		if err != nil {
			return errors.Wrapf(err, "failed to update app %s", app.Name)
		}
	}
	return nil
}

func (d *AppDeployer) isDeploySuccess(targetNamespace string, selector map[string]string) error {
	timeout := time.After(waitTimeout)
	timeoutErr := fmt.Errorf("timeout checking app status, app deployed namespace: %s, labels: %+v", targetNamespace, selector)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pods, err := d.PodLister.List(targetNamespace, labels.Set(selector).AsSelector())
			if err != nil {
				return errors.Wrap(err, "list pods failed in check app deploy")
			}

			for _, pod := range pods {
				switch pod.Status.Phase {
				case k8scorev1.PodFailed:
					return errors.New("get failed status from pod, please the check logs for " + pod.Namespace + ":" + pod.Name)
				case k8scorev1.PodRunning, k8scorev1.PodSucceeded:
					return nil
				}
			}

		case <-timeout:
			return timeoutErr
		}
	}
}

func rancherLoggingApp(appCreator, systemProjectID, catalogID, driverDir, dockerRoot string) *projectv3.App {
	appName := loggingconfig.AppName
	namepspace := loggingconfig.LoggingNamespace
	_, systemProjectName := ref.Parse(systemProjectID)

	return &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: appCreator,
			},
			Name:      appName,
			Namespace: systemProjectName,
		},
		Spec: projectv3.AppSpec{
			Answers: map[string]string{
				//compatible with old version
				"fluentd.enabled":              "true",
				"fluentd.cluster.dockerRoot":   dockerRoot,
				"log-aggregator.enabled":       "true",
				"log-aggregator.flexVolumeDir": driverDir,

				//new version
				"fluentd.fluentd-linux.enabled":                     "true",
				"fluentd.fluentd-linux.cluster.dockerRoot":          dockerRoot,
				"log-aggregator.log-aggregator-linux.enabled":       "true",
				"log-aggregator.log-aggregator-linux.flexVolumeDir": driverDir,
			},
			Description:     "Rancher Logging for collect logs",
			ExternalID:      catalogID,
			ProjectName:     systemProjectID,
			TargetNamespace: namepspace,
		},
	}
}

func loggingTesterApp(appCreator, systemProjectID, catalogID, clusterConfig, projectConfig string) *projectv3.App {
	appName := loggingconfig.TesterAppName
	namepspace := loggingconfig.LoggingNamespace
	_, systemProjectName := ref.Parse(systemProjectID)

	return &projectv3.App{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				creatorIDAnn: appCreator,
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

func updateInRancherLoggingAppWindowsConfig(app *projectv3.App, enableWindows bool) {
	if app.Spec.Answers == nil {
		app.Spec.Answers = make(map[string]string)
	}
	if enableWindows {
		app.Spec.Answers["fluentd.fluentd-windows.enabled"] = "true"
	} else {
		app.Spec.Answers["fluentd.fluentd-windows.enabled"] = "false"
	}
}
