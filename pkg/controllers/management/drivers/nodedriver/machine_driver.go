package nodedriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	batchcontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/batch/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

var (
	SchemaLock   = sync.Mutex{}
	driverLock   = sync.Mutex{}
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
		k8sClient:        management.K8sClient,
		jobClient:        management.Wrangler.Batch.Job(),
		jobCache:         management.Wrangler.Batch.Job().Cache(),
		podCache:         management.Wrangler.Core.Pod().Cache(),
		schemas:          management.Schemas,

		dockerMachineVersion: settings.MachineVersion.Get(),
	}

	nodeDriverClient.AddLifecycle(ctx, "node-driver-controller", nodeDriverLifecycle)

	// Watch for Job changes to re-enqueue NodeDrivers when get-flags Jobs complete or fail
	relatedresource.Watch(ctx, "node-driver-getcreateflags", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if obj == nil {
			return nil, nil
		}
		job, ok := obj.(*batchv1.Job)
		if !ok || !isJobFinished(job) || job.DeletionTimestamp != nil {
			return nil, nil
		}
		if driverName := job.Labels[JobLabelDriver]; driverName != "" {
			return []relatedresource.Key{{Name: driverName}}, nil
		}
		return nil, nil
	}, management.Management.NodeDrivers("").Controller(), management.Wrangler.Batch.Job())
}

type Lifecycle struct {
	nodeDriverClient     v3.NodeDriverInterface
	schemaClient         v3.DynamicSchemaInterface
	schemaLister         v3.DynamicSchemaLister
	secretStore          v1.SecretInterface
	nsStore              v1.NamespaceInterface
	k8sClient            kubernetes.Interface
	jobClient            batchcontrollers.JobClient
	jobCache             batchcontrollers.JobCache
	podCache             corecontrollers.PodCache
	schemas              *types.Schemas
	dockerMachineVersion string
}

func (m *Lifecycle) Create(obj *v32.NodeDriver) (runtime.Object, error) {
	logrus.Debugf("Handling creation of node driver %s", obj.Name)
	return m.download(obj)
}

