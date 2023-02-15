package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterTemplateRevisionType                                  = "clusterTemplateRevision"
	ClusterTemplateRevisionFieldAADClientCertSecret              = "aadClientCertSecret"
	ClusterTemplateRevisionFieldAADClientSecret                  = "aadClientSecret"
	ClusterTemplateRevisionFieldACIAPICUserKeySecret             = "aciAPICUserKeySecret"
	ClusterTemplateRevisionFieldACIKafkaClientKeySecret          = "aciKafkaClientKeySecret"
	ClusterTemplateRevisionFieldACITokenSecret                   = "aciTokenSecret"
	ClusterTemplateRevisionFieldAnnotations                      = "annotations"
	ClusterTemplateRevisionFieldBastionHostSSHKeySecret          = "bastionHostSSHKeySecret"
	ClusterTemplateRevisionFieldClusterConfig                    = "clusterConfig"
	ClusterTemplateRevisionFieldClusterTemplateID                = "clusterTemplateId"
	ClusterTemplateRevisionFieldConditions                       = "conditions"
	ClusterTemplateRevisionFieldCreated                          = "created"
	ClusterTemplateRevisionFieldCreatorID                        = "creatorId"
	ClusterTemplateRevisionFieldEnabled                          = "enabled"
	ClusterTemplateRevisionFieldKubeletExtraEnvSecret            = "kubeletExtraEnvSecret"
	ClusterTemplateRevisionFieldLabels                           = "labels"
	ClusterTemplateRevisionFieldName                             = "name"
	ClusterTemplateRevisionFieldOpenStackSecret                  = "openStackSecret"
	ClusterTemplateRevisionFieldOwnerReferences                  = "ownerReferences"
	ClusterTemplateRevisionFieldPrivateRegistryECRSecret         = "privateRegistryECRSecret"
	ClusterTemplateRevisionFieldPrivateRegistrySecret            = "privateRegistrySecret"
	ClusterTemplateRevisionFieldQuestions                        = "questions"
	ClusterTemplateRevisionFieldRemoved                          = "removed"
	ClusterTemplateRevisionFieldS3CredentialSecret               = "s3CredentialSecret"
	ClusterTemplateRevisionFieldSecretsEncryptionProvidersSecret = "secretsEncryptionProvidersSecret"
	ClusterTemplateRevisionFieldState                            = "state"
	ClusterTemplateRevisionFieldTransitioning                    = "transitioning"
	ClusterTemplateRevisionFieldTransitioningMessage             = "transitioningMessage"
	ClusterTemplateRevisionFieldUUID                             = "uuid"
	ClusterTemplateRevisionFieldVirtualCenterSecret              = "virtualCenterSecret"
	ClusterTemplateRevisionFieldVsphereSecret                    = "vsphereSecret"
	ClusterTemplateRevisionFieldWeavePasswordSecret              = "weavePasswordSecret"
)

type ClusterTemplateRevision struct {
	types.Resource
	AADClientCertSecret              string                             `json:"aadClientCertSecret,omitempty" yaml:"aadClientCertSecret,omitempty"`
	AADClientSecret                  string                             `json:"aadClientSecret,omitempty" yaml:"aadClientSecret,omitempty"`
	ACIAPICUserKeySecret             string                             `json:"aciAPICUserKeySecret,omitempty" yaml:"aciAPICUserKeySecret,omitempty"`
	ACIKafkaClientKeySecret          string                             `json:"aciKafkaClientKeySecret,omitempty" yaml:"aciKafkaClientKeySecret,omitempty"`
	ACITokenSecret                   string                             `json:"aciTokenSecret,omitempty" yaml:"aciTokenSecret,omitempty"`
	Annotations                      map[string]string                  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	BastionHostSSHKeySecret          string                             `json:"bastionHostSSHKeySecret,omitempty" yaml:"bastionHostSSHKeySecret,omitempty"`
	ClusterConfig                    *ClusterSpecBase                   `json:"clusterConfig,omitempty" yaml:"clusterConfig,omitempty"`
	ClusterTemplateID                string                             `json:"clusterTemplateId,omitempty" yaml:"clusterTemplateId,omitempty"`
	Conditions                       []ClusterTemplateRevisionCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Created                          string                             `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                        string                             `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Enabled                          *bool                              `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	KubeletExtraEnvSecret            string                             `json:"kubeletExtraEnvSecret,omitempty" yaml:"kubeletExtraEnvSecret,omitempty"`
	Labels                           map[string]string                  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                             string                             `json:"name,omitempty" yaml:"name,omitempty"`
	OpenStackSecret                  string                             `json:"openStackSecret,omitempty" yaml:"openStackSecret,omitempty"`
	OwnerReferences                  []OwnerReference                   `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PrivateRegistryECRSecret         string                             `json:"privateRegistryECRSecret,omitempty" yaml:"privateRegistryECRSecret,omitempty"`
	PrivateRegistrySecret            string                             `json:"privateRegistrySecret,omitempty" yaml:"privateRegistrySecret,omitempty"`
	Questions                        []Question                         `json:"questions,omitempty" yaml:"questions,omitempty"`
	Removed                          string                             `json:"removed,omitempty" yaml:"removed,omitempty"`
	S3CredentialSecret               string                             `json:"s3CredentialSecret,omitempty" yaml:"s3CredentialSecret,omitempty"`
	SecretsEncryptionProvidersSecret string                             `json:"secretsEncryptionProvidersSecret,omitempty" yaml:"secretsEncryptionProvidersSecret,omitempty"`
	State                            string                             `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning                    string                             `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage             string                             `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                             string                             `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	VirtualCenterSecret              string                             `json:"virtualCenterSecret,omitempty" yaml:"virtualCenterSecret,omitempty"`
	VsphereSecret                    string                             `json:"vsphereSecret,omitempty" yaml:"vsphereSecret,omitempty"`
	WeavePasswordSecret              string                             `json:"weavePasswordSecret,omitempty" yaml:"weavePasswordSecret,omitempty"`
}

