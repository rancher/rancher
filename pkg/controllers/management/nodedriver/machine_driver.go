package nodedriver

import (
	"fmt"
	"strings"
	"sync"

	errs "github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	schemaLock = sync.Mutex{}
)

const (
	driverNameLabel = "io.cattle.node_driver.name"
)

func Register(management *config.ManagementContext) {
	nodeDriverClient := management.Management.NodeDrivers("")
	nodeDriverLifecycle := &Lifecycle{
		nodeDriverClient: nodeDriverClient,
		schemaClient:     management.Management.DynamicSchemas(""),
		schemaLister:     management.Management.DynamicSchemas("").Controller().Lister(),
	}

	nodeDriverClient.
		AddLifecycle("node-driver-controller", nodeDriverLifecycle)
}

type Lifecycle struct {
	nodeDriverClient v3.NodeDriverInterface
	schemaClient     v3.DynamicSchemaInterface
	schemaLister     v3.DynamicSchemaLister
}

func (m *Lifecycle) Create(obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	return m.download(obj)
}

func (m *Lifecycle) download(obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	if !obj.Spec.Active {
		return obj, nil
	}

	err := errs.New("not found")
	// if node driver was created, we also activate the driver by default
	driver := NewDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	if obj.Spec.DisplayName != "" {
		schemaName := obj.Spec.DisplayName + "config"
		_, err = m.schemaLister.Get("", schemaName)
	}

	if driver.Exists() && err == nil {
		return obj, nil
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
		obj.Spec.DisplayName = strings.TrimPrefix(driver.Name(), dockerMachineDriverPrefix)
		return obj, nil
	})
	if err != nil {
		return newObj.(*v3.NodeDriver), err
	}

	obj = newObj.(*v3.NodeDriver)
	driverName := strings.TrimPrefix(driver.Name(), dockerMachineDriverPrefix)
	flags, err := getCreateFlagsForDriver(driverName)
	if err != nil {
		return nil, err
	}
	resourceFields := map[string]v3.Field{}
	for _, flag := range flags {
		name, field, err := flagToField(flag)
		if err != nil {
			return nil, err
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
	return obj, nil
}

func (m *Lifecycle) Updated(obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	var err error

	obj, err = m.download(obj)
	if err != nil {
		return obj, err
	}

	if err := m.createOrUpdateNodeForEmbeddedType(obj.Spec.DisplayName+"config", obj.Spec.DisplayName+"Config", obj.Spec.Active); err != nil {
		return obj, err
	}

	v3.NodeDriverConditionActive.True(obj)
	v3.NodeDriverConditionInactive.True(obj)

	return obj, nil
}

func (m *Lifecycle) Remove(obj *v3.NodeDriver) (*v3.NodeDriver, error) {
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

func (m *Lifecycle) createOrUpdateNodeForEmbeddedType(embeddedType, fieldName string, embedded bool) error {
	schemaLock.Lock()
	defer schemaLock.Unlock()

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
			logrus.Infof("uploading %s to node schema", fieldName)
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
			logrus.Infof("deleting %s from node schema", fieldName)
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
