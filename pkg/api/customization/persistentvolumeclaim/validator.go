package persistentvolumeclaim

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/store/storageclass"
	"github.com/rancher/rancher/pkg/clustermanager"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Validator struct {
	ClusterManager *clustermanager.Manager
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	clusterName := v.ClusterManager.ClusterName(request)
	c, err := v.ClusterManager.UserContext(clusterName)
	if err != nil {
		return err
	}

	storageClassID, _ := data["storageClassId"].(string)
	storageClass, err := c.Storage.StorageClasses("").Get(storageClassID, v1.GetOptions{})
	if err != nil {
		return err
	}

	// if the referenced storage class does not have a storageaccounttype, storage account creation will fail in k8s
	if storageClass.Provisioner == storageclass.AzureDisk {
		if storageClass.Parameters[storageclass.StorageAccountType] == "" && storageClass.Parameters[storageclass.SkuName] == "" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("invalid storage class [%s]: must provide "+
				"storageaccounttype or skuName", storageClass.Name))
		}
	}

	return nil
}
