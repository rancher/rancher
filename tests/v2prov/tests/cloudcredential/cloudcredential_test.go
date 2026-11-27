package cloudcredential

import (
	"context"
	"testing"
	"time"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	testCredentialType = "amazonec2"
	testNamespace      = "default"

	credentialNamespace = "cattle-cloud-credentials"
	secretTypePrefix    = "rke.cattle.io/cloud-credential-"
)

var cloudCredentialGVR = schema.GroupVersionResource{
	Group:    "ext.cattle.io",
	Version:  "v1",
	Resource: "cloudcredentials",
}

// CloudCredentialTestSuite is the test suite for CloudCredential integration tests
type CloudCredentialTestSuite struct {
	suite.Suite
	client *clients.Clients
	ctx    context.Context
}

// SetupSuite runs once before all tests
func (s *CloudCredentialTestSuite) SetupSuite() {
	var err error
	s.client, err = clients.New()
	s.NoError(err)
	s.ctx = context.Background()
}

// TearDownSuite runs once after all tests
func (s *CloudCredentialTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func Test_General_CloudCredentials(t *testing.T) {
	// skipping this for now until the webhook changes go in.
	t.Skip()
	suite.Run(t, new(CloudCredentialTestSuite))
}

// TestCreateGetIntegration verifies creating and retrieving a CloudCredential
func (s *CloudCredentialTestSuite) TestCreateGetIntegration() {
	created, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential", s.defaultCloudCredentialSpec())
	s.NoError(err)
	s.NotNil(created.Status.Secret)
	s.NotEmpty(created.Status.Secret.Name)

	testCredentialName := created.Name

	backingSecret, err := s.client.Core.Secret().Get(credentialNamespace, created.Status.Secret.Name, metav1.GetOptions{})
	s.NoError(err)

	s.validateBackingSecret(backingSecret, testCredentialName)
	s.validateCloudCredentialView(created, backingSecret.Name, testCredentialName)

	// Non-empty GetOptions forces non-cache path in store.Get.
	gotNoCache, err := s.getCloudCredential(testCredentialName, metav1.GetOptions{ResourceVersion: "0"})
	s.NoError(err)
	s.validateCloudCredentialView(gotNoCache, backingSecret.Name, testCredentialName)

	// Empty GetOptions uses cache path in store.Get.
	gotCache, err := s.getCloudCredential(testCredentialName, metav1.GetOptions{})
	s.NoError(err)
	s.validateCloudCredentialView(gotCache, backingSecret.Name, testCredentialName)

	s.Equal(gotNoCache.ResourceVersion, gotCache.ResourceVersion)
}

// TestCreateFailsWhenRequiredKeysMissing verifies validation of required fields
func (s *CloudCredentialTestSuite) TestCreateFailsWhenRequiredKeysMissing() {
	tests := []struct {
		name       string
		missingKey string
	}{
		{
			name:       "missing accessKey",
			missingKey: "accessKey",
		},
		{
			name:       "missing secretKey",
			missingKey: "secretKey",
		},
	}

	for _, tt := range tests {
		tt := tt
		s.Run(tt.name, func() {
			spec := s.defaultCloudCredentialSpec()
			delete(spec.Credentials, tt.missingKey)

			_, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-missing", spec)
			s.Error(err)
			s.Contains(err.Error(), tt.missingKey)
		})
	}
}

// TestVisibleFieldsControlsPublicData verifies VisibleFields controls public data exposure
func (s *CloudCredentialTestSuite) TestVisibleFieldsControlsPublicData() {
	spec := s.defaultCloudCredentialSpec()
	spec.VisibleFields = []string{"defaultRegion", "doesNotExist"}

	created, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-visible", spec)
	s.NoError(err)
	s.NotNil(created.Status.Secret)

	got, err := s.getCloudCredential(created.Name, metav1.GetOptions{})
	s.NoError(err)
	s.NotNil(got.Status.PublicData)

	s.Equal("us-west-2", got.Status.PublicData["defaultRegion"])
	_, hasAccessKey := got.Status.PublicData["accessKey"]
	s.False(hasAccessKey)
	_, hasSecretKey := got.Status.PublicData["secretKey"]
	s.False(hasSecretKey)
	_, hasUnknown := got.Status.PublicData["doesNotExist"]
	s.False(hasUnknown)
}

// TestGetFailsWhenNotFound verifies Get fails for missing credentials
func (s *CloudCredentialTestSuite) TestGetFailsWhenNotFound() {
	missingName := "v2prov-cloudcredential-missing-get-nonexistent"

	_, err := s.getCloudCredential(missingName, metav1.GetOptions{})
	s.Error(err)
	s.True(apierrors.IsNotFound(err))
}

// TestUpdateFailsWhenTypeChanges verifies type is immutable
func (s *CloudCredentialTestSuite) TestUpdateFailsWhenTypeChanges() {
	created, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-update-type", s.defaultCloudCredentialSpec())
	s.NoError(err)

	update := created.DeepCopy()
	update.Spec = s.defaultCloudCredentialSpec()
	update.Spec.Type = "azure"

	_, err = s.updateCloudCredential(update, metav1.UpdateOptions{})
	s.Error(err)
	s.Contains(err.Error(), "spec.type is immutable")
}

// TestUpdateFailsWhenCredentialNotFound verifies update fails after deletion
func (s *CloudCredentialTestSuite) TestUpdateFailsWhenCredentialNotFound() {
	created, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-update-notfound", s.defaultCloudCredentialSpec())
	s.NoError(err)

	err = s.deleteCloudCredential(created.Name, metav1.DeleteOptions{})
	s.NoError(err)

	update := created.DeepCopy()
	update.Spec = s.defaultCloudCredentialSpec()
	update.Spec.Description = "updated after delete"

	_, err = s.updateCloudCredential(update, metav1.UpdateOptions{})
	s.Error(err)
	s.True(apierrors.IsNotFound(err))
}

// TestDeleteFailsWhenNotFound verifies delete fails for missing credentials
func (s *CloudCredentialTestSuite) TestDeleteFailsWhenNotFound() {
	missingName := "v2prov-cloudcredential-missing-delete-nonexistent"

	err := s.deleteCloudCredential(missingName, metav1.DeleteOptions{})
	s.Error(err)
	s.True(apierrors.IsNotFound(err))
}

// TestDeleteForceDeleteIntegration verifies force delete works correctly
func (s *CloudCredentialTestSuite) TestDeleteForceDeleteIntegration() {
	created, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-force-delete", s.defaultCloudCredentialSpec())
	s.NoError(err)

	force := int64(0)
	err = s.deleteCloudCredential(created.Name, metav1.DeleteOptions{
		GracePeriodSeconds: &force,
	})
	s.NoError(err)

	_, err = s.getCloudCredential(created.Name, metav1.GetOptions{})
	s.Error(err)
	s.True(apierrors.IsNotFound(err))
}

// TestListIntegration verifies listing CloudCredentials
func (s *CloudCredentialTestSuite) TestListIntegration() {
	// Create two credentials to test listing
	cred1, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-list-1", s.defaultCloudCredentialSpec())
	s.NoError(err)
	cred2, err := s.createCloudCredentialWithSpec("v2prov-cloudcredential-list-2", s.defaultCloudCredentialSpec())
	s.NoError(err)

	unstrList, err := s.client.Dynamic.Resource(cloudCredentialGVR).
		Namespace(testNamespace).
		List(s.ctx, metav1.ListOptions{})
	s.NoError(err)

	var found1, found2 bool
	for _, unstr := range unstrList.Items {
		cred, err := toCloudCredential(&unstr)
		s.NoError(err)

		if cred.Name == cred1.Name {
			found1 = true
			s.Nil(cred.Spec.Credentials)
			s.NotNil(cred.Status.PublicData)
		} else if cred.Name == cred2.Name {
			found2 = true
			s.Nil(cred.Spec.Credentials)
			s.NotNil(cred.Status.PublicData)
		}
	}

	s.True(found1, "first credential not found in list")
	s.True(found2, "second credential not found in list")
}

// TestWatchIntegration verifies watching CloudCredentials
func (s *CloudCredentialTestSuite) TestWatchIntegration() {
	watcher, err := s.client.Dynamic.Resource(cloudCredentialGVR).
		Namespace(testNamespace).
		Watch(s.ctx, metav1.ListOptions{})
	s.NoError(err)
	defer watcher.Stop()

	// 1. Test ADD event
	credNamePrefix := "v2prov-cloudcredential-watch"
	spec := s.defaultCloudCredentialSpec()
	created, err := s.createCloudCredentialWithSpec(credNamePrefix, spec)
	s.NoError(err)

	addDone := make(chan struct{})
	go func() {
		for event := range watcher.ResultChan() {
			unstr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			cred, err := toCloudCredential(unstr)
			s.NoError(err)

			if cred.Name == created.Name && event.Type == watch.Added {
				s.Nil(cred.Spec.Credentials)
				close(addDone)
				return
			}
		}
	}()

	select {
	case <-addDone:
	case <-time.After(10 * time.Second):
		s.Fail("did not receive ADDED event")
	}

	// 2. Test MODIFY event
	// Update mutable fields to trigger a modification
	update := created.DeepCopy()
	update.Spec.Credentials = spec.Credentials
	update.Spec.Description = "updated description for watch"
	updated, err := s.updateCloudCredential(update, metav1.UpdateOptions{})
	s.NoError(err)

	modifyDone := make(chan struct{})
	go func() {
		for event := range watcher.ResultChan() {
			unstr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			cred, err := toCloudCredential(unstr)
			s.NoError(err)

			if cred.Name == updated.Name && event.Type == watch.Modified && cred.Spec.Description == "updated description for watch" {
				close(modifyDone)
				return
			}
		}
	}()

	select {
	case <-modifyDone:
	case <-time.After(10 * time.Second):
		s.Fail("did not receive MODIFIED event")
	}

	// 3. Test DELETE event
	err = s.deleteCloudCredential(created.Name, metav1.DeleteOptions{})
	s.NoError(err)

	deleteDone := make(chan struct{})
	go func() {
		for event := range watcher.ResultChan() {
			unstr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			cred, err := toCloudCredential(unstr)
			s.NoError(err)

			if cred.Name == created.Name && event.Type == watch.Deleted {
				close(deleteDone)
				return
			}
		}
	}()

	select {
	case <-deleteDone:
	case <-time.After(10 * time.Second):
		s.Fail("did not receive DELETED event")
	}
}

// createCloudCredentialWithSpec creates a CloudCredential with the given spec
func (s *CloudCredentialTestSuite) createCloudCredentialWithSpec(namePrefix string, spec extv1.CloudCredentialSpec) (*extv1.CloudCredential, error) {
	token, err := randomtoken.Generate()
	s.NoError(err)
	name := namePrefix + "-" + token[:8]

	s.client.OnClose(func() {
		_ = s.client.Dynamic.Resource(cloudCredentialGVR).Namespace(testNamespace).Delete(s.ctx, name, metav1.DeleteOptions{})
	})

	createObj := &extv1.CloudCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
		},
		Spec: spec,
	}

	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(createObj)
	if err != nil {
		return nil, err
	}

	created, err := s.client.Dynamic.Resource(cloudCredentialGVR).
		Namespace(testNamespace).
		Create(s.ctx, &unstructured.Unstructured{Object: unstrObj}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return toCloudCredential(created)
}

