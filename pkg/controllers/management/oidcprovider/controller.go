package oidcprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/oidc/randomstring"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

const (
	createClientSecretAnn     = "cattle.io/oidc-client-secret-create"
	removeClientSecretAnn     = "cattle.io/oidc-client-secret-remove"
	regenerateClientSecretAnn = "cattle.io/oidc-client-secret-regenerate"
	secretKeyPrefix           = "client-secret-"
	secretNamespace           = "cattle-oidc-client-secrets"
)

type ClientIDAndSecretGenerator interface {
	GenerateClientID() (string, error)
	GenerateClientSecret() (string, error)
}

type oidcClientController struct {
	secretClient    corev1.SecretClient
	secretCache     corev1.SecretCache
	oidcClient      wrangmgmtv3.OIDCClientClient
	oidcClientCache wrangmgmtv3.OIDCClientCache
	generator       ClientIDAndSecretGenerator
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	oidcClient := wContext.Mgmt.OIDCClient()
	controller := &oidcClientController{
		secretClient:    wContext.Core.Secret(),
		secretCache:     wContext.Core.Secret().Cache(),
		oidcClient:      wContext.Mgmt.OIDCClient(),
		oidcClientCache: wContext.Mgmt.OIDCClient().Cache(),
		generator:       &randomstring.Generator{},
	}
	oidcClient.OnChange(ctx, "oidcclient-change", controller.onChange)
	oidcClient.OnRemove(ctx, "oidcclient-remove", controller.onRemove)
}

// onChange sets a new client id in the status field, and creates a k8s with the client secret.
func (c *oidcClientController) onChange(_ string, oidcClient *v3.OIDCClient) (*v3.OIDCClient, error) {
	if oidcClient == nil {
		return nil, nil
	}

	clientID := oidcClient.Status.ClientID

	// generate client id
	if oidcClient.Status.ClientID == "" {
		var err error
		clientID, err = c.generator.GenerateClientID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate clientID: %v", err)
		}

		clients, err := c.oidcClientCache.List(labels.Everything())
		for _, client := range clients {
			if client.Status.ClientID == clientID {
				return nil, fmt.Errorf("client id already exists")
			}
		}
		patchData := map[string]interface{}{
			"status": map[string]string{
				"clientID": clientID,
			},
		}

		patchBytes, err := json.Marshal(patchData)
		if err != nil {
			return nil, err
		}
		// add client id to status
		_, err = c.oidcClient.Patch(oidcClient.Name, types.MergePatchType, patchBytes)
		if err != nil {
			return nil, err
		}
	}

	k8sSecret, err := c.secretCache.Get(secretNamespace, clientID)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	// generate client secret and store it in a k8s secret.
	if errors.IsNotFound(err) {
		clientSecret, err := c.generator.GenerateClientSecret()
		if err != nil {
			return nil, err
		}

		k8sSecret, err = c.secretClient.Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clientID,
				Namespace: secretNamespace,
			},
			StringData: map[string]string{
				secretKeyPrefix + "1": clientSecret,
			},
		})
		if err != nil && !errors.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// create another client secret if the cattle.io/oidc-client-secret-create annotation is present.
	// keys are incrementing. e.g. client-secret-1, client-secret-2,...
	if _, ok := oidcClient.Annotations[createClientSecretAnn]; ok {
		clientSecret, err := c.generator.GenerateClientSecret()
		if err != nil {
			return nil, err
		}
		maxSecretKeyCounter := 1
		for key, _ := range k8sSecret.Data {
			split := strings.Split(key, "-")
			if len(split) != 3 {
				return nil, fmt.Errorf("invalid key found in secret")
			}
			num, err := strconv.Atoi(split[2])
			if err != nil {
				return nil, err
			}
			if num > maxSecretKeyCounter {
				maxSecretKeyCounter = num
			}
		}
		k8sSecret.Data[secretKeyPrefix+strconv.Itoa(maxSecretKeyCounter+1)] = []byte(clientSecret)
		_, err = c.secretClient.Update(k8sSecret)
		if err != nil {
			return nil, err
		}
		delete(oidcClient.Annotations, createClientSecretAnn)
		_, err = c.oidcClient.Update(oidcClient)
		if err != nil {
			return nil, err
		}
	}

	// regenerate client secret if the cattle.io/oidc-client-secret-create annotation is present.
	// client secrets ids are comma separated
	if clientSecretIDs, ok := oidcClient.Annotations[regenerateClientSecretAnn]; ok {
		csids := strings.Split(clientSecretIDs, ",")
		for _, csid := range csids {
			if _, ok := k8sSecret.Data[csid]; ok {
				clientSecret, err := c.generator.GenerateClientSecret()
				if err != nil {
					return nil, err
				}
				k8sSecret.Data[csid] = []byte(clientSecret)
			}
		}
		_, err = c.secretClient.Update(k8sSecret)
		if err != nil {
			return nil, err
		}

		delete(oidcClient.Annotations, regenerateClientSecretAnn)
		_, err = c.oidcClient.Update(oidcClient)
		if err != nil {
			return nil, err
		}
	}

	// remove client secret if the cattle.io/oidc-client-secret-create annotation is present.
	// client secrets ids are comma separated
	if clientSecretIDs, ok := oidcClient.Annotations[removeClientSecretAnn]; ok {
		csids := strings.Split(clientSecretIDs, ",")
		for _, csid := range csids {
			delete(k8sSecret.Data, csid)
		}
		_, err = c.secretClient.Update(k8sSecret)
		if err != nil {
			return nil, err
		}

		delete(oidcClient.Annotations, removeClientSecretAnn)
		_, err = c.oidcClient.Update(oidcClient)
		if err != nil {
			return nil, err
		}
	}

	return oidcClient, nil
}

// onRemove removes the k8s secret that contains the client secret.
func (c *oidcClientController) onRemove(_ string, oidcClient *v3.OIDCClient) (*v3.OIDCClient, error) {
	err := c.secretClient.Delete(secretNamespace, oidcClient.Status.ClientID, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	return oidcClient, nil
}
