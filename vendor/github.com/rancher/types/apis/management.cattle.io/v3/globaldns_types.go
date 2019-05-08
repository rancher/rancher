package v3

import (
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GlobalDNS struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlobalDNSSpec   `json:"spec,omitempty"`
	Status GlobalDNSStatus `json:"status,omitempty"`
}

type GlobalDNSSpec struct {
	FQDN                string   `json:"fqdn,omitempty" norman:"type=hostname,required"`
	TTL                 int64    `json:"ttl,omitempty" norman:"default=300"`
	ProjectNames        []string `json:"projectNames" norman:"type=array[reference[project]],noupdate"`
	MultiClusterAppName string   `json:"multiClusterAppName,omitempty" norman:"type=reference[multiClusterApp]"`
	ProviderName        string   `json:"providerName,omitempty" norman:"type=reference[globalDnsProvider],required"`
	Members             []Member `json:"members,omitempty"`
}

type GlobalDNSStatus struct {
	Endpoints        []string            `json:"endpoints,omitempty"`
	ClusterEndpoints map[string][]string `json:"clusterEndpoints,omitempty"`
}

type GlobalDNSProvider struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	//ObjectMeta.Name = GlobalDNSProviderID
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GlobalDNSProviderSpec `json:"spec,omitempty"`
}

type GlobalDNSProviderSpec struct {
	Route53ProviderConfig    *Route53ProviderConfig    `json:"route53ProviderConfig,omitempty"`
	CloudflareProviderConfig *CloudflareProviderConfig `json:"cloudflareProviderConfig,omitempty"`
	AlidnsProviderConfig     *AlidnsProviderConfig     `json:"alidnsProviderConfig,omitempty"`
	Members                  []Member                  `json:"members,omitempty"`
	RootDomain               string                    `json:"rootDomain"`
}

type Route53ProviderConfig struct {
	AccessKey       string `json:"accessKey" norman:"notnullable,required,minLength=1"`
	SecretKey       string `json:"secretKey" norman:"notnullable,required,minLength=1,type=password"`
	CredentialsPath string `json:"credentialsPath" norman:"default=/.aws"`
	RoleArn         string `json:"roleArn,omitempty"`
	Region          string `json:"region" norman:"default=us-east-1"`
	ZoneType        string `json:"zoneType" norman:"default=public"`
}

type CloudflareProviderConfig struct {
	APIKey       string `json:"apiKey" norman:"notnullable,required,minLength=1,type=password"`
	APIEmail     string `json:"apiEmail" norman:"notnullable,required,minLength=1"`
	ProxySetting *bool  `json:"proxySetting" norman:"default=true"`
}

type UpdateGlobalDNSTargetsInput struct {
	ProjectNames []string `json:"projectNames" norman:"type=array[reference[project]]"`
}

type AlidnsProviderConfig struct {
	AccessKey string `json:"accessKey" norman:"notnullable,required,minLength=1"`
	SecretKey string `json:"secretKey" norman:"notnullable,required,minLength=1,type=password"`
}
