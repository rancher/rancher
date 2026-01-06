package oidcprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/oidc/randomstring"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

const (
	createClientSecretAnn          = "cattle.io/oidc-client-secret-create"
	removeClientSecretAnn          = "cattle.io/oidc-client-secret-remove"
	regenerateClientSecretAnn      = "cattle.io/oidc-client-secret-regenerate"
	clientSecretCreatedAtPrefixAnn = "cattle.io/oidc-client-secret-created-at_"
	clientSecretUsedAtPrefixAnn    = "cattle.io.oidc-client-secret-used-"
	secretKeyPrefix                = "client-secret-"
	secretNamespace                = "cattle-oidc-client-secrets"
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
	now             func() time.Time
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	oidcClient := wContext.Mgmt.OIDCClient()
	controller := &oidcClientController{
		secretClient:    wContext.Core.Secret(),
		secretCache:     wContext.Core.Secret().Cache(),
		oidcClient:      wContext.Mgmt.OIDCClient(),
		oidcClientCache: wContext.Mgmt.OIDCClient().Cache(),
		generator:       &randomstring.Generator{},
		now:             time.Now,
	}
	oidcClient.OnChange(ctx, "oidcclient-change", controller.onChange)
}

// onChange sets a new client id in the status field, and creates a k8s with the client secret.
func (c *oidcClientController) onChange(_ string, oidcClient *v3.OIDCClient) (*v3.OIDCClient, error) {
	if oidcClient == nil {
		return nil, nil
	}
	clientID := oidcClient.Status.ClientID

	// generate client id
	if clientID == "" {
		var err error
		clientID, err = c.generator.GenerateClientID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate clientID: %w", err)
		}

		clients, err := c.oidcClientCache.List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list OIDC clients: %w", err)
		}

		if slices.ContainsFunc(clients, func(client *v3.OIDCClient) bool {
			return client.Status.ClientID == clientID
		}) {
			return nil, fmt.Errorf("client id '%s' already exists", clientID)
		}
	}

	k8sSecret, err := c.secretCache.Get(secretNamespace, clientID)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	// generate client secret and store it in a k8s secret.
	if apierrors.IsNotFound(err) {
		clientSecret, err := c.generator.GenerateClientSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate client secret: %w", err)
		}
		clientSecretName := secretKeyPrefix + "1"
		k8sSecret, err = c.secretClient.Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: clientID,
				Annotations: map[string]string{
					clientSecretCreatedAtPrefixAnn + clientSecretName: fmt.Sprintf("%d", c.now().Unix()),
				},
				Namespace: secretNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "management.cattle.io/v3",
						Kind:       "OIDCClient",
						Name:       oidcClient.Name,
						UID:        oidcClient.UID,
					},
				},
			},
			StringData: map[string]string{
				clientSecretName: clientSecret,
			},
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("failed to create client secret: %w", err)
		}

		// add client id to status
		patchData := map[string]interface{}{
			"status": map[string]interface{}{
				"clientID": clientID,
			},
		}
		patchBytes, err := json.Marshal(patchData)
		if err != nil {
			// delete previously created secret as it will be created when a new clientID is generated in the next reconciliation loop
			return nil, errors.Join(c.secretClient.Delete(secretNamespace, clientID, &metav1.DeleteOptions{}), fmt.Errorf("failed to create clientID status patch: %w", err))
		}
		oidcClient, err = c.oidcClient.Patch(oidcClient.Name, types.MergePatchType, patchBytes, "status")
		if err != nil {
			// delete previously created secret as it will be created when a new clientID is generated in the next reconciliation loop
			return nil, errors.Join(c.secretClient.Delete(secretNamespace, clientID, &metav1.DeleteOptions{}), fmt.Errorf("failed to apply clientID status patch: %w", err))
		}
	}

	// create another client secret if the cattle.io/oidc-client-secret-create annotation is present.
	// keys are incrementing. e.g. client-secret-1, client-secret-2,...
	if _, ok := oidcClient.Annotations[createClientSecretAnn]; ok {
		clientSecret, err := c.generator.GenerateClientSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate client secret: %w", err)
		}
		clientSecretName, err := findNextSecretKey(k8sSecret.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to find next secret key: %w", err)
		}
		if k8sSecret.Data == nil {
			k8sSecret.Data = map[string][]byte{}
		}
		k8sSecret.Data[clientSecretName] = []byte(clientSecret)
		if k8sSecret.Annotations == nil {
			k8sSecret.Annotations = map[string]string{}
		}
		k8sSecret.Annotations[clientSecretCreatedAtPrefixAnn+clientSecretName] = fmt.Sprintf("%d", c.now().Unix())
		k8sSecret, err = c.secretClient.Update(k8sSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret: %w", err)
		}
		//delete annotation
		delete(oidcClient.Annotations, createClientSecretAnn)
		oidcClient, err = c.oidcClient.Update(oidcClient)
		if err != nil {
			return nil, fmt.Errorf("failed to update OIDC client: %w", err)
		}
	}

	// regenerate client secret if the cattle.io/oidc-client-secret-regenerate annotation is present.
	// client secrets ids are comma separated
	if clientSecretIDs, ok := oidcClient.Annotations[regenerateClientSecretAnn]; ok {
		csids := strings.Split(clientSecretIDs, ",")
		for _, csid := range csids {
			if _, ok := k8sSecret.Data[csid]; ok {
				clientSecret, err := c.generator.GenerateClientSecret()
				if err != nil {
					return nil, fmt.Errorf("failed to generate client secret: %w", err)
				}
				k8sSecret.Data[csid] = []byte(clientSecret)
			}
		}
		k8sSecret, err = c.secretClient.Update(k8sSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret: %w", err)
		}

		// delete annotation
		delete(oidcClient.Annotations, regenerateClientSecretAnn)
		oidcClient, err = c.oidcClient.Update(oidcClient)
		if err != nil {
			return nil, fmt.Errorf("failed to update OIDC client: %w", err)
		}
	}

	// remove client secret if the cattle.io/oidc-client-secret-remove annotation is present.
	// client secrets ids are comma separated
	if clientSecretIDs, ok := oidcClient.Annotations[removeClientSecretAnn]; ok {
		csids := strings.Split(clientSecretIDs, ",")
		for _, csid := range csids {
			delete(k8sSecret.Data, csid)
			delete(k8sSecret.Annotations, clientSecretCreatedAtPrefixAnn+csid)
		}
		k8sSecret, err = c.secretClient.Update(k8sSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to update secret: %w", err)
		}

		// delete annotation
		delete(oidcClient.Annotations, removeClientSecretAnn)
		oidcClient, err = c.oidcClient.Update(oidcClient)
		if err != nil {
			return nil, fmt.Errorf("failed to update OIDC client: %w", err)
		}
	}

	return oidcClient, c.updateStatusIfNeeded(oidcClient, k8sSecret)
}

