package storageclass

import (
	"testing"

	"github.com/rancher/norman/types"
	"k8s.io/utils/pointer"

	"github.com/stretchr/testify/assert"
)

type dummyStore struct{}

func createWithFields(store types.Store, prov string, kind *string, storageaccounttype *string) (map[string]interface{}, error) {
	testData := map[string]interface{}{
		"provisioner": prov,
	}

	if kind != nil {
		testData["parameters"] = map[string]interface{}{"kind": *kind}

		if storageaccounttype != nil {
			testData["parameters"].(map[string]interface{})["storageaccounttype"] = *storageaccounttype
		}
	}

	return store.Create(nil, nil, testData)
}

func TestCreate(t *testing.T) {
	assert := assert.New(t)
	testStore := Wrap(dummyStore{})

	// create with a storageaccounttype of nil should give the storage class a default value for the field
	data, err := createWithFields(testStore, AzureDisk, pointer.StringPtr("shared"), nil)
	assert.Nil(err)
	assert.Equal("Standard_LRS", data["parameters"].(map[string]interface{})["storageaccounttype"])

	// create with a kind that is not shared should not make a change to the storageaccounttype
	data, err = createWithFields(testStore, AzureDisk, pointer.StringPtr("dedicated"), nil)
	assert.Nil(err)
	assert.Equal(nil, data["parameters"].(map[string]interface{})["storageaccounttype"])

	// create with a storageaccounttype of empty string should give the storage class a default value for the field
	data, err = createWithFields(testStore, AzureDisk, pointer.StringPtr("shared"), pointer.StringPtr(""))
	assert.Nil(err)
	assert.Equal("Standard_LRS", data["parameters"].(map[string]interface{})["storageaccounttype"])

	// create with a storageaccounttype of any non-empty string should should not change the storageaccounttype
	data, err = createWithFields(testStore, AzureDisk, pointer.StringPtr("shared"), pointer.StringPtr("premium"))
	assert.Nil(err)
	assert.Equal("premium", data["parameters"].(map[string]interface{})["storageaccounttype"])

	// create with nil params should create with default value for storageaccounttype
	data, err = createWithFields(testStore, AzureDisk, nil, nil)
	assert.Nil(err)
	assert.Equal("Standard_LRS", data["parameters"].(map[string]interface{})["storageaccounttype"])

	// creaate with empty strings should create with default value for storageaccounttype
	data, err = createWithFields(testStore, AzureDisk, pointer.StringPtr(""), pointer.StringPtr(""))
	assert.Nil(err)
	assert.Equal("Standard_LRS", data["parameters"].(map[string]interface{})["storageaccounttype"])
}

func (ds dummyStore) Context() types.StorageContext {
	return types.StorageContext("test")
}
func (ds dummyStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (ds dummyStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	return nil, nil
}

func (ds dummyStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}

func (ds dummyStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	return nil, nil
}

func (ds dummyStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}

func (ds dummyStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}
