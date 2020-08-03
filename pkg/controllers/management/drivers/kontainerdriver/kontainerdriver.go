package kontainerdriver

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	errorsutil "github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/controllers/management/drivers/nodedriver"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	driverNameLabel = "io.cattle.kontainer_driver.name"
	DriverDir       = "./management-state/kontainer-drivers/"
)

var kontainerDriverName = regexp.MustCompile("kontainer-engine-driver-(.+)$")

func Register(ctx context.Context, management *config.ManagementContext) {
	lifecycle := &Lifecycle{
		dynamicSchemas:       management.Management.DynamicSchemas(""),
		dynamicSchemasLister: management.Management.DynamicSchemas("").Controller().Lister(),
		namespaces:           management.Core.Namespaces(""),
		coreV1:               management.Core,
	}

	management.Management.KontainerDrivers("").AddLifecycle(ctx, "mgmt-kontainer-driver-lifecycle", lifecycle)
}

type Lifecycle struct {
	dynamicSchemas       v3.DynamicSchemaInterface
	dynamicSchemasLister v3.DynamicSchemaLister
	namespaces           v1.NamespaceInterface
	coreV1               corev1.Interface
}

func (l *Lifecycle) Create(obj *v3.KontainerDriver) (runtime.Object, error) {
	logrus.Infof("create kontainerdriver %v", obj.Name)

	// return early if driver is not active
	// set driver to a non-transitioning state
	if !obj.Spec.Active {
		v32.KontainerDriverConditionInactive.True(obj)
		return obj, nil
	}

	if obj.Spec.BuiltIn {
		v32.KontainerDriverConditionDownloaded.True(obj)
		v32.KontainerDriverConditionInstalled.True(obj)
	} else {
		v32.KontainerDriverConditionDownloaded.Unknown(obj)
		v32.KontainerDriverConditionInstalled.Unknown(obj)
	}

	if hasStaticSchema(obj) {
		v32.KontainerDriverConditionActive.True(obj)
	} else {
		v32.KontainerDriverConditionActive.Unknown(obj)
	}

	return obj, nil
}

func (l *Lifecycle) driverExists(obj *v3.KontainerDriver) bool {
	return drivers.NewKontainerDriver(obj.Spec.BuiltIn, obj.Status.DisplayName, obj.Spec.URL, obj.Spec.Checksum).Exists()
}

func (l *Lifecycle) download(obj *v3.KontainerDriver) (*v3.KontainerDriver, error) {
	driver := drivers.NewKontainerDriver(obj.Spec.BuiltIn, obj.Status.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	err := driver.Stage(false)
	if err != nil {
		return nil, err
	}

	v32.KontainerDriverConditionDownloaded.True(obj)

	path, err := driver.Install()
	if err != nil {
		return nil, err
	}

	v32.KontainerDriverConditionInstalled.True(obj)

	obj.Status.ExecutablePath = path
	matches := kontainerDriverName.FindStringSubmatch(path)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not parse name of kontainer driver from path: %v", path)
	}

	obj.Status.DisplayName = matches[1]
	obj.Status.ActualURL = obj.Spec.URL

	logrus.Infof("kontainerdriver %v downloaded and registered at %v", obj.Name, path)

	return obj, nil
}

func (l *Lifecycle) createDynamicSchema(obj *v3.KontainerDriver) error {
	resourceFields, err := l.getResourceFields(obj)
	if err != nil {
		return err
	}

	dynamicSchema := &v3.DynamicSchema{
		Spec: v32.DynamicSchemaSpec{
			SchemaName:     getDynamicTypeName(obj),
			ResourceFields: resourceFields,
		},
	}
	dynamicSchema.Name = strings.ToLower(getDynamicTypeName(obj))
	dynamicSchema.OwnerReferences = []v13.OwnerReference{
		{
			UID:        obj.UID,
			Kind:       obj.Kind,
			APIVersion: obj.APIVersion,
			Name:       obj.Name,
		},
	}
	dynamicSchema.Labels = map[string]string{}
	dynamicSchema.Labels[obj.Name] = obj.Status.DisplayName
	dynamicSchema.Labels[driverNameLabel] = obj.Status.DisplayName
	_, err = l.dynamicSchemas.Create(dynamicSchema)
	if err != nil && !errors.IsAlreadyExists(err) {
		return errorsutil.WithMessage(err, "error creating dynamic schema")
	}

	return l.createOrUpdateKontainerDriverTypes(obj)
}