type ClusterTemplateRevisionCollection struct {
	types.Collection
	Data   []ClusterTemplateRevision `json:"data,omitempty"`
	client *ClusterTemplateRevisionClient
}

type ClusterTemplateRevisionClient struct {
	apiClient *Client
}

type ClusterTemplateRevisionOperations interface {
	List(opts *types.ListOpts) (*ClusterTemplateRevisionCollection, error)
	ListAll(opts *types.ListOpts) (*ClusterTemplateRevisionCollection, error)
	Create(opts *ClusterTemplateRevision) (*ClusterTemplateRevision, error)
	Update(existing *ClusterTemplateRevision, updates interface{}) (*ClusterTemplateRevision, error)
	Replace(existing *ClusterTemplateRevision) (*ClusterTemplateRevision, error)
	ByID(id string) (*ClusterTemplateRevision, error)
	Delete(container *ClusterTemplateRevision) error

	ActionDisable(resource *ClusterTemplateRevision) error

	ActionEnable(resource *ClusterTemplateRevision) error

	CollectionActionListquestions(resource *ClusterTemplateRevisionCollection) (*ClusterTemplateQuestionsOutput, error)
}

func newClusterTemplateRevisionClient(apiClient *Client) *ClusterTemplateRevisionClient {
	return &ClusterTemplateRevisionClient{
		apiClient: apiClient,
	}
}

func (c *ClusterTemplateRevisionClient) Create(container *ClusterTemplateRevision) (*ClusterTemplateRevision, error) {
	resp := &ClusterTemplateRevision{}
	err := c.apiClient.Ops.DoCreate(ClusterTemplateRevisionType, container, resp)
	return resp, err
}

func (c *ClusterTemplateRevisionClient) Update(existing *ClusterTemplateRevision, updates interface{}) (*ClusterTemplateRevision, error) {
	resp := &ClusterTemplateRevision{}
	err := c.apiClient.Ops.DoUpdate(ClusterTemplateRevisionType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterTemplateRevisionClient) Replace(obj *ClusterTemplateRevision) (*ClusterTemplateRevision, error) {
	resp := &ClusterTemplateRevision{}
	err := c.apiClient.Ops.DoReplace(ClusterTemplateRevisionType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterTemplateRevisionClient) List(opts *types.ListOpts) (*ClusterTemplateRevisionCollection, error) {
	resp := &ClusterTemplateRevisionCollection{}
	err := c.apiClient.Ops.DoList(ClusterTemplateRevisionType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterTemplateRevisionClient) ListAll(opts *types.ListOpts) (*ClusterTemplateRevisionCollection, error) {
	resp := &ClusterTemplateRevisionCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (cc *ClusterTemplateRevisionCollection) Next() (*ClusterTemplateRevisionCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterTemplateRevisionCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterTemplateRevisionClient) ByID(id string) (*ClusterTemplateRevision, error) {
	resp := &ClusterTemplateRevision{}
	err := c.apiClient.Ops.DoByID(ClusterTemplateRevisionType, id, resp)
	return resp, err
}

func (c *ClusterTemplateRevisionClient) Delete(container *ClusterTemplateRevision) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterTemplateRevisionType, &container.Resource)
}

func (c *ClusterTemplateRevisionClient) ActionDisable(resource *ClusterTemplateRevision) error {
	err := c.apiClient.Ops.DoAction(ClusterTemplateRevisionType, "disable", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterTemplateRevisionClient) ActionEnable(resource *ClusterTemplateRevision) error {
	err := c.apiClient.Ops.DoAction(ClusterTemplateRevisionType, "enable", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterTemplateRevisionClient) CollectionActionListquestions(resource *ClusterTemplateRevisionCollection) (*ClusterTemplateQuestionsOutput, error) {
	resp := &ClusterTemplateQuestionsOutput{}
	err := c.apiClient.Ops.DoCollectionAction(ClusterTemplateRevisionType, "listquestions", &resource.Collection, nil, resp)
	return resp, err
}
