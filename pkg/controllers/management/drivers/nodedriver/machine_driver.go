package nodedriver

import (
	"context"
	"fmt"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"reflect"
	"strings"
	"sync"

	"github.com/rancher/norman/types"

	errs "github.com/pkg/errors"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	SchemaLock = sync.Mutex{}
	driverLock = sync.Mutex{}
)

const (
	driverNameLabel = "io.cattle.node_driver.name"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	nodeDriverClient := management.Management.NodeDrivers("")
	nodeDriverLifecycle := &Lifecycle{
		nodeDriverClient: nodeDriverClient,
		schemaClient:     management.Management.DynamicSchemas(""),
		schemaLister:     management.Management.DynamicSchemas("").Controller().Lister(),
		secretStore:      management.Core.Secrets(""),
		nsStore:          management.Core.Namespaces(""),
		schemas:          management.Schemas,
	}

	nodeDriverClient.
		AddLifecycle(ctx, "node-driver-controller", nodeDriverLifecycle)
}

type Lifecycle struct {
	nodeDriverClient v3.NodeDriverInterface
	schemaClient     v3.DynamicSchemaInterface
	schemaLister     v3.DynamicSchemaLister
	secretStore      v1.SecretInterface
	nsStore          v1.NamespaceInterface
	schemas          *types.Schemas
}

func (m *Lifecycle) Create(obj *v3.NodeDriver) (runtime.Object, error) {
	return m.download(obj)
}

func (m *Lifecycle) download(obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	driverLock.Lock()
	defer driverLock.Unlock()
	if !obj.Spec.Active {
		return obj, nil
	}

	err := errs.New("not found")
	// if node driver was created, we also activate the driver by default
	driver := drivers.NewDynamicDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	schemaName := obj.Spec.DisplayName + "config"
	var existingSchema *v3.DynamicSchema
	if obj.Spec.DisplayName != "" {
		existingSchema, err = m.schemaLister.Get("", schemaName)
	}

	if driver.Exists() && err == nil {
		// add credential schema
		credFields := map[string]v3.Field{}
		if err != nil {
			logrus.Errorf("error getting schema %v", err)
		}
		userCredFields, pwdFields := getCredFields(obj.Annotations)
		for name, field := range existingSchema.Spec.ResourceFields {
			if pwdFields[name] {
				field.Type = "password"
			}
			if field.Type == "password" || userCredFields[name] {
				credField := field
				credField.Required = true
				credFields[name] = credField
			}
		}
		return m.createCredSchema(obj, credFields)
	}

	if !driver.Exists() {
		v3.NodeDriverConditionDownloaded.Unknown(obj)
		v3.NodeDriverConditionInstalled.Unknown(obj)
	}

	newObj, err := v3.NodeDriverConditionDownloaded.Once(obj, func() (runtime.Object, error) {
		// update status
		obj, err = m.nodeDriverClient.Update(obj)
		if err != nil {
			return nil, err
		}

		if err := driver.Stage(); err != nil {
			return nil, err
		}
		return obj, nil
	})
	if err != nil {
		return obj, err
	}

	obj = newObj.(*v3.NodeDriver)
	newObj, err = v3.NodeDriverConditionInstalled.Once(obj, func() (runtime.Object, error) {
		if err := driver.Install(); err != nil {
			return nil, err
		}
		if err = driver.Excutable(); err != nil {
			return nil, err
		}
		obj.Spec.DisplayName = strings.TrimPrefix(driver.Name(), drivers.DockerMachineDriverPrefix)
		return obj, nil
	})
	if err != nil {
		return newObj.(*v3.NodeDriver), err
	}

	obj = newObj.(*v3.NodeDriver)
	driverName := strings.TrimPrefix(driver.Name(), drivers.DockerMachineDriverPrefix)
	flags, err := getCreateFlagsForDriver(driverName)
	if err != nil {
		return nil, err
	}
	credFields := map[string]v3.Field{}
	resourceFields := map[string]v3.Field{}
	userCredFields, pwdFields := getCredFields(obj.Annotations)
	for _, flag := range flags {
		name, field, err := FlagToField(flag)
		if err != nil {
			return nil, err
		}
		if pwdFields[name] {
			field.Type = "password"
		}
		if field.Type == "password" || userCredFields[name] {
			credField := field
			credField.Required = true
			credFields[name] = credField
		}
		resourceFields[name] = field
	}

	dynamicSchema := &v3.DynamicSchema{
		Spec: v3.DynamicSchemaSpec{
			ResourceFields: resourceFields,
		},
	}
	dynamicSchema.Name = obj.Spec.DisplayName + "config"
	dynamicSchema.OwnerReferences = []metav1.OwnerReference{
		{
			UID:        obj.UID,
			Kind:       obj.Kind,
			APIVersion: obj.APIVersion,
			Name:       obj.Name,
		},
	}
	dynamicSchema.Labels = map[string]string{}
	dynamicSchema.Labels[driverNameLabel] = obj.Spec.DisplayName
	_, err = m.schemaClient.Create(dynamicSchema)
	if err != nil && !errors.IsAlreadyExists(err) {
		return obj, err
	}
	return m.createCredSchema(obj, credFields)
}

