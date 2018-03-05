package workload

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	managementv3 "github.com/rancher/types/client/management/v3"
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
	setPorts(data)
	setScheduling(apiContext, data)

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
			data["restartPolicy"] = "OnFailure"
		}
	}
}

func store(registries map[string]projectclient.RegistryCredential, domainToCreds map[string][]corev1.LocalObjectReference, name string) {
	for registry := range registries {
		secretRef := corev1.LocalObjectReference{Name: name}
		if _, ok := domainToCreds[registry]; ok {
			domainToCreds[registry] = append(domainToCreds[registry], secretRef)
		} else {
			domainToCreds[registry] = []corev1.LocalObjectReference{secretRef}
		}
	}
}

func setPorts(data map[string]interface{}) {
	containers, ok := values.GetValue(data, "containers")
	if !ok {
		return
	}

	for _, c := range convert.ToInterfaceSlice(containers) {
		cMap, err := convert.EncodeToMap(c)
		if err != nil {
			logrus.Warnf("Failed to transform container to map: %v", err)
			continue
		}
		v, ok := values.GetValue(cMap, "ports")

		if ok {
			ports := convert.ToInterfaceSlice(v)
			for _, p := range ports {
				port, err := convert.EncodeToMap(p)
				if err != nil {
					logrus.Warnf("Failed to transform port to map %v", err)
					continue
				}
				if convert.IsEmpty(port["name"]) {
					containerPort, err := convert.ToNumber(port["containerPort"])
					if err != nil {
						logrus.Warnf("Failed to transform container port [%v] to number: %v", port["containerPort"], err)
					}
					port["name"] = fmt.Sprintf("%s%s", strings.ToLower(convert.ToString(port["protocol"])), strconv.Itoa(int(containerPort)))
				}
			}
		}
	}
}

func getCreds(apiContext *types.APIContext, namespaceID string) map[string][]corev1.LocalObjectReference {
	domainToCreds := make(map[string][]corev1.LocalObjectReference)
	var namespacedCreds []projectclient.NamespacedDockerCredential
	if err := access.List(apiContext, &projectschema.Version, "namespacedDockerCredential", &types.QueryOptions{}, &namespacedCreds); err == nil {
		for _, cred := range namespacedCreds {
			if cred.NamespaceId == namespaceID {
				store(cred.Registries, domainToCreds, cred.Name)
			}
		}
	}
	var creds []projectclient.DockerCredential
	if err := access.List(apiContext, &projectschema.Version, "dockerCredential", &types.QueryOptions{}, &creds); err == nil {
		for _, cred := range creds {
			store(cred.Registries, domainToCreds, cred.Name)
		}
	}
	return domainToCreds
}

func setSecrets(apiContext *types.APIContext, data map[string]interface{}) {
	if val, _ := values.GetValue(data, "imagePullSecrets"); val != nil {
		return
	}
	if containers, _ := values.GetSlice(data, "containers"); len(containers) > 0 {
		imagePullSecrets, _ := data["imagePullSecrets"].([]corev1.LocalObjectReference)
		domainToCreds := getCreds(apiContext, convert.ToString(data["namespaceId"]))
		for _, container := range containers {
			if image := convert.ToString(container["image"]); image != "" {
				domain := getDomain(image)
				if secrets, ok := domainToCreds[domain]; ok {
					imagePullSecrets = append(imagePullSecrets, secrets...)
				}
			}
		}
		if imagePullSecrets != nil {
			values.PutValue(data, imagePullSecrets, "imagePullSecrets")
		}
	}
}

func setScheduling(apiContext *types.APIContext, data map[string]interface{}) {
	if nodeID := convert.ToString(values.GetValueN(data, "scheduling", "node", "nodeId")); nodeID != "" {
		nodeName := getNodeName(apiContext, nodeID)
		values.PutValue(data, nodeName, "scheduling", "node", "nodeId")
		state := getState(data)
		state[getKey(nodeName)] = nodeID
		setState(data, state)
	}
}

func getNodeName(apiContext *types.APIContext, nodeID string) string {
	var node managementv3.Node
	var nodeName string
	if err := access.ByID(apiContext, &managementschema.Version, managementv3.NodeType, nodeID, &node); err == nil {
		nodeName = node.NodeName
	}
	return nodeName
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
	setPorts(data)
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

func getDomain(image string) string {
	var repo string
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		logrus.Debug(err)
		return repo
	}
	domain := reference.Domain(named)
	if domain == "docker.io" {
		return "index.docker.io"
	}
	return domain
}

func setState(data map[string]interface{}, stateMap map[string]string) {
	content, err := json.Marshal(stateMap)
	if err != nil {
		logrus.Errorf("failed to save state on workload: %v", data["id"])
		return
	}

	values.PutValue(data, string(content), "annotations", "workload.cattle.io/state")
}

func getState(data map[string]interface{}) map[string]string {
	state := map[string]string{}

	v, ok := values.GetValue(data, "annotations", "workload.cattle.io/state")
	if ok {
		json.Unmarshal([]byte(convert.ToString(v)), &state)
	}

	return state
}

func getKey(key string) string {
	return base64.URLEncoding.EncodeToString([]byte(key))
}
