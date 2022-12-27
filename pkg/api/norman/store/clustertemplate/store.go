package clustertemplate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/norman/customization/clustertemplate"
	managementv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	mgmtSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

const (
	clusterTemplateLabelName = "io.cattle.field/clusterTemplateId"
	registrySecretKey        = "privateRegistrySecret"
	s3SecretKey              = "s3CredentialSecret"
	weaveSecretKey           = "weavePasswordSecret"
	vsphereSecretKey         = "vsphereSecret"
	virtualCenterSecretKey   = "virtualCenterSecret"
	openStackSecretKey       = "openStackSecret"
	aadClientSecretKey       = "aadClientSecret"
	aadClientCertSecretKey   = "aadClientCertSecret"
)

func WrapStore(store types.Store, mgmt *config.ScaledContext) types.Store {
	storeWrapped := &Store{
		Store:         store,
		users:         mgmt.Management.Users(""),
		grbLister:     mgmt.Management.GlobalRoleBindings("").Controller().Lister(),
		grLister:      mgmt.Management.GlobalRoles("").Controller().Lister(),
		ctLister:      mgmt.Management.ClusterTemplates("").Controller().Lister(),
		clusterLister: mgmt.Management.Clusters("").Controller().Lister(),
		secretMigrator: secretmigrator.NewMigrator(
			mgmt.Core.Secrets("").Controller().Lister(),
			mgmt.Core.Secrets(""),
		),
	}
	return storeWrapped
}

type Store struct {
	types.Store
	users          v3.UserInterface
	grbLister      v3.GlobalRoleBindingLister
	grLister       v3.GlobalRoleLister
	ctLister       v3.ClusterTemplateLister
	clusterLister  v3.ClusterLister
	SecretLister   v1.SecretLister
	secretMigrator *secretmigrator.Migrator
}