func (l *Lifecycle) createOrUpdateDynamicSchema(obj *v3.KontainerDriver) error {
	dynamicSchema, err := l.dynamicSchemasLister.Get("", strings.ToLower(getDynamicTypeName(obj)))
	if errors.IsNotFound(err) {
		return l.createDynamicSchema(obj)
	}
	if err != nil {
		return err

	}

	return l.updateDynamicSchema(dynamicSchema, obj)
}

func (l *Lifecycle) updateDynamicSchema(dynamicSchema *v3.DynamicSchema, obj *v3.KontainerDriver) error {
	dynamicSchema = dynamicSchema.DeepCopy()

	fields, err := l.getResourceFields(obj)
	if err != nil {
		return err
	}

	dynamicSchema.Spec.ResourceFields = fields

	logrus.Infof("dynamic schema for kontainerdriver %v updating", obj.Name)

	if _, err = l.dynamicSchemas.Update(dynamicSchema); err != nil {
		return err
	}

	return l.createOrUpdateKontainerDriverTypes(obj)
}

func (l *Lifecycle) getResourceFields(obj *v3.KontainerDriver) (map[string]v32.Field, error) {
	driver := service.NewEngineService(
		clusterprovisioner.NewPersistentStore(l.namespaces, l.coreV1),
	)
	flags, err := driver.GetDriverCreateOptions(context.Background(), obj.Name, obj, v32.ClusterSpec{
		GenericEngineConfig: &v32.MapStringInterface{
			clusterprovisioner.DriverNameField: obj.Status.DisplayName,
		},
	})
	if err != nil {
		return nil, err
	}

	resourceFields := map[string]v32.Field{}
	for key, flag := range flags.Options {
		formattedName, field, err := toResourceField(key, flag)
		if err != nil {
			return nil, errorsutil.WithMessage(err, "error formatting field name")
		}

		resourceFields[formattedName] = field
	}

	// all drivers need a driverName field so kontainer-engine knows what type they are
	resourceFields[clusterprovisioner.DriverNameField] = v32.Field{
		Create: true,
		Update: true,
		Type:   "string",
		Default: v32.Values{
			StringValue: obj.Name,
		},
	}

	return resourceFields, nil
}

func (l *Lifecycle) createOrUpdateKontainerDriverTypes(obj *v3.KontainerDriver) error {
	nodedriver.SchemaLock.Lock()
	defer nodedriver.SchemaLock.Unlock()

	embeddedType := getDynamicTypeName(obj)
	fieldName := getDynamicFieldName(obj)

	clusterSchema, err := l.dynamicSchemasLister.Get("", "cluster")
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		resourceField := map[string]v32.Field{}
		resourceField[fieldName] = v32.Field{
			Create:   true,
			Nullable: true,
			Update:   true,
			Type:     embeddedType,
		}

		dynamicSchema := &v3.DynamicSchema{}
		dynamicSchema.Name = "cluster"
		dynamicSchema.Spec.ResourceFields = resourceField
		dynamicSchema.Spec.Embed = true
		dynamicSchema.Spec.EmbedType = "cluster"
		_, err := l.dynamicSchemas.Create(dynamicSchema)
		if err != nil {
			return err
		}
		return nil
	}

	clusterSchema = clusterSchema.DeepCopy()

	shouldUpdate := false

	if clusterSchema.Spec.ResourceFields == nil {
		clusterSchema.Spec.ResourceFields = map[string]v32.Field{}
	}
	if _, ok := clusterSchema.Spec.ResourceFields[fieldName]; !ok {
		// if embedded we add the type to schema
		clusterSchema.Spec.ResourceFields[fieldName] = v32.Field{
			Create:   true,
			Nullable: true,
			Update:   true,
			Type:     embeddedType,
		}
		shouldUpdate = true
	}

	if shouldUpdate {
		_, err = l.dynamicSchemas.Update(clusterSchema)
		if err != nil {
			return err
		}
	}

	return nil
}

