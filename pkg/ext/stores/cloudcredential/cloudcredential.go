// cloudcredential implements the store for the CloudCredential resource.
package cloudcredential

import (
	"context"
	"fmt"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
)

const (
	// CredentialNamespace is the namespace where CloudCredential secrets are stored
	CredentialNamespace = "cattle-cloud-credentials"

	// SecretTypePrefix is the prefix for the Secret type field to identify cloud credential secrets
	SecretTypePrefix = "rke.cattle.io/cloud-credential-"

	// Labels for identifying and categorizing cloud credential secrets
	LabelCloudCredential          = "cattle.io/cloud-credential"
	LabelCloudCredentialName      = "cattle.io/cloud-credential-name"
	LabelCloudCredentialNamespace = "cattle.io/cloud-credential-namespace"
	LabelCloudCredentialOwner     = "cattle.io/cloud-credential-owner"

	// Annotations for cloud credential metadata
	AnnotationDescription = "cattle.io/cloud-credential-description"
	AnnotationCreatorID   = "field.cattle.io/creatorId"

	// AnnotationLastAppliedConfig is the standard kubectl annotation that leaks credential data.
	// It is always stripped from both the backing secret and the returned CloudCredential.
	AnnotationLastAppliedConfig = "kubectl.kubernetes.io/last-applied-configuration"

	// GeneratePrefix is the prefix used for generated credential secret names
	GeneratePrefix = "cc-"

	// GenericTypePrefix is the prefix required for generic credential types
	GenericTypePrefix = "x-"

	// CredentialConfigSuffix is appended to the type to form the credential config schema name.
	// Must be lowercase to match how DynamicSchemas are registered by node drivers and KEv2 operators.
	CredentialConfigSuffix = "credentialconfig"

	// Data field names used in the backing secret for status information
	FieldConditions    = "conditions"
	FieldVisibleFields = "visibleFields"

	SingularName = "cloudcredential"
	PluralName   = SingularName + "s"
)

var (
	GV = schema.GroupVersion{
		Group:   "ext.cattle.io",
		Version: "v1",
	}

	GVK = schema.GroupVersionKind{
		Group:   GV.Group,
		Version: GV.Version,
		Kind:    "CloudCredential",
	}

	GVR = schema.GroupVersionResource{
		Group:    GV.Group,
		Version:  GV.Version,
		Resource: "cloudcredentials",
	}
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// Store is the interface to the cloudcredential store.
type Store struct {
	SystemStore
	auth authorizer.Authorizer
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// SystemStore is the interface to the cloudcredential store used internally.
type SystemStore struct {
	namespaceClient    v1.NamespaceClient
	namespaceCache     v1.NamespaceCache
	secretClient       v1.SecretClient
	secretCache        v1.SecretCache
	dynamicSchemaCache mgmtv3.DynamicSchemaCache
	tableConverter     rest.TableConvertor
}

// NewFromWrangler is a convenience function for creating a cloudcredential store.
func NewFromWrangler(wranglerContext *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	return New(
		authorizer,
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Namespace().Cache(),
		wranglerContext.Core.Secret(),
		wranglerContext.Core.Secret().Cache(),
		wranglerContext.Mgmt.DynamicSchema().Cache(),
	)
}

// New is the main constructor for cloudcredential stores.
func New(
	auth authorizer.Authorizer,
	namespaceClient v1.NamespaceClient,
	namespaceCache v1.NamespaceCache,
	secretClient v1.SecretController,
	secretCache v1.SecretCache,
	dynamicSchemaCache mgmtv3.DynamicSchemaCache,
) *Store {
	return &Store{
		auth: auth,
		SystemStore: SystemStore{
			namespaceClient: namespaceClient,
			namespaceCache:  namespaceCache,
			secretClient:    secretClient,
			secretCache:     secretCache,

			dynamicSchemaCache: dynamicSchemaCache,
			tableConverter: printerstorage.TableConvertor{
				TableGenerator: printers.NewTableGenerator().With(printHandler),
			},
		},
	}
}

// GroupVersionKind implements [rest.GroupVersionKindProvider]
func (s *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper]
func (s *Store) NamespaceScoped() bool {
	return true
}

// GetSingularName implements [rest.SingularNameProvider]
func (s *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage]
func (s *Store) New() runtime.Object {
	obj := &ext.CloudCredential{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// userFrom extracts user info from context and checks cloud credential access.
// Returns the user info, whether they have full access to cloudcredentials, and
// whether they have the specific verb permission.
func (s *Store) userFrom(ctx context.Context, verb string) (user.Info, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, fmt.Errorf("context has no user info")
	}

	namespace := request.NamespaceValue(ctx)

	// Check for full access on cloudcredentials. This matches callers with
	// wildcard access to this resource, including both global admins and users
	// bound to the cloud-credential-administrator role.
	decision, _, err := s.auth.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            verb,
		APIGroup:        GVR.Group,
		Resource:        "*",
		Namespace:       namespace,
		ResourceRequest: true,
	})

	return userInfo, decision == authorizer.DecisionAllow, err
}

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {
}

// ConvertToTable implements [rest.TableConvertor]
func (s *Store) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return s.tableConverter.ConvertToTable(ctx, obj, tableOptions)
}

// printHandler registers the table printer for CloudCredential objects.
func printHandler(h printers.PrintHandler) {
	columnDefinitions := []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Type", Type: "string", Description: "Type is the cloud provider type"},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "Owner", Type: "string", Priority: 1, Description: "Owner is the user who owns this credential"},
		{Name: "Description", Type: "string", Priority: 1, Description: "Description is a human readable description of the credential"},
	}
	_ = h.TableHandler(columnDefinitions, printCloudCredentialList)
	_ = h.TableHandler(columnDefinitions, printCloudCredential)
}

// printCloudCredential prints a single CloudCredential object as a table row.
func printCloudCredential(credential *ext.CloudCredential, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	owner := "<unknown>"
	if credential.Annotations != nil {
		if ownerAnnotation, ok := credential.Annotations[AnnotationCreatorID]; ok && ownerAnnotation != "" {
			owner = ownerAnnotation
		}
	}
	if owner == "<unknown>" && credential.Labels != nil {
		if ownerLabel, ok := credential.Labels[LabelCloudCredentialOwner]; ok && ownerLabel != "" {
			owner = ownerLabel
		}
	}

	credType := credential.Spec.Type

	return []metav1.TableRow{{
		Object: runtime.RawExtension{Object: credential},
		Cells: []any{
			credential.Name,
			credType,
			translateTimestampSince(credential.CreationTimestamp),
			owner,
			credential.Spec.Description,
		},
	}}, nil
}

// printCloudCredentialList prints a list of CloudCredential objects as table rows.
func printCloudCredentialList(credentialList *ext.CloudCredentialList, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	rows := make([]metav1.TableRow, 0, len(credentialList.Items))
	for i := range credentialList.Items {
		r, err := printCloudCredential(&credentialList.Items[i], options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r...)
	}
	return rows, nil
}

// Interface implementations
var (
	_ rest.Creater = &Store{}
	_ rest.Getter  = &Store{}
	_ rest.Lister  = &Store{}
	//_ rest.Watcher                  = &Store{}
	_ rest.GracefulDeleter          = &Store{}
	_ rest.CollectionDeleter        = &Store{}
	_ rest.Updater                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
	_ rest.TableConvertor           = &Store{}
)