type secrets struct {
	regSecret       *corev1.Secret
	s3Secret        *corev1.Secret
	weaveSecret     *corev1.Secret
	vsphereSecret   *corev1.Secret
	vcenterSecret   *corev1.Secret
	openStackSecret *corev1.Secret
	aadClientSecret *corev1.Secret
	aadCertSecret   *corev1.Secret
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateType) {
		if err := p.checkMembersAccessType(data); err != nil {
			return nil, err
		}
	}

	var allSecrets secrets
	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		if data[managementv3.ClusterTemplateRevisionFieldClusterConfig] == nil {
			return nil, httperror.NewAPIError(httperror.MissingRequired, "ClusterTemplateRevision field ClusterConfig is required")
		}
		err := p.checkPermissionToCreateRevision(apiContext, data)
		if err != nil {
			return nil, err
		}
		err = p.checkKubernetesVersionFormat(apiContext, data)
		if err != nil {
			return nil, err
		}
		if err := setLabelsAndOwnerRef(apiContext, data); err != nil {
			return nil, err
		}
		allSecrets, err = p.migrateSecrets(data, "", "", "", "", "", "", "", "")
		if err != nil {
			return nil, err
		}
		data = cleanQuestions(data)
	}

	result, err := p.Store.Create(apiContext, schema, data)
	if err != nil {
		if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
			if allSecrets.regSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.regSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.s3Secret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.s3Secret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.weaveSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.weaveSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.vsphereSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.vsphereSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.vcenterSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.vcenterSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.openStackSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.openStackSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.aadClientSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.aadClientSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.aadCertSecret != nil {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.aadCertSecret.Name); cleanupErr != nil {
					logrus.Errorf("clustertemplate store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
		}
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status {
				return result, httperror.WrapAPIError(err, httperror.PermissionDenied, "You must have the `Create Cluster Templates` global role in order to create cluster templates or revisions. These permissions can be granted by an administrator.")
			}
		}
		return nil, err
	}

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		_, clusterTemplateName := ref.Parse(result["id"].(string))
		owner := metav1.OwnerReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "ClusterTemplateRevision",
			Name:       clusterTemplateName,
			UID:        k8sTypes.UID(result["uuid"].(string)),
		}
		if allSecrets.regSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.regSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.s3Secret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.s3Secret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.weaveSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.weaveSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.vsphereSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.vsphereSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.vcenterSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.vcenterSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.openStackSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.openStackSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.aadClientSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.aadClientSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
		if allSecrets.aadCertSecret != nil {
			err = p.secretMigrator.UpdateSecretOwnerReference(allSecrets.aadCertSecret, owner)
			if err != nil {
				logrus.Errorf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
			}
		}
	}
	return result, err
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateType) {
		if err := p.checkMembersAccessType(data); err != nil {
			return nil, err
		}
	}

	var allSecrets secrets
	var currentRegSecret, currentS3Secret, currentWeaveSecret string
	var currentVsphereSecret, currentVcenterSecret, currentOpenStackSecret, currentAADClientSecret, currentAADCertSecret string
	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		err := p.checkKubernetesVersionFormat(apiContext, data)
		if err != nil {
			return nil, err
		}

		isUsed, err := p.isTemplateInUse(apiContext, id)
		if err != nil {
			return nil, err
		}
		if isUsed {
			return nil, httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Cannot update the %v until Clusters are referring it", apiContext.Type))
		}
		existingClusterTemplate, err := p.ByID(apiContext, schema, id)
		if err != nil {
			return nil, err
		}
		currentRegSecret, _ = existingClusterTemplate[registrySecretKey].(string)
		currentS3Secret, _ = existingClusterTemplate[s3SecretKey].(string)
		currentWeaveSecret, _ = existingClusterTemplate[weaveSecretKey].(string)
		currentVsphereSecret, _ = existingClusterTemplate[vsphereSecretKey].(string)
		currentVcenterSecret, _ = existingClusterTemplate[virtualCenterSecretKey].(string)
		currentOpenStackSecret, _ = existingClusterTemplate[openStackSecretKey].(string)
		currentAADClientSecret, _ = existingClusterTemplate[aadClientSecretKey].(string)
		currentAADCertSecret, _ = existingClusterTemplate[aadClientCertSecretKey].(string)
		allSecrets, err = p.migrateSecrets(data, currentRegSecret, currentS3Secret, currentWeaveSecret, currentVsphereSecret, currentVcenterSecret, currentOpenStackSecret, currentAADClientSecret, currentAADCertSecret)
		if err != nil {
			return nil, err
		}
		data = cleanQuestions(data)
	}

	result, err := p.Store.Update(apiContext, schema, data, id)

	if err != nil {
		if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
			if allSecrets.regSecret != nil && currentRegSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.regSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.s3Secret != nil && currentS3Secret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.s3Secret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.weaveSecret != nil && currentWeaveSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.weaveSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.vsphereSecret != nil && currentVsphereSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.vsphereSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.vcenterSecret != nil && currentVcenterSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.vcenterSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.openStackSecret != nil && currentOpenStackSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.openStackSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.aadClientSecret != nil && currentAADClientSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.aadClientSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if allSecrets.aadCertSecret != nil && currentAADCertSecret == "" {
				if cleanupErr := p.secretMigrator.Cleanup(allSecrets.aadCertSecret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
			if apiError, ok := err.(*httperror.APIError); ok {
				if apiError.Code.Status == httperror.PermissionDenied.Status {
					return result, httperror.WrapAPIError(err, httperror.PermissionDenied, "You do not have permissions to create or edit the cluster templates or revisions. These permissions can be granted by an administrator.")
				}
			}
		}
	}

	return result, err
}

func (p *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {

	isUsed, err := p.isTemplateInUse(apiContext, id)
	if err != nil {
		return nil, err
	}
	if isUsed {
		return nil, httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Cannot delete the %v until Clusters referring it are removed", apiContext.Type))
	}

	// check if template.DefaultRevisionId is set, if yes error out if the revision is being deleted.
	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		isDefault, err := isDefaultTemplateRevision(apiContext, id)
		if err != nil {
			return nil, err
		}
		if isDefault {
			return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("Cannot delete the %v since this is the default revision of the Template, Please change the default revision first", apiContext.Type))
		}
	}

	result, err := p.Store.Delete(apiContext, schema, id)

	if err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status {
				return result, httperror.WrapAPIError(err, httperror.PermissionDenied, "You do not have permissions to delete the cluster templates or revisions. These permissions can be granted by an administrator.")
			}
		}
	}

	return result, err
}

