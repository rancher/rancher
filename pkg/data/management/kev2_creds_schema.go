package management

import (
	"fmt"
	"reflect"
	"sort"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	credentialConfigSchemaID = "credentialconfig"
	cloudCredentialSchemaID  = "cloudCredential"
)

type CredentialFields map[string]v32.Field

// Credential Fields data for KEv2 Operators which don't have a corresponding node driver.
var KEv2OperatorsCredentialFields = map[string]CredentialFields{
	AlibabaOperator: {
		"accessKeyId": v32.Field{
			Create: true,
			Update: true,
			Type:   "string",
		},
		"accessKeySecret": v32.Field{
			Create: true,
			Update: true,
			Type:   "password",
		},
	},
}

type KEv2CredsSchemaHandler struct {
	schemaLister v3.DynamicSchemaLister
	schemaClient v3.DynamicSchemaInterface
}

func addKev2OperatorCredsSchemas(management *config.ManagementContext) error {
	schemaHandler := KEv2CredsSchemaHandler{
		schemaLister: management.Management.DynamicSchemas("").Controller().Lister(),
		schemaClient: management.Management.DynamicSchemas(""),
	}
	for operatorName, credsField := range KEv2OperatorsCredentialFields {
		err := schemaHandler.createOrUpdateCredSchema(operatorName, credsField)
		if err != nil {
			return err
		}

		err = schemaHandler.addEmbeddedCredentialConfigField(credentialConfigSchemaName(operatorName),
			operatorName+"credentialConfig")
		if err != nil {
			return err
		}
	}
	return nil
}

func (csh *KEv2CredsSchemaHandler) createOrUpdateCredSchema(operatorName string, credFields map[string]v32.Field) error {
	name := credentialConfigSchemaName(operatorName)
	publicFields := derivePublicFields(credFields)
	privateFields := derivePrivateFields(credFields)
	credSchema, err := csh.schemaLister.Get("", name)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Infof("creating %s schema", name)

			credentialSchema := &v32.DynamicSchema{
				Spec: v32.DynamicSchemaSpec{
					ResourceFields: credFields,
					PublicFields:   publicFields,
					PrivateFields:  privateFields,
				},
			}
			credentialSchema.Name = name
			_, err := csh.schemaClient.Create(credentialSchema)
			return err
		}
		return err
	}

	needsUpdate := !reflect.DeepEqual(credSchema.Spec.ResourceFields, credFields) ||
		!reflect.DeepEqual(credSchema.Spec.PublicFields, publicFields) ||
		!reflect.DeepEqual(credSchema.Spec.PrivateFields, privateFields)
	if needsUpdate {
		toUpdate := credSchema.DeepCopy()
		toUpdate.Spec.ResourceFields = credFields
		toUpdate.Spec.PublicFields = publicFields
		toUpdate.Spec.PrivateFields = privateFields
		_, err := csh.schemaClient.Update(toUpdate)
		if err != nil {
			return err
		}
	}

	return nil
}

// derivePublicFields returns a sorted list of field names whose type is not
// "password". This provides a default PublicFields list for credential schemas
// that don't have explicit public/private annotations.
func derivePublicFields(credFields map[string]v32.Field) []string {
	var publicFields []string
	for name, field := range credFields {
		if field.Type != "password" {
			publicFields = append(publicFields, name)
		}
	}
	sort.Strings(publicFields)
	return publicFields
}

// derivePrivateFields returns a sorted list of field names whose type is
// "password". This provides a default PrivateFields list for credential schemas
// that don't have explicit public/private annotations.
func derivePrivateFields(credFields map[string]v32.Field) []string {
	var privateFields []string
	for name, field := range credFields {
		if field.Type == "password" {
			privateFields = append(privateFields, name)
		}
	}
	sort.Strings(privateFields)
	return privateFields
}

func credentialConfigSchemaName(operatorName string) string {
	return fmt.Sprintf("%s%s", operatorName, "credentialconfig")
}

func (csh *KEv2CredsSchemaHandler) addEmbeddedCredentialConfigField(embeddedType, fieldName string) error {
	nodeSchema, err := csh.schemaLister.Get("", credentialConfigSchemaID)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		logrus.Infof("creating %s schema (parent: %s) with field: %s", credentialConfigSchemaID, cloudCredentialSchemaID, fieldName)

		resourceField := map[string]v32.Field{}
		resourceField[fieldName] = v32.Field{
			Create:   true,
			Nullable: true,
			Update:   true,
			Type:     embeddedType,
		}

		dynamicSchema := &v32.DynamicSchema{}
		dynamicSchema.Name = credentialConfigSchemaID
		dynamicSchema.Spec.ResourceFields = resourceField
		dynamicSchema.Spec.Embed = true
		dynamicSchema.Spec.EmbedType = cloudCredentialSchemaID
		_, err := csh.schemaClient.Create(dynamicSchema)
		if err != nil {
			return err
		}
		return nil
	}

	nodeSchema = nodeSchema.DeepCopy()
	if nodeSchema.Spec.ResourceFields == nil {
		nodeSchema.Spec.ResourceFields = map[string]v32.Field{}
	}

	if _, ok := nodeSchema.Spec.ResourceFields[fieldName]; !ok {
		logrus.Infof("uploading %s to %s schema", fieldName, credentialConfigSchemaID)

		nodeSchema.Spec.ResourceFields[fieldName] = v32.Field{
			Create:   true,
			Nullable: true,
			Update:   true,
			Type:     embeddedType,
		}

		_, err = csh.schemaClient.Update(nodeSchema)
		if err != nil {
			return err
		}
	}

	return nil
}
