package auth

import (
	"fmt"
	"testing"
	"time"

	management "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSyncEnsureUserRetentionLabels(t *testing.T) {
	userID := "u-abcdef"
	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		return userAttribute.DeepCopy(), nil
	})

	var ensureLabelsCalledTimes int
	controller := UserAttributeController{
		userAttributes: userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error {
			ensureLabelsCalledTimes++
			return nil
		},
	}

	newAttribs := func(lastLogin metav1.Time) *v3.UserAttribute {
		return &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{
				Name: userID,
			},
			LastLogin: &lastLogin,
		}
	}

	// Make sure labeler was called.
	_, err := controller.sync("", newAttribs(metav1.NewTime(time.Now())))
	require.NoError(t, err)
	assert.Equal(t, 1, ensureLabelsCalledTimes)
}

func TestSyncProviderRefreshNoConflict(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var (
		userAttributesGetCalledTimes    int
		userAttributesUpdateCalledTimes int
		providerRefreshCalledTimes      int
	)

	now := time.Now().Truncate(time.Second)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		userAttributesGetCalledTimes++
		return attribs.DeepCopy(), nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		userAttributesUpdateCalledTimes++
		attribs = userAttribute.DeepCopy()
		return attribs, nil
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalledTimes++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals = map[string]v3.Principals{"activedirectory": {}}
			a.ExtraByProvider = map[string]map[string][]string{"activedirectory": {}}
			return a, nil
		},
	}

	obj, err := controller.sync("", attribs)
	require.NoError(t, err)

	synced, ok := obj.(*v3.UserAttribute)
	assert.True(t, ok)
	assert.NotNil(t, synced)

	assert.Equal(t, 1, providerRefreshCalledTimes)
	assert.Equal(t, 1, userAttributesGetCalledTimes)
	assert.Equal(t, 1, userAttributesUpdateCalledTimes)

	assert.False(t, synced.NeedsRefresh)
	assert.Equal(t, now.Format(time.RFC3339), synced.LastRefresh)
	assert.Contains(t, synced.GroupPrincipals, "activedirectory")
	assert.Contains(t, synced.ExtraByProvider, "activedirectory")
}

func TestSyncProviderRefreshConflict(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var (
		userAttributesGetCalledTimes    int
		userAttributesUpdateCalledTimes int
		providerRefreshCalledTimes      int
	)

	groupResource := schema.GroupResource{
		Group:    management.GroupName,
		Resource: v3.UserAttributeResourceName,
	}

	now := time.Now().Truncate(time.Second)

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		userAttributesGetCalledTimes++

		a := attribs.DeepCopy()
		if userAttributesGetCalledTimes > 1 {
			a.LastLogin = &metav1.Time{Time: now}
		}

		return a, nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		userAttributesUpdateCalledTimes++

		if userAttributesUpdateCalledTimes == 1 {
			return nil, apierrors.NewConflict(groupResource, userAttribute.Name, fmt.Errorf("some error"))
		}

		attribs = userAttribute.DeepCopy()
		return attribs, nil
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalledTimes++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals = map[string]v3.Principals{"activedirectory": {}}
			a.ExtraByProvider = map[string]map[string][]string{"activedirectory": {}}
			return a, nil
		},
	}

	obj, err := controller.sync("", attribs)
	require.NoError(t, err)

	synced, ok := obj.(*v3.UserAttribute)
	assert.True(t, ok)
	assert.NotNil(t, synced)

	assert.Equal(t, 1, providerRefreshCalledTimes)
	assert.Equal(t, 2, userAttributesGetCalledTimes)
	// Make sure Update is called the second time.
	assert.Equal(t, 2, userAttributesUpdateCalledTimes)

	// Make sure that changes from the provider refresh call were merged.
	assert.Equal(t, now, synced.LastLogin.Time)
	assert.False(t, synced.NeedsRefresh)
	assert.Equal(t, now.Format(time.RFC3339), synced.LastRefresh)
	assert.Contains(t, synced.GroupPrincipals, "activedirectory")
	assert.Contains(t, synced.ExtraByProvider, "activedirectory")
}