func setLabelsAndOwnerRef(apiContext *types.APIContext, data map[string]interface{}) error {
	var template managementv3.ClusterTemplate

	templateID := convert.ToString(data[managementv3.ClusterTemplateRevisionFieldClusterTemplateID])
	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateType, templateID, &template); err != nil {
		return err
	}

	split := strings.SplitN(template.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("error in splitting clusterTemplate ID %v", template.ID)
	}
	templateName := split[1]

	labels := map[string]string{
		clusterTemplateLabelName: templateName,
	}
	data["labels"] = labels

	var ownerReferencesSlice []map[string]interface{}
	ownerReference := map[string]interface{}{
		managementv3.OwnerReferenceFieldKind:       "ClusterTemplate",
		managementv3.OwnerReferenceFieldAPIVersion: "management.cattle.io/v3",
		managementv3.OwnerReferenceFieldName:       templateName,
		managementv3.OwnerReferenceFieldUID:        template.UUID,
	}
	ownerReferencesSlice = append(ownerReferencesSlice, ownerReference)
	data["ownerReferences"] = ownerReferencesSlice

	return nil
}

func (p *Store) isTemplateInUse(apiContext *types.APIContext, id string) (bool, error) {

	/*check if there are any clusters referencing this template or templateRevision */
	var clusters []*v3.Cluster
	var field string

	clusters, err := p.clusterLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	for _, cluster := range clusters {
		switch apiContext.Type {
		case managementv3.ClusterTemplateType:
			field = cluster.Spec.ClusterTemplateName
		case managementv3.ClusterTemplateRevisionType:
			field = cluster.Spec.ClusterTemplateRevisionName
		default:
			break
		}
		if field != id {
			continue
		}
		return true, nil
	}

	return false, nil
}

func isDefaultTemplateRevision(apiContext *types.APIContext, id string) (bool, error) {

	var template managementv3.ClusterTemplate
	var templateRevision managementv3.ClusterTemplateRevision

	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateRevisionType, id, &templateRevision); err != nil {
		return false, err
	}

	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateType, templateRevision.ClusterTemplateID, &template); err != nil {
		return false, err
	}

	if template.DefaultRevisionID == id {
		return true, nil
	}

	return false, nil
}

func (p *Store) checkPermissionToCreateRevision(apiContext *types.APIContext, data map[string]interface{}) error {
	value, found := values.GetValue(data, managementv3.ClusterTemplateRevisionFieldClusterTemplateID)
	if !found {
		return httperror.NewAPIError(httperror.NotFound, "invalid request: clusterTemplateID not found")
	}

	clusterTemplateID := convert.ToString(value)
	_, clusterTemplateName := ref.Parse(clusterTemplateID)
	var ctMap map[string]interface{}
	if err := access.ByID(apiContext, &mgmtSchema.Version, managementv3.ClusterTemplateType, clusterTemplateID, &ctMap); err != nil {
		return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to access clusterTemplate by id: %v", err))
	}
	if err := apiContext.AccessControl.CanDo(v3.ClusterTemplateGroupVersionKind.Group, v3.ClusterTemplateResource.Name, "update", apiContext, ctMap, apiContext.Schema); err != nil {
		return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("user does not have permission to update clusterTemplate %s by creating a revision for it", clusterTemplateName))
	}
	return nil
}

func (p *Store) checkKubernetesVersionFormat(apiContext *types.APIContext, data map[string]interface{}) error {
	clusterConfig, found := values.GetValue(data, managementv3.ClusterTemplateRevisionFieldClusterConfig)
	if !found || clusterConfig == nil {
		return httperror.NewAPIError(httperror.MissingRequired, "ClusterTemplateRevision field ClusterConfig is required")
	}
	k8sVersionReq := values.GetValueN(data, managementv3.ClusterTemplateRevisionFieldClusterConfig, "rancherKubernetesEngineConfig", "kubernetesVersion")
	if k8sVersionReq == nil {
		return nil
	}
	k8sVersion := convert.ToString(k8sVersionReq)
	genericPatch, err := clustertemplate.CheckKubernetesVersionFormat(k8sVersion)
	if err != nil {
		return err
	}
	if genericPatch {
		// ensure a question is added for "rancherKubernetesEngineConfig.kubernetesVersion"
		templateQuestions, ok := data[managementv3.ClusterTemplateRevisionFieldQuestions]
		if !ok {
			return httperror.NewAPIError(httperror.MissingRequired, fmt.Sprintf("ClusterTemplateRevision must have a Question set for %v", clustertemplate.RKEConfigK8sVersion))
		}
		templateQuestionsSlice := convert.ToMapSlice(templateQuestions)
		var foundQ bool
		for _, question := range templateQuestionsSlice {
			if question["variable"] == clustertemplate.RKEConfigK8sVersion {
				foundQ = true
				break
			}
		}
		if !foundQ {
			return httperror.NewAPIError(httperror.MissingRequired, fmt.Sprintf("ClusterTemplateRevision must have a Question set for %v", clustertemplate.RKEConfigK8sVersion))
		}
	}
	return nil
}

