package store

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v1 "github.com/rancher/types/apis/core/v1"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
	"github.com/rancher/types/namespace"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	separator = ".."
)

var (
	arrKeys      = map[string]string{"fluentServers": "endpoint"}
	ignoreFields = map[string]bool{"appliedSpec": true, "failedSpec": true}
)

/*
arrKeys = map{arrayFieldName: uniqueKeyName}
When password fields are not passed from api, uniqueKeyName is used to search matching entry in existing array from db
Make sure all nested array fields from pwdTypes are added and validate entries don't have identical uniqueKeyName
*/

type PasswordStore struct {
	Schemas     map[string]*types.Schema
	Fields      map[string]map[string]interface{}
	Stores      map[string]types.Store
	secretStore v1.SecretInterface
	nsStore     v1.NamespaceInterface
}

func SetPasswordStore(schemas *types.Schemas, secretStore v1.SecretInterface, nsStore v1.NamespaceInterface) {
	modifyProjectTypes := map[string]bool{
		"githubPipelineConfig": true,
		"gitlabPipelineConfig": true,
	}

	pwdStore := &PasswordStore{
		Schemas:     map[string]*types.Schema{},
		Fields:      map[string]map[string]interface{}{},
		Stores:      map[string]types.Store{},
		secretStore: secretStore,
		nsStore:     nsStore,
	}

	//add your parent schema name here
	pwdTypes := []string{
		"clusterlogging",
		"projectlogging",
		"globaldnsprovider",
	}

	for _, storeType := range pwdTypes {
		var schema *types.Schema
		if _, ok := modifyProjectTypes[storeType]; ok {
			schema = schemas.Schema(&projectschema.Version, storeType)
		} else {
			schema = schemas.Schema(&managementschema.Version, storeType)
		}
		if schema != nil {
			data := getFields(schema, schemas, map[string]bool{})
			id := schema.ID
			pwdStore.Stores[id] = schema.Store
			pwdStore.Fields[id] = data
			schema.Store = pwdStore
			ans, _ := json.Marshal(data)
			logrus.Debugf("password fields %s", string(ans))
		}
	}
}

func (p *PasswordStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := p.replacePasswords(p.Fields[schema.ID], data, nil); err != nil {
		return nil, err
	}
	return p.Stores[schema.ID].Create(apiContext, schema, data)
}

func (p *PasswordStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return p.Stores[schema.ID].ByID(apiContext, schema, id)
}

func (p *PasswordStore) Context() types.StorageContext {
	return config.ManagementStorageContext
}

func (p *PasswordStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := p.Stores[schema.ID].ByID(apiContext, schema, id)
	if err != nil {
		return data, err
	}
	p.deleteSecretData(data, schema.ID)
	return p.Stores[schema.ID].Delete(apiContext, schema, id)
}

func (p *PasswordStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	return p.Stores[schema.ID].List(apiContext, schema, opt)
}

func (p *PasswordStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	existing, err := p.Stores[schema.ID].ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	if err := p.replacePasswords(p.Fields[schema.ID], data, existing); err != nil {
		return nil, err
	}
	return p.Stores[schema.ID].Update(apiContext, schema, data, id)
}

func (p *PasswordStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return p.Stores[schema.ID].Watch(apiContext, schema, opt)
}