func (m *Lifecycle) createCredSchema(obj *v3.NodeDriver, credFields map[string]v3.Field) (*v3.NodeDriver, error) {
	name := credentialConfigSchemaName(obj.Spec.DisplayName)
	credSchema, err := m.schemaLister.Get("", name)
	if err != nil {
		if errors.IsNotFound(err) {
			credentialSchema := &v3.DynamicSchema{
				Spec: v3.DynamicSchemaSpec{
					ResourceFields: credFields,
				},
			}
			credentialSchema.Name = name
			credentialSchema.OwnerReferences = []metav1.OwnerReference{
				{
					UID:        obj.UID,
					Kind:       obj.Kind,
					APIVersion: obj.APIVersion,
					Name:       obj.Name,
				},
			}
			_, err := m.schemaClient.Create(credentialSchema)
			return obj, err
		}
		return obj, err
	} else if !reflect.DeepEqual(credSchema.Spec.ResourceFields, credFields) {
		toUpdate := credSchema.DeepCopy()
		toUpdate.Spec.ResourceFields = credFields
		_, err := m.schemaClient.Update(toUpdate)
		if err != nil {
			return obj, err
		}
	}
	return obj, nil
}

func (m *Lifecycle) Updated(obj *v3.NodeDriver) (runtime.Object, error) {
	var err error

	obj, err = m.download(obj)
	if err != nil {
		return obj, err
	}

	if err := m.createOrUpdateNodeForEmbeddedType(obj.Spec.DisplayName+"config", obj.Spec.DisplayName+"Config", obj.Spec.Active); err != nil {
		return obj, err
	}

	if err := m.createOrUpdateNodeForEmbeddedTypeCredential(credentialConfigSchemaName(obj.Spec.DisplayName),
		obj.Spec.DisplayName+"credentialConfig", obj.Spec.Active); err != nil {
		return obj, err
	}

	v3.NodeDriverConditionActive.True(obj)
	v3.NodeDriverConditionInactive.True(obj)

	return obj, nil
}

func (m *Lifecycle) Remove(obj *v3.NodeDriver) (runtime.Object, error) {
	schemas, err := m.schemaClient.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", driverNameLabel, obj.Spec.DisplayName),
	})
	if err != nil {
		return obj, err
	}
	for _, schema := range schemas.Items {
		logrus.Infof("Deleting schema %s", schema.Name)
		if err := m.schemaClient.Delete(schema.Name, &metav1.DeleteOptions{}); err != nil {
			return obj, err
		}
		logrus.Infof("Deleting schema %s done", schema.Name)
	}
	if err := m.createOrUpdateNodeForEmbeddedType(obj.Spec.DisplayName+"config", obj.Spec.DisplayName+"Config", false); err != nil {
		return obj, err
	}
	return obj, nil
}

func (m *Lifecycle) createOrUpdateNodeForEmbeddedTypeCredential(embeddedType, fieldName string, embedded bool) error {
	SchemaLock.Lock()
	defer SchemaLock.Unlock()

	return m.createOrUpdateNodeForEmbeddedTypeWithParents(embeddedType, fieldName, "credentialconfig", "cloudCredential", embedded, true)
}

func (m *Lifecycle) createOrUpdateNodeForEmbeddedType(embeddedType, fieldName string, embedded bool) error {
	SchemaLock.Lock()
	defer SchemaLock.Unlock()

	if err := m.createOrUpdateNodeForEmbeddedTypeWithParents(embeddedType, fieldName, "nodeconfig", "node", embedded, false); err != nil {
		return err
	}

	return m.createOrUpdateNodeForEmbeddedTypeWithParents(embeddedType, fieldName, "nodetemplateconfig", "nodeTemplate", embedded, true)
}

func (m *Lifecycle) createOrUpdateNodeForEmbeddedTypeWithParents(embeddedType, fieldName, schemaID, parentID string, embedded, update bool) error {
	nodeSchema, err := m.schemaLister.Get("", schemaID)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		resourceField := map[string]v3.Field{}
		if embedded {
			resourceField[fieldName] = v3.Field{
				Create:   true,
				Nullable: true,
				Update:   update,
				Type:     embeddedType,
			}
		}
		dynamicSchema := &v3.DynamicSchema{}
		dynamicSchema.Name = schemaID
		dynamicSchema.Spec.ResourceFields = resourceField
		dynamicSchema.Spec.Embed = true
		dynamicSchema.Spec.EmbedType = parentID
		_, err := m.schemaClient.Create(dynamicSchema)
		if err != nil {
			return err
		}
		return nil
	}

	shouldUpdate := false
	if embedded {
		if nodeSchema.Spec.ResourceFields == nil {
			nodeSchema.Spec.ResourceFields = map[string]v3.Field{}
		}
		if _, ok := nodeSchema.Spec.ResourceFields[fieldName]; !ok {
			// if embedded we add the type to schema
			logrus.Infof("uploading %s to %s schema", fieldName, schemaID)
			nodeSchema.Spec.ResourceFields[fieldName] = v3.Field{
				Create:   true,
				Nullable: true,
				Update:   update,
				Type:     embeddedType,
			}
			shouldUpdate = true
		}
	} else {
		// if not we delete it from schema
		if _, ok := nodeSchema.Spec.ResourceFields[fieldName]; ok {
			logrus.Infof("deleting %s from %s schema", fieldName, schemaID)
			delete(nodeSchema.Spec.ResourceFields, fieldName)
			shouldUpdate = true
		}
	}

	if shouldUpdate {
		_, err = m.schemaClient.Update(nodeSchema)
		if err != nil {
			return err
		}
	}

	return nil
}

func getCredFields(annotations map[string]string) (map[string]bool, map[string]bool) {
	getMap := func(fields string) map[string]bool {
		data := map[string]bool{}
		for _, field := range strings.Split(fields, ",") {
			data[field] = true
		}
		return data
	}
	return getMap(annotations["cred"]), getMap(annotations["password"])
}

func credentialConfigSchemaName(driverName string) string {
	return fmt.Sprintf("%s%s", driverName, "credentialconfig")
}