func (m *Lifecycle) download(obj *v32.NodeDriver) (*v32.NodeDriver, error) {
	if !obj.Spec.Active && !obj.Spec.AddCloudCredential {
		driverName := obj.Spec.DisplayName
		if driverName == "" {
			driverName = obj.Name
		}
		if driverName != "" {
			jobName := getCreateFlagsJobName(driverName)
			if err := m.deleteJob(jobName); err != nil && !apierrors.IsNotFound(err) {
				logrus.Warnf("failed to delete job %s for inactive driver %s: %v", jobName, obj.Name, err)
			}
		}
		return obj, nil
	}
	driverLock.Lock()
	defer driverLock.Unlock()
	logrus.Debugf("Downloading node driver %s", obj.Name)

	// if node driver was created, we also activate the driver by default
	driver, err := drivers.NewDynamicDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	if err != nil {
		// Configuration is incorrect, no point on retrying until it's changed
		v32.NodeDriverConditionDownloaded.False(obj)
		v32.NodeDriverConditionDownloaded.Reason(obj, err.Error())
		return obj, controller.ErrIgnore
	} else if !driver.Valid() {
		v32.NodeDriverConditionDownloaded.Unknown(obj)
		v32.NodeDriverConditionInstalled.Unknown(obj)
		return obj, fmt.Errorf("node driver binary is not ready")
	}

	driverName := strings.TrimPrefix(driver.Name(), drivers.DockerMachineDriverPrefix)
	obj.Spec.DisplayName = driverName
	v32.NodeDriverConditionDownloaded.True(obj)
	v32.NodeDriverConditionDownloaded.Reason(obj, "")
	v32.NodeDriverConditionDownloaded.Message(obj, "")
	v32.NodeDriverConditionInstalled.True(obj)
	v32.NodeDriverConditionInstalled.Reason(obj, "")
	v32.NodeDriverConditionInstalled.Message(obj, "")

	needsUpdate := m.checkDriverVersion(obj)

	schemaName := driverName + "config"
	existingSchema, err := m.schemaLister.Get("", schemaName)
	if err != nil {
		logrus.Errorf("failed to get schema %q: %v", schemaName, err)
		needsUpdate = true
	}

	// If dynamic schema exists already, and this specific version of the driver was already evaluated, only update the schema
	if !needsUpdate {
		// add credential schema
		credFields := map[string]v32.Field{}
		pubCredFields, privateCredFields, passwordFields, defaults, optionals := getCredFields(obj.Annotations)
		for name, field := range existingSchema.Spec.ResourceFields {
			if SSHKeyFields[name] || passwordFields[name] || privateCredFields[name] {
				if field.Type != "password" {
					needsUpdate = true
					break
				}
			}
			// even if forceUpdate is false, calculate credFields to check if credSchema needs to be updated
			if privateCredFields[name] || pubCredFields[name] {
				credField := field
				credField.Required = !optionals[name]
				if val, ok := defaults[name]; ok {
					credField = updateDefault(credField, val, field.Type)
				}
				credFields[name] = credField
			}
		}
		if !needsUpdate {
			return m.createCredSchema(obj, credFields)
		}
	}

	m.addVersionInfo(obj)
	if err := m.addUIHintsAnno(obj); err != nil {
		return nil, fmt.Errorf("failed JSON in addUIHintsAnno: %w", err)
	}

	// Resolve create flags asynchronously via Job
	flags, ready, err := m.resolveCreateFlags(obj, driverName)
	if err != nil {
		if errors.Is(err, controller.ErrIgnore) {
			return obj, controller.ErrIgnore
		}
		return nil, err
	} else if !ready {
		m.nodeDriverClient.Controller().EnqueueAfter("", obj.Name, 10*time.Second)
		logrus.Debugf("discovery job for NodeDriver %q is still pending, re-queueing...", obj.Name)
		return nil, controller.ErrIgnore
	}

	credFields := map[string]v32.Field{}
	resourceFields := map[string]v32.Field{}
	pubCredFields, privateCredFields, passwordFields, defaults, optionals := getCredFields(obj.Annotations)
	for _, flag := range flags {
		name, err := fieldNameFromFlag(flag)
		if err != nil {
			return nil, err
		}
		field := flag.toV3Field()
		if fileToFieldAliases, exists := obj.Annotations[FileToFieldAliasesAnno]; exists && strings.Contains(fileToFieldAliases, name) {
			fields := ParseKeyValueString(fileToFieldAliases)
			for schemaField, driverField := range fields {
				if driverField == name {
					name = schemaField
					field.Description = fmt.Sprintf("File contents for %v", schemaField)
				}
			}
		}

		if privateCredFields[name] || passwordFields[name] || SSHKeyFields[name] {
			field.Type = "password"
		}

		if pubCredFields[name] || privateCredFields[name] {
			credField := field
			credField.Required = !optionals[name]
			if val, ok := defaults[name]; ok {
				credField = updateDefault(credField, val, field.Type)
			}
			credFields[name] = credField
		}

		resourceFields[name] = field
	}
	dynamicSchema := &v32.DynamicSchema{
		Spec: v32.DynamicSchemaSpec{
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
		if !apierrors.IsAlreadyExists(err) {
			return obj, err
		}
		existingSchema, err := m.schemaClient.Get(dynamicSchema.Name, metav1.GetOptions{})
		if err != nil {
			return obj, err
		}
		if !reflect.DeepEqual(existingSchema.Spec.ResourceFields, dynamicSchema.Spec.ResourceFields) {
			toUpdate := existingSchema.DeepCopy()
			toUpdate.Spec.ResourceFields = dynamicSchema.Spec.ResourceFields
			_, err := m.schemaClient.Update(toUpdate)
			if err != nil {
				return obj, err
			}
		}
	}

	return m.createCredSchema(obj, credFields)
}

func (m *Lifecycle) createCredSchema(obj *v32.NodeDriver, credFields map[string]v32.Field) (*v32.NodeDriver, error) {
	name := credentialConfigSchemaName(obj.Spec.DisplayName)
	credSchema, err := m.schemaLister.Get("", name)

	if name == "amazonec2credentialconfig" {
		credFields["defaultRegion"] = v32.Field{
			Type:         "string",
			Description:  "AWS Default Region",
			DynamicField: true,
			Create:       true,
			Update:       true,
		}
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			credentialSchema := &v32.DynamicSchema{
				Spec: v32.DynamicSchemaSpec{
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

// resolveCreateFlags retrieves driver create flags by ensuring a Job runs to completion.
// Returns (flags, ready, error). If ready=false, the caller should requeue.
func (m *Lifecycle) resolveCreateFlags(obj *v32.NodeDriver, driverName string) ([]driverFlag, bool, error) {
	var url, hash string
	if !obj.Spec.Builtin {
		var err error
		url, hash, err = drivers.GetDriverDownloadURL(obj)
		if err != nil {
			logrus.Warnf("failed to get local driver download URL for %s, falling back to spec URL: %v", obj.Name, err)
			url = obj.Spec.URL
			hash = obj.Spec.Checksum
		}
	}

	expectedHash := m.objectKey(obj)

	// Check if Job exists
	jobName := getCreateFlagsJobName(driverName)
	status, payload, err := m.getJobStatus(jobName, expectedHash)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get job status: %w", err)
	}

	switch status {
	case jobStatusStale:
		logrus.Infof("canceling stale discovery job %s for driver %s", jobName, driverName)
		if delErr := m.deleteJob(jobName); delErr != nil && !apierrors.IsNotFound(delErr) {
			return nil, false, fmt.Errorf("failed to delete stale job: %w", delErr)
		}
		return nil, false, nil

	case jobStatusNotFound:
		// Create the Job
		_, err := m.createGetFlagsJob(driverName, expectedHash, url, hash)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, false, fmt.Errorf("failed to create get-flags job: %w", err)
		}
		return nil, false, nil

	case jobStatusRunning:
		// Job is still running - requeue
		return nil, false, nil

	case jobStatusSucceeded:
		// Job succeeded - decode, delete job, return
		flags, err := decodeFields(payload)
		if err != nil {
			return nil, false, fmt.Errorf("failed to decode flags from job: %w", err)
		}

		// Delete the job
		if err := m.deleteJob(jobName); err != nil {
			logrus.Warnf("failed to delete job %s: %v", jobName, err)
		}

		return flags, true, nil

	case jobStatusFailed:
		failMsg := string(payload)
		if failMsg == "" {
			failMsg = "get-create-flags job failed"
		}
		v32.NodeDriverConditionInstalled.False(obj)
		v32.NodeDriverConditionInstalled.Message(obj, failMsg)
		v32.NodeDriverConditionDownloaded.False(obj)
		v32.NodeDriverConditionDownloaded.Message(obj, failMsg)
		return nil, false, controller.ErrIgnore

	default:
		return nil, false, fmt.Errorf("unknown job status: %s", status)
	}
}

func (m *Lifecycle) objectKey(obj *v32.NodeDriver) string {
	driverName := obj.Spec.DisplayName
	if obj.Spec.Builtin {
		// For builtin drivers, key by rancher-machine version
		return name.SafeConcatName(driverName, m.dockerMachineVersion)
	}
	// For non-builtin drivers, include both rancher-machine version and URL+checksum hash
	return name.SafeConcatName(driverName, m.dockerMachineVersion, name.Hex(obj.Spec.URL+obj.Spec.Checksum, 16))
}

func (m *Lifecycle) checkDriverVersion(obj *v32.NodeDriver) bool {
	if !v32.NodeDriverConditionDownloaded.IsTrue(obj) || !v32.NodeDriverConditionInstalled.IsTrue(obj) {
		return true
	}

	if _, ok := obj.Annotations[FileToFieldAliasesAnno]; ok {
		if val, ok := obj.Annotations[uiFieldHintsAnno]; !ok || val == "" {
			return true
		}
	}

	// Any driver version changed if rancher-machine version differs
	if obj.Status.AppliedDockerMachineVersion != m.dockerMachineVersion {
		return true
	}

	if !obj.Spec.Builtin {
		if obj.Spec.URL != obj.Status.AppliedURL || obj.Spec.Checksum != obj.Status.AppliedChecksum {
			return true
		}
	}

	return false
}

func (m *Lifecycle) addVersionInfo(obj *v32.NodeDriver) {
	obj.Status.AppliedDockerMachineVersion = m.dockerMachineVersion
	if !obj.Spec.Builtin {
		obj.Status.AppliedURL = obj.Spec.URL
		obj.Status.AppliedChecksum = obj.Spec.Checksum
	}
}

func (m *Lifecycle) addUIHintsAnno(obj *v32.NodeDriver) error {
	if aliasedField, ok := obj.Annotations[FileToFieldAliasesAnno]; ok {
		anno := make(map[string]map[string]string)
		aliases := ParseKeyValueString(aliasedField)
		for field := range aliases {
			anno[field] = map[string]string{
				"type": "multiline",
			}
		}

		jsonAnno, err := json.Marshal(anno)
		if err != nil {
			return err
		}

		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}

		obj.Annotations[uiFieldHintsAnno] = string(jsonAnno)
	}
	return nil
}

func (m *Lifecycle) Updated(obj *v32.NodeDriver) (runtime.Object, error) {
	var err error

	obj, err = m.download(obj)
	if err != nil {
		return obj, err
	}

	if err := m.createOrUpdateNodeForEmbeddedType(obj.Spec.DisplayName+"config", obj.Spec.DisplayName+"Config", obj.Spec.Active); err != nil {
		return obj, err
	}

	if err := m.createOrUpdateNodeForEmbeddedTypeCredential(credentialConfigSchemaName(obj.Spec.DisplayName),
		obj.Spec.DisplayName+"credentialConfig", obj.Spec.Active || obj.Spec.AddCloudCredential); err != nil {
		return obj, err
	}

	v32.NodeDriverConditionActive.True(obj)
	v32.NodeDriverConditionInactive.True(obj)

	return obj, nil
}

func (m *Lifecycle) Remove(obj *v32.NodeDriver) (runtime.Object, error) {
	driverName := obj.Spec.DisplayName
	if driverName == "" {
		driverName = obj.Name
	}
	if driverName != "" {
		jobName := getCreateFlagsJobName(driverName)
		if err := m.deleteJob(jobName); err != nil && !apierrors.IsNotFound(err) {
			logrus.Warnf("failed to delete job %s when removing driver %s: %v", jobName, obj.Name, err)
		}
	}

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
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if apierrors.IsNotFound(err) {
		resourceField := map[string]v32.Field{}
		if embedded {
			resourceField[fieldName] = v32.Field{
				Create:   true,
				Nullable: true,
				Update:   update,
				Type:     embeddedType,
			}
		}
		dynamicSchema := &v32.DynamicSchema{}
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

	nodeSchema = nodeSchema.DeepCopy()

	shouldUpdate := false
	if embedded {
		if nodeSchema.Spec.ResourceFields == nil {
			nodeSchema.Spec.ResourceFields = map[string]v32.Field{}
		}
		if _, ok := nodeSchema.Spec.ResourceFields[fieldName]; !ok {
			// if embedded we add the type to schema
			logrus.Infof("uploading %s to %s schema", fieldName, schemaID)
			nodeSchema.Spec.ResourceFields[fieldName] = v32.Field{
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

func getCredFields(annotations map[string]string) (map[string]bool, map[string]bool, map[string]bool, map[string]string, map[string]bool) {
	getMap := func(fields string) map[string]bool {
		data := map[string]bool{}
		for _, field := range strings.Split(fields, ",") {
			data[field] = true
		}
		return data
	}
	return getMap(annotations["publicCredentialFields"]),
		getMap(annotations["privateCredentialFields"]),
		getMap(annotations["passwordFields"]),
		ParseKeyValueString(annotations["defaults"]),
		getMap(annotations["optionalCredentialFields"])
}

func credentialConfigSchemaName(driverName string) string {
	return fmt.Sprintf("%s%s", driverName, "credentialconfig")
}

func updateDefault(credField v32.Field, val, kind string) v32.Field {
	switch kind {
	case "int":
		i, err := strconv.Atoi(val)
		if err == nil {
			credField.Default = v32.Values{IntValue: i}
		} else {
			logrus.Errorf("error converting %s to int %v", val, err)
		}
	case "boolean":
		credField.Default = v32.Values{BoolValue: convert.ToBool(val)}
	case "array[string]":
		credField.Default = v32.Values{StringSliceValue: convert.ToStringSlice(val)}
	case "password", "string":
		credField.Default = v32.Values{StringValue: val}
	default:
		logrus.Errorf("unsupported kind for default val:%s kind:%s", val, kind)
	}
	return credField
}