func (p *PasswordStore) replacePasswords(sepData, data, existing map[string]interface{}) error {
	/*
		sepData: path to all password fields built recursively from schema
		data: incoming data from api
		existing: data from db by id

		Looping through sepData,
			if separator == reached end, check to update/create or replace password fields
			else recursive call for map/array
	*/
	if len(data) == 0 {
		// nothing to put in data, delete existing secret for this path
		if err := p.deleteExistingSecrets(sepData, existing); err != nil {
			return err
		}
		return nil
	}
	for sepKey, sepVal := range sepData {
		if convert.ToString(sepVal) == separator {
			if val2, ok := data[sepKey]; ok {
				if err := p.putSecretData(data, existing, sepKey, convert.ToString(val2), false); err != nil {
					return err
				}
			} else if _, ok := existing[sepKey]; ok {
				if err := p.putSecretData(data, existing, sepKey, "", true); err != nil {
					return err
				}
			} else if data != nil {
				logrus.Debugf("[%v] not present in incoming data, secret not stored", sepKey)
			}
		} else if val2, ok := data[sepKey]; ok {
			valArr := convert.ToMapSlice(val2)
			existArr := convert.ToMapSlice(existing[sepKey])
			if valArr == nil {
				if err := p.replacePasswords(convert.ToMapInterface(sepVal), convert.ToMapInterface(data[sepKey]), convert.ToMapInterface(existing[sepKey])); err != nil {
					return err
				}
			}
			// build matching array entries from existing data {uniqueKeyName: index}
			existingDataMap := map[string]int{}
			searchKey := arrKeys[sepKey]
			if searchKey != "" {
				for i, each := range existArr {
					if name, ok := each[searchKey]; ok {
						if strName := convert.ToString(name); strName != "" {
							existingDataMap[convert.ToString(name)] = i
						}
					}
				}
			}
			for _, each := range valArr {
				exists := false
				if name, ok := each[searchKey]; ok {
					if strName := convert.ToString(name); strName != "" {
						if ind, ok := existingDataMap[strName]; ok {
							if err := p.replacePasswords(convert.ToMapInterface(sepVal), each, existArr[ind]); err != nil {
								return err
							}
							exists = true
							delete(existingDataMap, strName)
						}
					}
				}
				if !exists {
					if err := p.replacePasswords(convert.ToMapInterface(sepVal), each, nil); err != nil {
						return err
					}
				}
			}
			// delete remaining secrets from existingArr
			for _, ind := range existingDataMap {
				if err := p.replacePasswords(convert.ToMapInterface(sepVal), nil, existArr[ind]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *PasswordStore) deleteExistingSecrets(sepData, existing map[string]interface{}) error {
	/*
		Similar to function replacePasswords, except when data is nil,
		traverse through existing map to find secrets for deletion
	*/
	for sepKey, sepVal := range sepData {
		if convert.ToString(sepVal) == separator {
			if exisVal, ok := existing[sepKey]; ok {
				splitKey := strings.SplitN(convert.ToString(exisVal), ":", 2)
				if len(splitKey) == 2 {
					if err := p.deleteSecret(splitKey[1], splitKey[0]); err != nil {
						return err
					}
				}
			}
		} else {
			existArr := convert.ToMapSlice(existing[sepKey])
			if existArr == nil {
				if err := p.deleteExistingSecrets(convert.ToMapInterface(sepData[sepKey]), convert.ToMapInterface(existing[sepKey])); err != nil {
					return err
				}
				continue
			}
			for _, each := range existArr {
				if err := p.deleteExistingSecrets(convert.ToMapInterface(sepData[sepKey]), each); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *PasswordStore) putSecretData(data, existing map[string]interface{}, key, val string, replace bool) error {
	var (
		name string
		err  error
	)
	if existing != nil {
		if exisVal, ok := existing[key]; ok {
			splitKey := strings.SplitN(convert.ToString(exisVal), ":", 2)
			if len(splitKey) == 2 && splitKey[0] == namespace.GlobalNamespace {
				name = splitKey[1]
			}
		}
	}
	if replace {
		if name == "" {
			return fmt.Errorf("replacePasswords: secret name not available in existing data")
		}
		data[key] = fmt.Sprintf("%s:%s", namespace.GlobalNamespace, name)
		return nil
	}
	if name == "" {
		name, err = randomtoken.Generate()
		if err != nil {
			return fmt.Errorf("replacePasswords: error generating random name %v", err)
		}
	}
	if err := p.createOrUpdateSecrets(val, name, namespace.GlobalNamespace); err != nil {
		return fmt.Errorf("replacePasswords: createOrUpdate secrets %v", err)
	}
	data[key] = fmt.Sprintf("%s:%s", namespace.GlobalNamespace, name)
	return nil
}

func (p *PasswordStore) buildSecretNames(data1 map[string]interface{}, data2 map[string]interface{}, secrets map[string]bool) {
	for key1, val1 := range data1 {
		if val2, ok := data2[key1]; ok {
			if convert.ToString(val1) == separator {
				val := convert.ToString(val2)
				if val != "" {
					secrets[val] = true
				}
			} else {
				valArr := convert.ToMapSlice(val2)
				if valArr == nil {
					p.buildSecretNames(convert.ToMapInterface(val1), convert.ToMapInterface(val2), secrets)
				} else {
					for _, each := range valArr {
						p.buildSecretNames(convert.ToMapInterface(val1), each, secrets)
					}
				}
			}
		}
	}
}

func (p *PasswordStore) createOrUpdateSecrets(data, name, namespace string) error {
	_, err := p.nsStore.Get(namespace, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		if _, err := p.nsStore.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	if err != nil {
		return err
	}
	name = strings.ToLower(name)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{name: data},
		Type:       corev1.SecretTypeOpaque,
	}
	existing, err := p.secretStore.GetNamespaced(namespace, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = p.secretStore.Create(secret)
		return err
	} else if err != nil {
		return err
	}
	if !reflect.DeepEqual(existing.StringData, secret.StringData) {
		existing.StringData = secret.StringData
		_, err = p.secretStore.Update(existing)
	}
	return err
}

func (p *PasswordStore) deleteSecretData(data map[string]interface{}, id string) error {
	secrets := map[string]bool{}
	p.buildSecretNames(p.Fields[id], data, secrets)
	logrus.Infof("deleteSecretData: %v", secrets)
	for secret := range secrets {
		split := strings.SplitN(secret, ":", 2)
		if len(split) != 2 || split[0] != namespace.GlobalNamespace {
			continue
		}
		if err := p.deleteSecret(split[1], split[0]); err != nil {
			return err
		}
	}
	return nil
}

func (p *PasswordStore) getSecret(input []string) (string, error) {
	returned := ""
	secret, err := p.secretStore.GetNamespaced(input[0], input[1], metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for key, val := range secret.Data {
		if key == input[1] {
			returned = string(val)
		}
	}
	return returned, nil
}

func (p *PasswordStore) deleteSecret(name string, namespace string) error {
	err := p.secretStore.DeleteNamespaced(namespace, name, &metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	return err
}

func getFields(schema *types.Schema, schemas *types.Schemas, arrFields map[string]bool) map[string]interface{} {
	data := map[string]interface{}{}
	for name, field := range schema.ResourceFields {
		fieldType := field.Type
		if strings.HasPrefix(fieldType, "array") {
			fieldType = strings.Split(fieldType, "[")[1]
			fieldType = fieldType[:len(fieldType)-1]
			arrFields[fieldType] = true
		}
		checkSchema := schemas.Schema(&managementschema.Version, fieldType)
		if checkSchema != nil {
			if _, ok := ignoreFields[name]; ok {
				continue
			}
			value := getFields(checkSchema, schemas, arrFields)
			if len(value) > 0 {
				data[name] = value
				if _, ok := arrFields[name]; ok {
					if _, ok := arrKeys[fmt.Sprintf("%ss", name)]; !ok {
						logrus.Warnf("passwordFields: assigned searchKey not present for array[%v], editing might not work as expected for passwords", name)
					}
				}
			}
		} else {
			if field.Type == "password" {
				data[name] = separator
			}
		}
	}
	return data
}
