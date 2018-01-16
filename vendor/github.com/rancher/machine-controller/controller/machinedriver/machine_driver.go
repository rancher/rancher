package machinedriver

import (
	"fmt"
	"strings"
	"sync"

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
	driverNameLabel = "io.cattle.machine_driver.name"
)

func Register(management *config.ManagementContext) {
	machineDriverClient := management.Management.MachineDrivers("")
	machineDriverLifecycle := &Lifecycle{
		machineDriverClient: machineDriverClient,
		schemaClient:        management.Management.DynamicSchemas(""),
		schemaLister:        management.Management.DynamicSchemas("").Controller().Lister(),
	}

	machineDriverClient.
		AddLifecycle("machine-driver-controller", machineDriverLifecycle)
}

type Lifecycle struct {
	machineDriverClient v3.MachineDriverInterface
	schemaClient        v3.DynamicSchemaInterface
	schemaLister        v3.DynamicSchemaLister
}

func (m *Lifecycle) Create(obj *v3.MachineDriver) (*v3.MachineDriver, error) {
	return m.download(obj)
}

func (m *Lifecycle) download(obj *v3.MachineDriver) (*v3.MachineDriver, error) {
	var err error
	// if machine driver was created, we also activate the driver by default
	driver := NewDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	schemaName := obj.Name + "config"
	_, err = m.schemaLister.Get("", schemaName)

	if driver.Exists() && err == nil {
		return obj, nil
	}

	_, err = v3.MachineDriverConditionDownloaded.Do(obj, func() (runtime.Object, error) {
		// update status
		obj, err = m.machineDriverClient.Update(obj)
		if err != nil {
			return nil, err
		}

		if err := driver.Stage(); err != nil {
			return nil, err
		}

		if err := driver.Install(); err != nil {
			logrus.Errorf("Failed to download/install driver %s: %v", driver.Name(), err)
			return nil, err
		}

		obj.Spec.DisplayName = strings.TrimPrefix(driver.Name(), "docker-machine-driver-")
		return obj, nil
	})
	if err != nil {
		return obj, err
	}

	driverName := strings.TrimPrefix(driver.Name(), "docker-machine-driver-")
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
	dynamicSchema.Name = obj.Name + "config"
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
		return nil, err
	}
	return obj, nil
}

func (m *Lifecycle) Updated(obj *v3.MachineDriver) (*v3.MachineDriver, error) {
	var err error

	obj, err = m.download(obj)
	if err != nil {
		return obj, err
	}

	if err := m.createOrUpdateMachineForEmbeddedType(obj.Spec.DisplayName+"config", obj.Spec.DisplayName+"Config", obj.Spec.Active); err != nil {
		return obj, err
	}

	v3.MachineDriverConditionActive.True(obj)
	v3.MachineDriverConditionInactive.True(obj)
	v3.MachineDriverConditionDownloaded.True(obj)

	return obj, nil
}

func (m *Lifecycle) Remove(obj *v3.MachineDriver) (*v3.MachineDriver, error) {
	schemas, err := m.schemaClient.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", driverNameLabel, obj.Spec.DisplayName),
	})
	if err != nil {
		return nil, err
	}
	for _, schema := range schemas.Items {
		logrus.Infof("Deleting schema %s", schema.Name)
		if err := m.schemaClient.Delete(schema.Name, &metav1.DeleteOptions{}); err != nil {
			return nil, err
		}
		logrus.Infof("Deleting schema %s done", schema.Name)
	}
	if err := m.createOrUpdateMachineForEmbeddedType(obj.Spec.DisplayName+"config", obj.Spec.DisplayName+"Config", false); err != nil {
		return nil, err
	}
	return obj, nil
}

func (m *Lifecycle) createOrUpdateMachineForEmbeddedType(embeddedType, fieldName string, embedded bool) error {
	schemaLock.Lock()
	defer schemaLock.Unlock()

	if err := m.createOrUpdateMachineForEmbeddedTypeWithParents(embeddedType, fieldName, "machineconfig", "machine", embedded); err != nil {
		return err
	}

	return m.createOrUpdateMachineForEmbeddedTypeWithParents(embeddedType, fieldName, "machinetemplateconfig", "machineTemplate", embedded)
}

func (m *Lifecycle) createOrUpdateMachineForEmbeddedTypeWithParents(embeddedType, fieldName, schemaID, parentID string, embedded bool) error {
	machineSchema, err := m.schemaLister.Get("", schemaID)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		resourceField := map[string]v3.Field{}
		if embedded {
			resourceField[fieldName] = v3.Field{
				Create:   true,
				Nullable: true,
				Update:   false,
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
		if machineSchema.Spec.ResourceFields == nil {
			machineSchema.Spec.ResourceFields = map[string]v3.Field{}
		}
		if _, ok := machineSchema.Spec.ResourceFields[fieldName]; !ok {
			// if embedded we add the type to schema
			logrus.Infof("uploading %s to machine schema", fieldName)
			machineSchema.Spec.ResourceFields[fieldName] = v3.Field{
				Create:   true,
				Nullable: true,
				Update:   true,
				Type:     embeddedType,
			}
			shouldUpdate = true
		}
	} else {
		// if not we delete it from schema
		if _, ok := machineSchema.Spec.ResourceFields[fieldName]; ok {
			logrus.Infof("deleting %s from machine schema", fieldName)
			delete(machineSchema.Spec.ResourceFields, fieldName)
			shouldUpdate = true
		}
	}

	if shouldUpdate {
		_, err = m.schemaClient.Update(machineSchema)
		if err != nil {
			return err
		}
	}

	return nil
}
