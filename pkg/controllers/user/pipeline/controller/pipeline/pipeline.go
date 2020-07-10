package pipeline

import (
	"context"

	"github.com/rancher/rancher/pkg/pipeline/providers"
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// This controller is responsible for watching pipelines and handling
// webhook management in source code providers.

type Lifecycle struct {
	sourceCodeCredentialLister v3.SourceCodeCredentialLister
	sourceCodeCredentials      v3.SourceCodeCredentialInterface
}

func Register(ctx context.Context, cluster *config.UserContext) {
	pipelines := cluster.Management.Project.Pipelines("")
	sourceCodeCredentials := cluster.Management.Project.SourceCodeCredentials("")
	sourceCodeCredentialLister := sourceCodeCredentials.Controller().Lister()

	pipelineLifecycle := &Lifecycle{
		sourceCodeCredentialLister: sourceCodeCredentialLister,
		sourceCodeCredentials:      sourceCodeCredentials,
	}

	pipelines.AddClusterScopedLifecycle(ctx, "pipeline-controller", cluster.ClusterName, pipelineLifecycle)
}

func (l *Lifecycle) Create(obj *v3.Pipeline) (runtime.Object, error) {
	return obj, nil
}

func (l *Lifecycle) Updated(obj *v3.Pipeline) (runtime.Object, error) {
	return l.sync(obj)
}

func (l *Lifecycle) Remove(obj *v3.Pipeline) (runtime.Object, error) {
	if obj.Status.WebHookID != "" {
		if err := l.deleteHook(obj); err != nil {
			logrus.WithError(err).Warnf("fail to delete webhook for pipeline %q", obj.Name)
			return obj, nil
		}
	}
	return obj, nil
}

func (l *Lifecycle) sync(obj *v3.Pipeline) (*v3.Pipeline, error) {
	if obj.Status.Token == "" {
		//random token for webhook validation
		obj.Status.Token = uuid.NewV4().String()
	}

	sourceCodeCredentialID := obj.Spec.SourceCodeCredentialName
	if sourceCodeCredentialID != "" {
		ns, name := ref.Parse(sourceCodeCredentialID)
		if obj.Status.SourceCodeCredential == nil ||
			obj.Status.SourceCodeCredential.Namespace != ns ||
			obj.Status.SourceCodeCredential.Name != name {
			updatedCred, err := l.sourceCodeCredentialLister.Get(ns, name)
			if err != nil {
				return obj, err
			}
			updatedCred = updatedCred.DeepCopy()
			updatedCred.Spec.AccessToken = ""
			obj.Status.SourceCodeCredential = updatedCred
		}
	}

	//handle webhook
	if obj.Status.WebHookID != "" && !hasWebhookTrigger(obj) {
		if err := l.deleteHook(obj); err != nil {
			return obj, err
		}
		obj.Status.WebHookID = ""
	} else if hasWebhookTrigger(obj) && obj.Status.WebHookID == "" {
		id, err := l.createHook(obj)
		if err != nil {
			return obj, err
		}
		obj.Status.WebHookID = id
	}

	return obj, nil
}

func (l *Lifecycle) createHook(obj *v3.Pipeline) (string, error) {
	credentialID := obj.Spec.SourceCodeCredentialName

	ns, name := ref.Parse(credentialID)
	credential, err := l.sourceCodeCredentialLister.Get(ns, name)
	if err != nil {
		return "", err
	}
	_, projID := ref.Parse(obj.Spec.ProjectName)
	scpConfig, err := providers.GetSourceCodeProviderConfig(credential.Spec.SourceCodeType, projID)
	if err != nil {
		return "", err
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return "", err
	}
	accessToken, err := utils.EnsureAccessToken(l.sourceCodeCredentials, remote, credential)
	if err != nil {
		return "", err
	}
	id, err := remote.CreateHook(obj, accessToken)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (l *Lifecycle) deleteHook(obj *v3.Pipeline) error {
	credentialID := obj.Spec.SourceCodeCredentialName

	ns, name := ref.Parse(credentialID)
	credential, err := l.sourceCodeCredentialLister.Get(ns, name)
	if err != nil {
		return err
	}
	_, projID := ref.Parse(obj.Spec.ProjectName)
	scpConfig, err := providers.GetSourceCodeProviderConfig(credential.Spec.SourceCodeType, projID)
	if err != nil {
		return err
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return err
	}
	accessToken, err := utils.EnsureAccessToken(l.sourceCodeCredentials, remote, credential)
	if err != nil {
		return err
	}
	return remote.DeleteHook(obj, accessToken)
}

func hasWebhookTrigger(obj *v3.Pipeline) bool {
	if obj != nil && (obj.Spec.TriggerWebhookPr || obj.Spec.TriggerWebhookPush || obj.Spec.TriggerWebhookTag) {
		return true
	}
	return false
}