func toResourceField(name string, flag *types.Flag) (string, v32.Field, error) {
	field := v32.Field{
		Create: true,
		Update: true,
		Type:   "string",
	}

	name, err := toLowerCamelCase(name)
	if err != nil {
		return name, field, err
	}

	field.Description = flag.Usage

	if flag.Type == types.StringType {
		field.Default.StringValue = flag.Value

		if flag.Password {
			field.Type = "password"
		} else {
			field.Type = "string"
		}

		if flag.Default != nil {
			field.Default.StringValue = flag.Default.DefaultString
		}
	} else if flag.Type == types.IntType || flag.Type == types.IntPointerType {
		field.Type = "int"

		if flag.Default != nil {
			field.Default.IntValue = int(flag.Default.DefaultInt)
		}
	} else if flag.Type == types.BoolType || flag.Type == types.BoolPointerType {
		field.Type = "boolean"

		if flag.Default != nil {
			field.Default.BoolValue = flag.Default.DefaultBool
		}
	} else if flag.Type == types.StringSliceType {
		field.Type = "array[string]"

		if flag.Default != nil {
			field.Default.StringSliceValue = flag.Default.DefaultStringSlice.Value
		}
	} else {
		return name, field, fmt.Errorf("unknown type of flag %v: %v", flag, reflect.TypeOf(flag))
	}

	return name, field, nil
}

func toLowerCamelCase(nodeFlagName string) (string, error) {
	flagNameParts := strings.Split(nodeFlagName, "-")
	flagName := flagNameParts[0]
	for _, flagNamePart := range flagNameParts[1:] {
		flagName = flagName + strings.ToUpper(flagNamePart[:1]) + flagNamePart[1:]
	}
	return flagName, nil
}

func (l *Lifecycle) Updated(obj *v3.KontainerDriver) (runtime.Object, error) {
	logrus.Infof("update kontainerdriver %v", obj.Name)
	if hasStaticSchema(obj) {
		return obj, nil
	}

	if obj.Spec.BuiltIn && v32.KontainerDriverConditionActive.IsTrue(obj) {
		// Builtin drivers can still have their schema change during Rancher upgrades so we need to try
		return obj, l.createOrUpdateDynamicSchema(obj)
	}

	var err error
	var tmpObj runtime.Object
	switch { // dealing with deactivate action
	case !obj.Spec.Active && l.DynamicSchemaExists(obj):
		v32.KontainerDriverConditionInactive.Unknown(obj)
		// delete the active condition
		var i int
		for _, con := range obj.Status.Conditions {
			if con.Type != string(v32.KontainerDriverConditionActive) {
				obj.Status.Conditions[i] = con
				i++
			}
		}
		obj.Status.Conditions = obj.Status.Conditions[:i]
		// we don't need to show the Inactivating state so we don't need to store the Inactivate condition
		fallthrough
	case !obj.Spec.Active:
		tmpObj, err = v32.KontainerDriverConditionInactive.DoUntilTrue(obj, func() (runtime.Object, error) {
			if err = l.dynamicSchemas.Delete(getDynamicTypeName(obj), &v13.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, fmt.Errorf("error deleting schema: %v", err)
			}
			if err = l.removeFieldFromCluster(obj); err != nil {
				return nil, err
			}

			return obj, nil
		})
		obj = tmpObj.(*v3.KontainerDriver)

	// redownload file if active AND url changed or not downloaded
	// prevents downloading on air-gapped installations
	// and update the Downloaded condition to Unknown to show the downloading state
	case !v32.KontainerDriverConditionDownloaded.IsUnknown(obj) &&
		(obj.Spec.URL != obj.Status.ActualURL ||
			v32.KontainerDriverConditionDownloaded.IsFalse(obj) ||
			!l.driverExists(obj)):
		v32.KontainerDriverConditionDownloaded.Unknown(obj)

	// if it is a buildin driver, set true without downloading
	case obj.Spec.BuiltIn && !v32.KontainerDriverConditionDownloaded.IsTrue(obj):
		v32.KontainerDriverConditionDownloaded.True(obj)
		v32.KontainerDriverConditionInstalled.True(obj)

	// handling download process
	case !obj.Spec.BuiltIn && !v32.KontainerDriverConditionDownloaded.IsTrue(obj):
		obj, err = l.download(obj)
		if err != nil {
			return obj, err
		}

		// Force create/update of schemas
		v32.KontainerDriverConditionActive.Unknown(obj)

	// set active status to unknown to show the activating state
	case !l.DynamicSchemaExists(obj) && !v32.KontainerDriverConditionActive.IsUnknown(obj):
		v32.KontainerDriverConditionActive.Unknown(obj)

	// create schema and set active status to true
	default:
		tmpObj, err = v32.KontainerDriverConditionActive.DoUntilTrue(obj, func() (runtime.Object, error) {
			return obj, l.createOrUpdateDynamicSchema(obj)
		})
		obj = tmpObj.(*v3.KontainerDriver)
	}

	return obj, err
}

