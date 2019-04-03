package globaldns

import (
	"context"
	"fmt"

	"reflect"

	"github.com/rancher/rancher/pkg/controllers/management/globalnamespacerbac"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GlobaldnsProviderCatalogLauncher = "mgmt-global-dns-provider-catalog-launcher"
	cattleCreatorIDAnnotationKey     = "field.cattle.io/creatorId"
	localClusterName                 = "local"
)

type ProviderCatalogLauncher struct {
	managementContext *config.ManagementContext
	Apps              pv3.AppInterface
	ProjectLister     v3.ProjectLister
	appLister         pv3.AppLister
}

func newGlobalDNSProviderCatalogLauncher(ctx context.Context, mgmt *config.ManagementContext) *ProviderCatalogLauncher {
	n := &ProviderCatalogLauncher{
		managementContext: mgmt,
		Apps:              mgmt.Project.Apps(""),
		ProjectLister:     mgmt.Management.Projects("").Controller().Lister(),
		appLister:         mgmt.Project.Apps("").Controller().Lister(),
	}
	return n
}

//sync is called periodically and on real updates
func (n *ProviderCatalogLauncher) sync(key string, obj *v3.GlobalDNSProvider) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}
	metaAccessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	creatorID, ok := metaAccessor.GetAnnotations()[globalnamespacerbac.CreatorIDAnn]
	if !ok {
		return nil, fmt.Errorf("GlobalDNS %v has no creatorId annotation", metaAccessor.GetName())
	}

	if err := globalnamespacerbac.CreateRoleAndRoleBinding(globalnamespacerbac.GlobalDNSProviderResource, obj.Name,
		obj.UID, obj.Spec.Members, creatorID, n.managementContext); err != nil {
		return nil, err
	}

	//handle external-dns deployment
	if obj.Spec.Route53ProviderConfig != nil {
		return n.handleRoute53Provider(obj)
	}

	if obj.Spec.CloudflareProviderConfig != nil {
		return n.handleCloudflareProvider(obj)
	}

	if obj.Spec.AlidnsProviderConfig != nil {
		return n.handleAlidnsProvider(obj)
	}

	return nil, nil
}

func (n *ProviderCatalogLauncher) handleRoute53Provider(obj *v3.GlobalDNSProvider) (runtime.Object, error) {
	rancherInstallUUID := settings.InstallUUID.Get()
	//create external-dns route53 provider
	answers := map[string]string{
		"provider":      "aws",
		"aws.zoneType":  "public",
		"aws.accessKey": obj.Spec.Route53ProviderConfig.AccessKey,
		"aws.secretKey": obj.Spec.Route53ProviderConfig.SecretKey,
		"txtOwnerId":    rancherInstallUUID + "_" + obj.Name,
		"rbac.create":   "true",
		"policy":        "sync",
	}

	if obj.Spec.RootDomain != "" {
		answers["domainFilters[0]"] = obj.Spec.RootDomain
	}
	return n.createUpdateExternalDNSApp(obj, answers)
}

func (n *ProviderCatalogLauncher) handleCloudflareProvider(obj *v3.GlobalDNSProvider) (runtime.Object, error) {
	rancherInstallUUID := settings.InstallUUID.Get()
	//create external-dns route53 provider
	answers := map[string]string{
		"provider":          "cloudflare",
		"cloudflare.apiKey": obj.Spec.CloudflareProviderConfig.APIKey,
		"cloudflare.email":  obj.Spec.CloudflareProviderConfig.APIEmail,
		"txtOwnerId":        rancherInstallUUID + "_" + obj.Name,
		"rbac.create":       "true",
		"policy":            "sync",
	}

	if obj.Spec.RootDomain != "" {
		answers["domainFilters[0]"] = obj.Spec.RootDomain
	}

	return n.createUpdateExternalDNSApp(obj, answers)
}

func (n *ProviderCatalogLauncher) handleAlidnsProvider(obj *v3.GlobalDNSProvider) (runtime.Object, error) {
	rancherInstallUUID := settings.InstallUUID.Get()
	//create external-dns alidns provider
	answers := map[string]string{
		"provider":               "alibabacloud",
		"alibabacloud.zoneType":  "public",
		"alibabacloud.accessKey": obj.Spec.AlidnsProviderConfig.AccessKey,
		"alibabacloud.secretKey": obj.Spec.AlidnsProviderConfig.SecretKey,
		"txtOwnerId":             rancherInstallUUID + "_" + obj.Name,
		"rbac.create":            "true",
		"policy":                 "sync",
	}

	if obj.Spec.RootDomain != "" {
		answers["domainFilters[0]"] = obj.Spec.RootDomain
	}

	return n.createUpdateExternalDNSApp(obj, answers)
}

func (n *ProviderCatalogLauncher) createUpdateExternalDNSApp(obj *v3.GlobalDNSProvider, answers map[string]string) (runtime.Object, error) {
	//check if provider already running for this GlobalDNSProvider.
	existingApp, err := n.getProviderIfAlreadyRunning(obj)
	if err != nil {
		return nil, err
	}

	if existingApp != nil {
		//check if answers should be updated
		if !reflect.DeepEqual(existingApp.Spec.Answers, answers) {
			appToupdate := existingApp.DeepCopy()
			appToupdate.Spec.Answers = answers
			_, err = n.Apps.Update(appToupdate)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
		}
	} else {
		//create new app
		appCatalogID := settings.SystemExternalDNSCatalogID.Get()
		sysProject, err := n.getSystemProjectID()
		if err != nil {
			return nil, err
		}

		//set ownerRef to the dnsProvider CR
		controller := true
		ownerRef := []metav1.OwnerReference{{
			Name:       obj.Name,
			APIVersion: "v3",
			UID:        obj.UID,
			Kind:       obj.Kind,
			Controller: &controller,
		}}

		toCreate := pv3.App{
			ObjectMeta: metav1.ObjectMeta{
				Name:            fmt.Sprintf("%s-%s", "systemapp", obj.Name),
				Namespace:       sysProject,
				OwnerReferences: ownerRef,
			},
			Spec: pv3.AppSpec{
				ProjectName:     localClusterName + ":" + sysProject,
				TargetNamespace: namespace.GlobalNamespace,
				ExternalID:      appCatalogID,
				Answers:         answers,
			},
		}
		// Now create the App instance
		_, err = n.Apps.Create(&toCreate)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	return nil, nil
}

func (n *ProviderCatalogLauncher) getProviderIfAlreadyRunning(obj *v3.GlobalDNSProvider) (*pv3.App, error) {
	sysProject, err := n.getSystemProjectID()
	if err != nil {
		return nil, err
	}
	existingApp, err := n.appLister.Get(sysProject, fmt.Sprintf("%s-%s", "systemapp", obj.Name))

	if (err != nil && k8serrors.IsNotFound(err)) || existingApp == nil {
		return nil, nil
	} else if err != nil && !k8serrors.IsNotFound(err) {
		logrus.Errorf("GlobaldnsProviderCatalogLauncher: Error listing external-dns %v app %v", obj.Name, err)
		return nil, err
	}

	return existingApp, nil
}

func (n *ProviderCatalogLauncher) getSystemProjectID() (string, error) {
	systemProject, err := project.GetSystemProject(localClusterName, n.ProjectLister)
	if err != nil {
		return "", err
	}

	return systemProject.Name, nil
}