// defaultCloudCredentialSpec returns a default CloudCredential spec for testing
func (s *CloudCredentialTestSuite) defaultCloudCredentialSpec() extv1.CloudCredentialSpec {
	return extv1.CloudCredentialSpec{
		Type:        testCredentialType,
		Description: "v2prov cloudcredential integration test",
		Credentials: map[string]string{
			"accessKey":     "v2prov-access",
			"secretKey":     "v2prov-secret",
			"defaultRegion": "us-west-2",
		},
	}
}

// getCloudCredential retrieves a CloudCredential by name
func (s *CloudCredentialTestSuite) getCloudCredential(name string, opts metav1.GetOptions) (*extv1.CloudCredential, error) {
	out, err := s.client.Dynamic.Resource(cloudCredentialGVR).
		Namespace(testNamespace).
		Get(s.ctx, name, opts)
	if err != nil {
		return nil, err
	}

	return toCloudCredential(out)
}

// updateCloudCredential updates a CloudCredential
func (s *CloudCredentialTestSuite) updateCloudCredential(credential *extv1.CloudCredential, opts metav1.UpdateOptions) (*extv1.CloudCredential, error) {
	unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(credential)
	if err != nil {
		return nil, err
	}

	out, err := s.client.Dynamic.Resource(cloudCredentialGVR).
		Namespace(testNamespace).
		Update(s.ctx, &unstructured.Unstructured{Object: unstrObj}, opts)
	if err != nil {
		return nil, err
	}

	return toCloudCredential(out)
}

