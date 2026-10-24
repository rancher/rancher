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
	// CredentialNamespace is the namespace where backing CloudCredential secrets are stored.
	CredentialNamespace = "cattle-cloud-credentials"

	// SecretTypePrefix prefixes Secret.Type for CloudCredential backing secrets; the suffix is spec.type.
	SecretTypePrefix = "rke.cattle.io/cloud-credential-"

	// CloudCredentialLabel marks Secrets managed by the CloudCredential store.
	CloudCredentialLabel = "cattle.io/cloud-credential"
	// CloudCredentialNameLabel stores the CloudCredential metadata.name on the backing Secret.
	CloudCredentialNameLabel = "cattle.io/cloud-credential-name"
	// CloudCredentialNamespaceLabel stores the CloudCredential namespace while the backing Secret lives in CredentialNamespace.
	CloudCredentialNamespaceLabel = "cattle.io/cloud-credential-namespace"
	// CloudCredentialOwnerLabel stores a sanitized owner value so label selectors can enforce scoped access.
	CloudCredentialOwnerLabel = "cattle.io/cloud-credential-owner"
	// CloudCredentialDescriptionAnnotation stores spec.description on the backing Secret.
	CloudCredentialDescriptionAnnotation = "cattle.io/cloud-credential-description"
	// CreatorIDAnnotation stores the unsanitized creator ID for display and filtering by the UI.
	CreatorIDAnnotation = "field.cattle.io/creatorId"

	// GeneratePrefix is the prefix used for generated backing Secret names.
	GeneratePrefix = "cc-"

	// GenericTypePrefix is the prefix required for generic credential types.
	GenericTypePrefix = "x-"

	// CredentialConfigSuffix is appended to the type to form the credential config schema name.
	// Must be lowercase to match how DynamicSchemas are registered by node drivers and KEv2 operators.
	CredentialConfigSuffix = "credentialconfig"

	// FieldConditions stores marshaled status.conditions in backing Secret data.
	FieldConditions = "conditions"
	// FieldVisibleFields stores the per-credential visible field override in backing Secret data.
	FieldVisibleFields = "visibleFields"

	// SingularName is the storage singular for CloudCredential resources.
	SingularName = "cloudcredential"
	// PluralName is the storage plural for CloudCredential resources.
	PluralName = SingularName + "s"

	unknownOwnerValue = "<unknown>"
)

var (
	GV = schema.GroupVersion{
		Group:   "ext.cattle.io",
		Version: "v1",
	}

	GR = schema.GroupResource{
		Group:    GV.Group,
		Resource: PluralName,
	}

	GVK = schema.GroupVersionKind{
		Group:   GV.Group,
		Version: GV.Version,
		Kind:    "CloudCredential",
	}

	GVR = schema.GroupVersionResource{
		Group:    GV.Group,
		Version:  GV.Version,
		Resource: PluralName,
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

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {
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

// ConvertToTable implements [rest.TableConvertor]
func (s *Store) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return s.tableConverter.ConvertToTable(ctx, obj, tableOptions)
}

// printHandler registers the table printer for CloudCredential objects.
func printHandler(h printers.PrintHandler) {
	columnDefinitions := []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Type", Type: "string", Description: "Type specifies the service the credential can authenticate with"},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "Owner", Type: "string", Priority: 1, Description: "Owner is the user who owns this credential"},
		{Name: "Description", Type: "string", Priority: 1, Description: "Description is a human readable description of the credential"},
	}
	_ = h.TableHandler(columnDefinitions, cloudCredentialListToTableRows)
	_ = h.TableHandler(columnDefinitions, cloudCredentialToTableRows)
}

// cloudCredentialToTableRows converts a single CloudCredential into table rows.
func cloudCredentialToTableRows(credential *ext.CloudCredential, _ printers.GenerateOptions) ([]metav1.TableRow, error) {
	owner := unknownOwnerValue
	if credential.Annotations != nil {
		if ownerAnnotation, ok := credential.Annotations[CreatorIDAnnotation]; ok && ownerAnnotation != "" {
			owner = ownerAnnotation
		}
	}
	if owner == unknownOwnerValue && credential.Labels != nil {
		if ownerLabel, ok := credential.Labels[CloudCredentialOwnerLabel]; ok && ownerLabel != "" {
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

// cloudCredentialListToTableRows converts a CloudCredential list into table rows.
func cloudCredentialListToTableRows(credentialList *ext.CloudCredentialList, options printers.GenerateOptions) ([]metav1.TableRow, error) {
	rows := make([]metav1.TableRow, 0, len(credentialList.Items))
	for i := range credentialList.Items {
		r, err := cloudCredentialToTableRows(&credentialList.Items[i], options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, r...)
	}
	return rows, nil
}

// Interface implementations
var (
	_ rest.Creater                  = &Store{}
	_ rest.Getter                   = &Store{}
	_ rest.Lister                   = &Store{}
	_ rest.Watcher                  = &Store{}
	_ rest.GracefulDeleter          = &Store{}
	_ rest.CollectionDeleter        = &Store{}
	_ rest.Updater                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
	_ rest.TableConvertor           = &Store{}
)