func (c *oidcClientController) updateStatusIfNeeded(oidcClient *v3.OIDCClient, secret *v1.Secret) error {
	// calculate status
	observedClientSecrets := map[string]v3.OIDCClientSecretStatus{}
	for clientSecretName, clientSecretBytes := range secret.Data {
		clientSecretValue := string(clientSecretBytes)
		lastFiveCharacters := clientSecretValue
		if len(clientSecretValue) > 5 {
			lastFiveCharacters = clientSecretValue[len(clientSecretValue)-5:]
		}
		observedClientSecrets[clientSecretName] = v3.OIDCClientSecretStatus{
			LastFiveCharacters: lastFiveCharacters,
			CreatedAt:          secret.Annotations[clientSecretCreatedAtPrefixAnn+clientSecretName],
			LastUsedAt:         oidcClient.Annotations[clientSecretUsedAtPrefixAnn+clientSecretName],
		}
	}
	observedStatus := v3.OIDCClientStatus{
		ClientID: secret.Name,
	}
	if len(observedClientSecrets) > 0 {
		observedStatus.ClientSecrets = observedClientSecrets
	}

	if !reflect.DeepEqual(oidcClient.Status, observedStatus) {
		patchOp := []map[string]interface{}{
			{
				"op":    "add",
				"path":  "/status",
				"value": observedStatus,
			},
		}
		patchBytes, err := json.Marshal(patchOp)
		if err != nil {
			return fmt.Errorf("failed to create status patch: %w", err)
		}
		oidcClient, err = c.oidcClient.Patch(oidcClient.Name, types.JSONPatchType, patchBytes, "status")
		if err != nil {
			return fmt.Errorf("failed to apply status patch: %w", err)
		}
	}

	return nil
}

func findNextSecretKey(secretData map[string][]byte) (string, error) {
	maxSecretKeyCounter := 0
	for key := range secretData {
		split := strings.Split(key, "-")
		if len(split) != 3 {
			return "", fmt.Errorf("invalid key found in secret")
		}
		num, err := strconv.Atoi(split[2])
		if err != nil {
			return "", fmt.Errorf("invalid key found in secret: %w", err)
		}
		if num > maxSecretKeyCounter {
			maxSecretKeyCounter = num
		}
	}
	return secretKeyPrefix + strconv.Itoa(maxSecretKeyCounter+1), nil
}
