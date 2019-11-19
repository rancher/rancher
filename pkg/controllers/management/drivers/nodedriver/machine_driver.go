package nodedriver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"

	errs "github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	SchemaLock = sync.Mutex{}
	driverLock = sync.Mutex{}
	// Aliases maps Driver field => schema field
	// The opposite of this lives in pkg/controllers/management/node/controller.go
	Aliases = map[string]map[string]string{
		"aliyunecs":     map[string]string{"sshKeypath": "sshKeyContents"},
		"amazonec2":     map[string]string{"sshKeypath": "sshKeyContents", "userdata": "userdata"},
		"azure":         map[string]string{"customData": "customData"},
		"digitalocean":  map[string]string{"sshKeyPath": "sshKeyContents", "userdata": "userdata"},
		"exoscale":      map[string]string{"sshKey": "sshKey", "userdata": "userdata"},
		"openstack":     map[string]string{"cacert": "cacert", "privateKeyFile": "privateKeyFile", "userDataFile": "userDataFile"},
		"otc":           map[string]string{"privateKeyFile": "privateKeyFile"},
		"packet":        map[string]string{"userdata": "userdata"},
		"vmwarevsphere": map[string]string{"cloud-config": "cloudConfig"},
	}
	SSHKeyFields = map[string]bool{
		"sshKeyContents": true,
		"sshKey":         true,
		"privateKeyFile": true,
	}
)

const (
	driverNameLabel  = "io.cattle.node_driver.name"
	uiFieldHintsAnno = "io.cattle.nodedriver/ui-field-hints"
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

	version, err := getRancherMachineVersion()
	if err != nil {
		logrus.Warnf("error getting rancher-machine version: %v", err)
	}
	nodeDriverLifecycle.dockerMachineVersion = version

	nodeDriverClient.AddLifecycle(ctx, "node-driver-controller", nodeDriverLifecycle)
}

