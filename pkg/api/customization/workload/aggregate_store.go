package workload

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

const (
	SelectorLabel = "workload.user.cattle.io/workloadselector"
)

type AggregateStore struct {
	Stores          map[string]types.Store
	Schemas         map[string]*types.Schema
	FieldToSchemaID map[string]string
}

func NewAggregateStore(schemas ...*types.Schema) *AggregateStore {
	a := &AggregateStore{
		Stores:          map[string]types.Store{},
		Schemas:         map[string]*types.Schema{},
		FieldToSchemaID: map[string]string{},
	}

	for _, schema := range schemas {
		a.Schemas[strings.ToLower(schema.ID)] = schema
		a.Stores[strings.ToLower(schema.ID)] = schema.Store
		fieldKey := fmt.Sprintf("%sConfig", schema.ID)
		a.FieldToSchemaID[fieldKey] = strings.ToLower(schema.ID)
	}

	return a
}

func (a *AggregateStore) Context() types.StorageContext {
	return config.UserStorageContext
}

func (a *AggregateStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, schemaType, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	_, shortID := splitTypeAndID(id)
	return store.ByID(apiContext, a.Schemas[schemaType], shortID)
}

func (a *AggregateStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	readerGroup, ctx := errgroup.WithContext(apiContext.Request.Context())
	apiContext.Request = apiContext.Request.WithContext(ctx)

	events := make(chan map[string]interface{})
	for _, schema := range a.Schemas {
		streamStore(readerGroup, apiContext, schema, opt, events)
	}

	go func() {
		readerGroup.Wait()
		close(events)
	}()
	return events, nil
}