// deleteCloudCredential deletes a CloudCredential by name
func (s *CloudCredentialTestSuite) deleteCloudCredential(name string, opts metav1.DeleteOptions) error {
	return s.client.Dynamic.Resource(cloudCredentialGVR).
		Namespace(testNamespace).
		Delete(s.ctx, name, opts)
}

func toCloudCredential(obj *unstructured.Unstructured) (*extv1.CloudCredential, error) {
	cred := &extv1.CloudCredential{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cred); err != nil {
		return nil, err
	}
	return cred, nil
}

// validateBackingSecret validates the backing Kubernetes secret
func (s *CloudCredentialTestSuite) validateBackingSecret(secret *corev1.Secret, credentialName string) {
	s.Equal(credentialNamespace, secret.Namespace)
	s.Equal(corev1.SecretType(secretTypePrefix+testCredentialType), secret.Type)
	s.Equal("true", secret.Labels["cattle.io/cloud-credential"])
	s.Equal(credentialName, secret.Labels["cattle.io/cloud-credential-name"])
	s.Equal(testNamespace, secret.Labels["cattle.io/cloud-credential-namespace"])
	s.NotEmpty(secret.Labels["cattle.io/cloud-credential-owner"])
	s.Equal("v2prov cloudcredential integration test", secret.Annotations["cattle.io/cloud-credential-description"])
	s.NotEmpty(secret.Annotations["field.cattle.io/creatorId"])
	s.Empty(secret.Annotations["cattle.io/cloud-credential-owner"])

	s.Equal("v2prov-access", string(secret.Data["accessKey"]))
	s.Equal("v2prov-secret", string(secret.Data["secretKey"]))
	s.Equal("us-west-2", string(secret.Data["defaultRegion"]))
}

// validateCloudCredentialView validates the CloudCredential view
func (s *CloudCredentialTestSuite) validateCloudCredentialView(cred *extv1.CloudCredential, backingSecretName, credentialName string) {
	s.NotNil(cred)
	s.Equal(credentialName, cred.Name)
	s.Equal(testNamespace, cred.Namespace)
	s.Equal(testCredentialType, cred.Spec.Type)
	s.Equal("v2prov cloudcredential integration test", cred.Spec.Description)
	s.Nil(cred.Spec.Credentials)

	s.NotNil(cred.Status.Secret)
	s.Equal(credentialNamespace, cred.Status.Secret.Namespace)
	s.Equal(backingSecretName, cred.Status.Secret.Name)

	s.NotNil(cred.Status.PublicData)
	s.Equal("v2prov-access", cred.Status.PublicData["accessKey"])
	s.Equal("us-west-2", cred.Status.PublicData["defaultRegion"])
	_, hasSecret := cred.Status.PublicData["secretKey"]
	s.False(hasSecret)
}