func TestSyncProviderRefreshUpdateNonConflictError(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var (
		userAttributesGetCalledTimes int
		providerRefreshCalledTimes   int
	)

	now := time.Now().Truncate(time.Second)

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		userAttributesGetCalledTimes++

		a := attribs.DeepCopy()
		if userAttributesGetCalledTimes > 1 {
			a.LastLogin = &metav1.Time{Time: now}
		}

		return a, nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		return nil, fmt.Errorf("some error")
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalledTimes++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals = map[string]v3.Principals{"activedirectory": {}}
			a.ExtraByProvider = map[string]map[string][]string{"activedirectory": {}}
			return a, nil
		},
	}

	_, err := controller.sync("", attribs)
	require.Error(t, err)

	assert.Equal(t, 1, providerRefreshCalledTimes)
	assert.Equal(t, 1, userAttributesGetCalledTimes)
}

func TestSyncProviderRefreshErrorAfterHandlingConflict(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var (
		userAttributesGetCalledTimes int
		providerRefreshCalledTimes   int
	)

	groupResource := schema.GroupResource{
		Group:    management.GroupName,
		Resource: v3.UserAttributeResourceName,
	}

	now := time.Now().Truncate(time.Second)

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		userAttributesGetCalledTimes++

		a := attribs.DeepCopy()
		if userAttributesGetCalledTimes > 1 {
			a.LastLogin = &metav1.Time{Time: now}
		}

		return a, nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		return nil, apierrors.NewConflict(groupResource, userAttribute.Name, fmt.Errorf("some error"))
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalledTimes++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals = map[string]v3.Principals{"activedirectory": {}}
			a.ExtraByProvider = map[string]map[string][]string{"activedirectory": {}}
			return a, nil
		},
	}

	_, err := controller.sync("", attribs)
	require.Error(t, err)

	assert.Equal(t, 1, providerRefreshCalledTimes)
	assert.Equal(t, 2, userAttributesGetCalledTimes)
}

func TestSyncGetUserAttributeFails(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		return nil, fmt.Errorf("some error")
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).Times(0)

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
	}

	_, err := controller.sync("", attribs)
	require.Error(t, err)
}

func TestSyncNonTransientProviderError(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var lastUpdated *v3.UserAttribute

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		return attribs.DeepCopy(), nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(ua *v3.UserAttribute) (*v3.UserAttribute, error) {
		lastUpdated = ua.DeepCopy()
		return lastUpdated, nil
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			return nil, &common.NonTransientError{Err: fmt.Errorf("user not found")}
		},
	}

	obj, err := controller.sync("", attribs)
	require.NoError(t, err)
	assert.Nil(t, obj)

	require.NotNil(t, lastUpdated)
	assert.False(t, lastUpdated.NeedsRefresh)
	assert.Contains(t, lastUpdated.Annotations, common.ProviderRefreshErrorAnnotation)
	assert.Contains(t, lastUpdated.Annotations[common.ProviderRefreshErrorAnnotation], "user not found")
}

func TestSyncRequestEntityTooLarge(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var lastUpdated *v3.UserAttribute
	var updateCount int

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		return attribs.DeepCopy(), nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(ua *v3.UserAttribute) (*v3.UserAttribute, error) {
		updateCount++
		if updateCount == 1 {
			return nil, apierrors.NewRequestEntityTooLargeError("limit is 3145728")
		}
		lastUpdated = ua.DeepCopy()
		return lastUpdated, nil
	})

	now := time.Now().Truncate(time.Second)

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			return a, nil
		},
	}

	obj, err := controller.sync("", attribs)
	require.NoError(t, err)
	assert.Nil(t, obj)

	require.NotNil(t, lastUpdated)
	assert.False(t, lastUpdated.NeedsRefresh)
	assert.Contains(t, lastUpdated.Annotations, common.ProviderRefreshErrorAnnotation)
}

func TestSyncSkipsAnnotatedUserAttribute(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
			Annotations: map[string]string{
				common.ProviderRefreshErrorAnnotation: "user not found",
			},
		},
		NeedsRefresh: true,
	}

	var lastUpdated *v3.UserAttribute
	providerRefreshCalled := false

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		return attribs.DeepCopy(), nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(ua *v3.UserAttribute) (*v3.UserAttribute, error) {
		lastUpdated = ua.DeepCopy()
		return lastUpdated, nil
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalled = true
			return attribs, nil
		},
	}

	obj, err := controller.sync("", attribs)
	require.NoError(t, err)

	assert.False(t, providerRefreshCalled, "provider refresh should not be called for annotated user")

	synced, ok := obj.(*v3.UserAttribute)
	require.True(t, ok)
	assert.False(t, synced.NeedsRefresh)
	// Annotation is preserved (only cleared on login).
	assert.Contains(t, synced.Annotations, common.ProviderRefreshErrorAnnotation)
}