func (a *AggregateStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	items := make(chan map[string]interface{})
	g, ctx := errgroup.WithContext(apiContext.Request.Context())
	submit := func(schema *types.Schema, store types.Store) {
		g.Go(func() error {
			data, err := store.List(apiContext, schema, opt)
			if err != nil {
				return err
			}
			for _, item := range data {
				select {
				case items <- item:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}

	for typeName, store := range a.Stores {
		submit(a.Schemas[typeName], store)
	}

	go func() {
		g.Wait()
		close(items)
	}()

	var result []map[string]interface{}
	for item := range items {
		result = append(result, item)
	}

	return result, g.Wait()
}

func (a *AggregateStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	// deployment is default if otherwise is not specified
	kind := client.DeploymentType
	toSchema := a.Schemas[kind]
	toStore := a.Stores[kind]
	for field, schemaID := range a.FieldToSchemaID {
		if val, ok := data[field]; ok && val != nil {
			toSchema = a.Schemas[schemaID]
			toStore = a.Stores[schemaID]
			break
		}
	}

	setSelector(toSchema.ID, data)
	setWorkloadSpecificDefaults(toSchema.ID, data)
	setSecrets(apiContext, data)

	return toStore.Create(apiContext, toSchema, data)
}

func setSelector(schemaID string, data map[string]interface{}) {
	setSelector := false
	isJob := strings.EqualFold(schemaID, "job") || strings.EqualFold(schemaID, "cronJob")
	if convert.IsEmpty(data["selector"]) && !isJob {
		setSelector = true
	}
	if setSelector {
		workloadID := resolveWorkloadID(schemaID, data)
		// set selector
		data["selector"] = map[string]interface{}{
			"matchLabels": map[string]interface{}{
				SelectorLabel: workloadID,
			},
		}

		// set workload labels
		workloadLabels := convert.ToMapInterface(data["workloadLabels"])
		if workloadLabels == nil {
			workloadLabels = make(map[string]interface{})
		}
		workloadLabels[SelectorLabel] = workloadID
		data["workloadLabels"] = workloadLabels

		// set labels
		labels := convert.ToMapInterface(data["labels"])
		if labels == nil {
			labels = make(map[string]interface{})
		}
		labels[SelectorLabel] = workloadID
		data["labels"] = labels
	}
}

func setWorkloadSpecificDefaults(schemaID string, data map[string]interface{}) {
	if strings.EqualFold(schemaID, "job") || strings.EqualFold(schemaID, "cronJob") {
		// job has different defaults
		if _, ok := data["restartPolicy"]; !ok {
			logrus.Info("Setting restart policy")
			data["restartPolicy"] = "OnFailure"
		}
	}
}

func setSecrets(apiContext *types.APIContext, data map[string]interface{}) {
	if _, ok := values.GetValue(data, "imagePullSecrets"); ok {
		return
	}
	var imagePullSecrets []corev1.LocalObjectReference

	if containers, ok := values.GetSlice(data, "containers"); ok {
		imageToSecret := make(map[string]*corev1.LocalObjectReference)
		imagePullSecrets, _ = data["imagePullSecrets"].([]corev1.LocalObjectReference)

		for _, container := range containers {
			if image := convert.ToString(container["image"]); image != "" {
				if secretRef, ok := imageToSecret[image]; ok {
					imagePullSecrets = append(imagePullSecrets, *secretRef)
					continue
				}
				if name := getRepo(image); name != "" {
					if gotSecret(data["projectId"], data["namespaceId"], apiContext, name) {
						secretRef := &corev1.LocalObjectReference{}
						secretRef.Name = name
						imagePullSecrets = append(imagePullSecrets, *secretRef)
					}
				}
			}
		}
	}
	if imagePullSecrets != nil {
		values.PutValue(data, imagePullSecrets, "imagePullSecrets")
	}
}

func resolveWorkloadID(schemaID string, data map[string]interface{}) string {
	return fmt.Sprintf("%s-%s-%s", schemaID, data["namespaceId"], data["name"])
}

func (a *AggregateStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	store, schemaType, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	_, shortID := splitTypeAndID(id)
	return store.Update(apiContext, a.Schemas[schemaType], data, shortID)
}

func (a *AggregateStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	store, schemaType, err := a.getStore(id)
	if err != nil {
		return nil, err
	}
	_, shortID := splitTypeAndID(id)
	return store.Delete(apiContext, a.Schemas[schemaType], shortID)
}

func (a *AggregateStore) getStore(id string) (types.Store, string, error) {
	typeName, _ := splitTypeAndID(id)
	store, ok := a.Stores[typeName]
	if !ok {
		return nil, "", httperror.NewAPIError(httperror.NotFound, "failed to find type "+typeName)
	}
	return store, typeName, nil
}

func streamStore(eg *errgroup.Group, apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions, result chan map[string]interface{}) {
	eg.Go(func() error {
		events, err := schema.Store.Watch(apiContext, schema, opt)
		if err != nil || events == nil {
			if err != nil {
				logrus.Errorf("failed on subscribe %s: %v", schema.ID, err)
			}
			return err
		}

		logrus.Debugf("watching %s", schema.ID)

		for e := range events {
			result <- e
		}

		return errors.New("disconnect")
	})
}

func splitTypeAndID(id string) (string, string) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) < 2 {
		// Must conform
		return "", ""
	}
	return parts[0], parts[1]
}

func gotSecret(projectID interface{}, namespaceID interface{}, apiContext *types.APIContext, repoName string) bool {
	if project := convert.ToString(projectID); project != "" {
		splitID := strings.Split(project, ":")
		if len(splitID) == 2 {
			projectName := splitID[1]
			if ok := foundSecret(projectName, repoName, projectclient.SecretType, apiContext); ok {
				return true
			}
		}
	}
	if namespace := convert.ToString(namespaceID); namespace != "" {
		if ok := foundSecret(namespace, repoName, "namespacedSecret", apiContext); ok {
			return true
		}
	}
	logrus.Debugf("couldn't find secret [%s]", repoName)
	return false
}

func foundSecret(prefix string, repoName string, datatype string, apiContext *types.APIContext) bool {
	secretName := fmt.Sprintf("%s:%s", prefix, repoName)
	var secret interface{}
	if err := access.ByID(apiContext, &projectschema.Version, datatype, secretName, &secret); err == nil {
		if secretMap := convert.ToMapInterface(secret); secretMap != nil {
			if val, _ := secretMap["type"]; convert.ToString(val) == "dockerCredential" {
				return true
			}
		}
	}
	return false
}

func getRepo(image string) string {
	var repo string
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		logrus.Debug(err)
		return repo
	}
	return reference.Domain(named)
}