type Lifecycle struct {
	nodeDriverClient     v3.NodeDriverInterface
	schemaClient         v3.DynamicSchemaInterface
	schemaLister         v3.DynamicSchemaLister
	secretStore          v1.SecretInterface
	nsStore              v1.NamespaceInterface
	schemas              *types.Schemas
	dockerMachineVersion string
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

	forceUpdate := m.checkDriverVersion(obj)

	err := errs.New("not found")
	// if node driver was created, we also activate the driver by default
	driver := drivers.NewDynamicDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	schemaName := obj.Spec.DisplayName + "config"
	var existingSchema *v3.DynamicSchema
	if obj.Spec.DisplayName != "" {
		existingSchema, err = m.schemaLister.Get("", schemaName)
	}

	if driver.Exists() && err == nil && !forceUpdate {
		// add credential schema
		credFields := map[string]v3.Field{}
		if err != nil {
			logrus.Errorf("error getting schema %v", err)
		}
		pubCredFields, privateCredFields, passwordFields, defaults := getCredFields(obj.Annotations)
		for name, field := range existingSchema.Spec.ResourceFields {
			if SSHKeyFields[name] || passwordFields[name] || privateCredFields[name] {
				if field.Type != "password" {
					forceUpdate = true
					break
				}
			}
			// even if forceUpdate is false, calculate credFields to check if credSchema needs to be updated
			if privateCredFields[name] || pubCredFields[name] {
				credField := field
				credField.Required = true
				if val, ok := defaults[name]; ok {
					credField = updateDefault(credField, val, field.Type)
				}
				credFields[name] = credField
			}
		}
		if !forceUpdate {
			return m.createCredSchema(obj, credFields)
		}
	}

	if !driver.Exists() || forceUpdate {
		v3.NodeDriverConditionDownloaded.Unknown(obj)
		v3.NodeDriverConditionInstalled.Unknown(obj)
	}

	newObj, err := v3.NodeDriverConditionDownloaded.Once(obj, func() (runtime.Object, error) {
		// update status
		obj, err = m.nodeDriverClient.Update(obj)
		if err != nil {
			return nil, err
		}

		if err := driver.Stage(forceUpdate); err != nil {
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
		if err = driver.Executable(); err != nil {
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

	obj = m.addVersionInfo(obj)

	obj, err = m.addUIHintsAnno(driverName, obj)
	if err != nil {
		return obj, errs.Wrap(err, "failed JSON in addUIHintsAnno")
	}

	flags, err := getCreateFlagsForDriver(driverName)
	if err != nil {
		return nil, err
	}
	credFields := map[string]v3.Field{}
	resourceFields := map[string]v3.Field{}
	pubCredFields, privateCredFields, passwordFields, defaults := getCredFields(obj.Annotations)
	for _, flag := range flags {
		name, field, err := FlagToField(flag)
		if err != nil {
			return nil, err
		}
		if aliases, ok := Aliases[driverName]; ok {
			// convert path fields to their alias to take file contents
			if alias, ok := aliases[name]; ok {
				name = alias
				field.Description = fmt.Sprintf("File contents for %v", alias)
			}
		}

		if privateCredFields[name] || passwordFields[name] || SSHKeyFields[name] {
			field.Type = "password"
		}

		if pubCredFields[name] || privateCredFields[name] {
			credField := field
			credField.Required = true
			if val, ok := defaults[name]; ok {
				credField = updateDefault(credField, val, field.Type)
			}
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
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return obj, err
		}
		ds, err := m.schemaClient.Get(dynamicSchema.Name, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}
		ds.Spec.ResourceFields = resourceFields

		_, err = m.schemaClient.Update(ds)
		if err != nil {
			return obj, err
		}
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

func (m *Lifecycle) checkDriverVersion(obj *v3.NodeDriver) bool {
	if v3.NodeDriverConditionDownloaded.IsUnknown(obj) || v3.NodeDriverConditionInstalled.IsUnknown(obj) {
		return true
	}

	driverName := strings.TrimPrefix(obj.Spec.DisplayName, drivers.DockerMachineDriverPrefix)

	if _, ok := Aliases[driverName]; ok {
		if val, ok := obj.Annotations[uiFieldHintsAnno]; !ok || val == "" {
			return true
		}
	}

	// Builtin drivers use the rancher-machine version to validate against
	if obj.Spec.Builtin {
		if obj.Status.AppliedDockerMachineVersion != m.dockerMachineVersion {
			return true
		}
		return false
	}

	if obj.Spec.URL != obj.Status.AppliedURL || obj.Spec.Checksum != obj.Status.AppliedChecksum {
		return true
	}

	return false
}

func (m *Lifecycle) addVersionInfo(obj *v3.NodeDriver) *v3.NodeDriver {
	if obj.Spec.Builtin {
		obj.Status.AppliedDockerMachineVersion = m.dockerMachineVersion
	} else {
		obj.Status.AppliedURL = obj.Spec.URL
		obj.Status.AppliedChecksum = obj.Spec.Checksum
	}
	return obj
}

func (m *Lifecycle) addUIHintsAnno(driverName string, obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	if aliases, ok := Aliases[driverName]; ok {
		anno := make(map[string]map[string]string)

		for _, aliased := range aliases {
			anno[aliased] = map[string]string{
				"type": "multiline",
			}
		}

		jsonAnno, err := json.Marshal(anno)
		if err != nil {
			return obj, err
		}

		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}

		obj.Annotations[uiFieldHintsAnno] = string(jsonAnno)
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

func getCredFields(annotations map[string]string) (map[string]bool, map[string]bool, map[string]bool, map[string]string) {
	getMap := func(fields string) map[string]bool {
		data := map[string]bool{}
		for _, field := range strings.Split(fields, ",") {
			data[field] = true
		}
		return data
	}
	getDefaults := func(fields string) map[string]string {
		data := map[string]string{}
		for _, pattern := range strings.Split(fields, ",") {
			split := strings.SplitN(pattern, ":", 2)
			if len(split) == 2 {
				data[split[0]] = split[1]
			}
		}
		return data
	}
	return getMap(annotations["publicCredentialFields"]),
		getMap(annotations["privateCredentialFields"]),
		getMap(annotations["passwordFields"]),
		getDefaults(annotations["defaults"])
}

func credentialConfigSchemaName(driverName string) string {
	return fmt.Sprintf("%s%s", driverName, "credentialconfig")
}

func updateDefault(credField v3.Field, val, kind string) v3.Field {
	switch kind {
	case "int":
		i, err := strconv.Atoi(val)
		if err == nil {
			credField.Default = v3.Values{IntValue: i}
		} else {
			logrus.Errorf("error converting %s to int %v", val, err)
		}
	case "boolean":
		credField.Default = v3.Values{BoolValue: convert.ToBool(val)}
	case "array[string]":
		credField.Default = v3.Values{StringSliceValue: convert.ToStringSlice(val)}
	case "password", "string":
		credField.Default = v3.Values{StringValue: val}
	default:
		logrus.Errorf("unsupported kind for default val:%s kind:%s", val, kind)
	}
	return credField
}

func getRancherMachineVersion() (string, error) {
	cmd := exec.Command("rancher-machine", "--version")
	out, err := cmd.CombinedOutput()
	return string(out), err
}