func hasStaticSchema(obj *v3.KontainerDriver) bool {
	return obj.Name == service.RancherKubernetesEngineDriverName || obj.Name == service.ImportDriverName
}

func getDynamicTypeName(obj *v3.KontainerDriver) string {
	var s string
	if obj.Spec.BuiltIn {
		s = obj.Status.DisplayName + "Config"
	} else {
		s = obj.Status.DisplayName + "EngineConfig"
	}

	return s
}

func getDynamicFieldName(obj *v3.KontainerDriver) string {
	if obj.Spec.BuiltIn {
		return obj.Status.DisplayName + "Config"
	}

	return obj.Status.DisplayName + "EngineConfig"
}

// Remove the Kontainer Cluster driver, see also the Schema.Store for kontainer driver
func (l *Lifecycle) Remove(obj *v3.KontainerDriver) (runtime.Object, error) {
	logrus.Infof("remove kontainerdriver %v", obj.Name)

	driver := drivers.NewKontainerDriver(obj.Spec.BuiltIn, obj.Name, obj.Spec.URL, obj.Spec.Checksum)
	err := driver.Remove()
	if err != nil {
		return nil, err
	}

	if hasStaticSchema(obj) {
		return obj, nil
	}

	if err := l.dynamicSchemas.Delete(getDynamicTypeName(obj), &v13.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("error deleting dynamic schema: %v", err)
	}

	if err := l.removeFieldFromCluster(obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// Removes the v3/dynamicschema "cluster"'s resourceField associated with the KontainerDriver obj
func (l *Lifecycle) removeFieldFromCluster(obj *v3.KontainerDriver) error {
	nodedriver.SchemaLock.Lock()
	defer nodedriver.SchemaLock.Unlock()

	fieldName := getDynamicFieldName(obj)

	nodeSchema, err := l.dynamicSchemasLister.Get("", "cluster")
	if err != nil {
		return fmt.Errorf("error getting schema: %v", err) // this error may fire during Rancher startup
	}

	nodeSchema = nodeSchema.DeepCopy()

	delete(nodeSchema.Spec.ResourceFields, fieldName)

	if _, err = l.dynamicSchemas.Update(nodeSchema); err != nil {
		return fmt.Errorf("error removing schema from cluster: %v", err)
	}

	return nil
}

func (l *Lifecycle) DynamicSchemaExists(obj *v3.KontainerDriver) bool {
	if obj == nil {
		return false
	}
	if obj.Spec.BuiltIn {
		return true
	}

	_, err := l.dynamicSchemasLister.Get("", strings.ToLower(getDynamicTypeName(obj)))
	return err == nil
}