func (p *Store) checkMembersAccessType(data map[string]interface{}) error {
	members := convert.ToMapSlice(data[managementv3.ClusterTemplateFieldMembers])
	for _, m := range members {
		accessType := convert.ToString(m[managementv3.MemberFieldAccessType])
		if accessType != rbac.OwnerAccess && accessType != rbac.ReadOnlyAccess {
			return httperror.NewAPIError(httperror.InvalidBodyContent, "Invalid accessType provided while sharing cluster template")
		}
	}
	return nil
}

func (p *Store) migrateSecrets(data map[string]interface{}, currentReg, currentS3, currentWeave, currentVsphere, currentVCenter, currentOpenStack, currentAADClientSecret, currentAADCert string) (secrets, error) {
	rkeConfig, err := getRkeConfig(data)
	if err != nil || rkeConfig == nil {
		return secrets{}, err
	}
	var s secrets
	s.regSecret, err = p.secretMigrator.CreateOrUpdatePrivateRegistrySecret(currentReg, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.regSecret != nil {
		data[registrySecretKey] = s.regSecret.Name
		rkeConfig.PrivateRegistries = secretmigrator.CleanRegistries(rkeConfig.PrivateRegistries)
	}
	s.s3Secret, err = p.secretMigrator.CreateOrUpdateS3Secret(currentS3, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.s3Secret != nil {
		data[s3SecretKey] = s.s3Secret.Name
		rkeConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
	}
	s.weaveSecret, err = p.secretMigrator.CreateOrUpdateWeaveSecret(currentWeave, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.weaveSecret != nil {
		data[weaveSecretKey] = s.weaveSecret.Name
		rkeConfig.Network.WeaveNetworkProvider.Password = ""
	}
	s.vsphereSecret, err = p.secretMigrator.CreateOrUpdateVsphereGlobalSecret(currentVsphere, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.vsphereSecret != nil {
		data[vsphereSecretKey] = s.vsphereSecret.Name
		rkeConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
	}
	s.vcenterSecret, err = p.secretMigrator.CreateOrUpdateVsphereVirtualCenterSecret(currentVCenter, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.vcenterSecret != nil {
		data[virtualCenterSecretKey] = s.vcenterSecret.Name
		for k, v := range rkeConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
			v.Password = ""
			rkeConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
		}
	}
	s.openStackSecret, err = p.secretMigrator.CreateOrUpdateOpenStackSecret(currentOpenStack, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.openStackSecret != nil {
		data[openStackSecretKey] = s.openStackSecret.Name
		rkeConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
	}
	s.aadClientSecret, err = p.secretMigrator.CreateOrUpdateAADClientSecret(currentAADClientSecret, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aadClientSecret != nil {
		data[aadClientSecretKey] = s.aadClientSecret.Name
		rkeConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
	}
	s.aadCertSecret, err = p.secretMigrator.CreateOrUpdateAADCertSecret(currentAADCert, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aadCertSecret != nil {
		data[aadClientCertSecretKey] = s.aadCertSecret.Name
		rkeConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
	}

	encodedRkeConfig, err := convert.EncodeToMap(rkeConfig)
	if err != nil {
		return secrets{}, err
	}
	values.PutValue(data, encodedRkeConfig, "clusterConfig", "rancherKubernetesEngineConfig")
	return s, nil
}

func getRkeConfig(data map[string]interface{}) (*rketypes.RancherKubernetesEngineConfig, error) {
	rkeConfig := values.GetValueN(data, managementv3.ClusterTemplateRevisionFieldClusterConfig, "rancherKubernetesEngineConfig")
	if rkeConfig == nil {
		return nil, nil
	}
	config, err := json.Marshal(rkeConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshaling rkeConfig")
	}
	var spec *rketypes.RancherKubernetesEngineConfig
	if err := json.Unmarshal([]byte(config), &spec); err != nil {
		return nil, errors.Wrapf(err, "error reading rkeConfig")
	}
	return spec, nil
}

func cleanQuestions(data map[string]interface{}) map[string]interface{} {
	if _, ok := data["questions"]; ok {
		questions := data["questions"].([]interface{})
		for i := range questions {
			q := questions[i].(map[string]interface{})
			if secretmigrator.MatchesQuestionPath(q["variable"].(string)) {
				delete(q, "default")
			}
			questions[i] = q
		}
		values.PutValue(data, questions, "questions")
	}
	return data
}
