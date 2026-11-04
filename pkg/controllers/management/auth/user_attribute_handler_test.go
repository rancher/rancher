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
	// The original error from the first update is returned.
	assert.True(t, apierrors.IsConflict(err))
	assert.ErrorContains(t, err, "error updating user attribute "+userID+" after provider refresh")

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

func TestSyncNonTransientProviderErrorRetriesOnConflict(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	var (
		updateAttempts int
		lastUpdated    *v3.UserAttribute
	)

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		return attribs.DeepCopy(), nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(ua *v3.UserAttribute) (*v3.UserAttribute, error) {
		updateAttempts++
		if updateAttempts == 1 {
			return nil, apierrors.NewConflict(
				schema.GroupResource{Group: management.GroupName, Resource: "userattributes"},
				userID, fmt.Errorf("the object has been modified"))
		}
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

	assert.Equal(t, 2, updateAttempts, "skipRefresh should retry after a 409 conflict")
	require.NotNil(t, lastUpdated)
	assert.False(t, lastUpdated.NeedsRefresh)
	assert.Contains(t, lastUpdated.Annotations, common.ProviderRefreshErrorAnnotation)
}

func TestSyncNonTransientProviderErrorIgnoresUserAttributeNotFound(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		NeedsRefresh: true,
	}

	ctrl := gomock.NewController(t)

	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	gets := 0
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		gets++
		if gets == 1 {
			// First read is the sync handler's own re-fetch at the top of sync.
			return attribs.DeepCopy(), nil
		}
		// Subsequent read inside skipRefresh's retry: the user attribute is
		// gone (e.g. the user was deleted while the refresh was in flight).
		return nil, apierrors.NewNotFound(
			schema.GroupResource{Group: management.GroupName, Resource: "userattributes"},
			name)
	})
	// No Update expectation: the retry must short-circuit on NotFound.

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			return nil, &common.NonTransientError{Err: fmt.Errorf("user not found")}
		},
	}

	obj, err := controller.sync("", attribs)
	require.NoError(t, err, "Must not error when the user attribute is gone")
	assert.Nil(t, obj)
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

func TestSyncProviderRefreshConflictKeepsConcurrentChanges(t *testing.T) {
	t.Parallel()

	userID := "u-abcdef"
	stored := &v3.UserAttribute{
		ObjectMeta:   metav1.ObjectMeta{Name: userID},
		NeedsRefresh: true,
		GroupPrincipals: map[string]v3.Principals{
			"okta":  {Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "okta_group://g1"}}, {ObjectMeta: metav1.ObjectMeta{Name: "okta_group://g2"}}}},
			"azure": {Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "azuread_group://a1"}}}},
		},
		ExtraByProvider: map[string]map[string][]string{
			"okta": {
				"principalid": {"okta_user://alice"},
				"externalid":  {"00u123"},
				"email":       {"old@example.com"},
			},
			"azure": {
				"principalid": {"azuread_user://alice"},
				"stale":       {"value"},
			},
		},
	}

	groupResource := schema.GroupResource{
		Group:    management.GroupName,
		Resource: v3.UserAttributeResourceName,
	}
	now := time.Now().Truncate(time.Second)

	var getCalls, updateCalls, refreshCalls int
	var updated *v3.UserAttribute

	ctrl := gomock.NewController(t)
	userAttributeClient := fake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributeClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
		getCalls++
		a := stored.DeepCopy()
		if getCalls > 1 {
			// Changes SCIM made while the refresh was running.
			a.GroupPrincipals["okta"] = v3.Principals{Items: []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "okta_group://g1"}}}}
			a.ExtraByProvider["okta"]["email"] = []string{"new@example.com"}
		}
		return a, nil
	})
	userAttributeClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
		updateCalls++
		if updateCalls == 1 {
			return nil, apierrors.NewConflict(groupResource, userAttribute.Name, fmt.Errorf("some error"))
		}
		updated = userAttribute.DeepCopy()
		return updated, nil
	})

	controller := UserAttributeController{
		userAttributes:            userAttributeClient,
		ensureUserRetentionLabels: func(attribs *v3.UserAttribute) error { return nil },
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			refreshCalls++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals["azure"] = v3.Principals{Items: []v3.Principal{
				{ObjectMeta: metav1.ObjectMeta{Name: "azuread_group://a1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "azuread_group://a2"}},
			}}
			a.GroupPrincipals["github"] = v3.Principals{}
			a.ExtraByProvider["azure"]["username"] = []string{"alice@example.com"}
			delete(a.ExtraByProvider["azure"], "stale")
			a.ExtraByProvider["github"] = map[string][]string{}
			return a, nil
		},
	}

	_, err := controller.sync("", stored)
	require.NoError(t, err)

	assert.Equal(t, 1, refreshCalls)
	assert.Equal(t, 2, updateCalls)
	require.NotNil(t, updated)

	assert.False(t, updated.NeedsRefresh)
	assert.Equal(t, now.Format(time.RFC3339), updated.LastRefresh)

	// SCIM's changes made during the refresh are kept.
	assert.Equal(t, []v3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "okta_group://g1"}}}, updated.GroupPrincipals["okta"].Items)
	assert.Equal(t, map[string][]string{
		"principalid": {"okta_user://alice"},
		"externalid":  {"00u123"},
		"email":       {"new@example.com"},
	}, updated.ExtraByProvider["okta"])

	// The refresh's own changes are applied.
	assert.Len(t, updated.GroupPrincipals["azure"].Items, 2)
	assert.Equal(t, map[string][]string{
		"principalid": {"azuread_user://alice"},
		"username":    {"alice@example.com"},
	}, updated.ExtraByProvider["azure"])
	assert.Contains(t, updated.GroupPrincipals, "github")
	assert.Contains(t, updated.ExtraByProvider, "github")
}

func TestApplyRefreshChanges(t *testing.T) {
	t.Parallel()

	principal := func(name string) v3.Principal {
		return v3.Principal{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}

	tests := []struct {
		name      string
		before    *v3.UserAttribute
		refreshed *v3.UserAttribute
		current   *v3.UserAttribute
		want      *v3.UserAttribute
	}{
		{
			name: "provider removed by the refresh",
			before: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			refreshed: &v3.UserAttribute{},
			current: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			want: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{},
				ExtraByProvider: map[string]map[string][]string{},
			},
		},
		{
			name: "reordered groups are unchanged",
			before: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1"), principal("g2")}}},
			},
			refreshed: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g2"), principal("g1")}}},
			},
			current: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
			},
			want: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
			},
		},
		{
			name: "extras merge key by key within one provider",
			before: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"old"}, "email": {"e1"}}},
			},
			refreshed: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"new"}, "email": {"e1"}}},
			},
			current: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"old"}, "email": {"e2"}}},
			},
			want: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"new"}, "email": {"e2"}}},
			},
		},
		{
			name: "refresh list wins when both change one provider's groups",
			before: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1"), principal("g2")}}},
			},
			refreshed: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1"), principal("g2"), principal("g3")}}},
			},
			current: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
			},
			want: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1"), principal("g2"), principal("g3")}}},
			},
		},
		{
			name: "current without the provider's extras takes the refreshed entry",
			before: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"old"}}},
			},
			refreshed: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"new"}}},
			},
			current: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{},
			},
			want: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"new"}}},
			},
		},
		{
			name: "entry the refresh left alone stays removed",
			before: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			refreshed: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			current: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{},
			},
			want: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{},
			},
		},
		{
			name: "current entry is null",
			before: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			refreshed: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "username": {"u"}}},
			},
			current: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": nil},
			},
			want: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"username": {"u"}}},
			},
		},
		{
			name:   "entry added by another writer during the refresh",
			before: &v3.UserAttribute{},
			refreshed: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			current: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"email": {"e"}}},
			},
			want: &v3.UserAttribute{
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}, "email": {"e"}}},
			},
		},
		{
			name:   "current without maps",
			before: &v3.UserAttribute{},
			refreshed: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
			current: &v3.UserAttribute{},
			want: &v3.UserAttribute{
				GroupPrincipals: map[string]v3.Principals{"okta": {Items: []v3.Principal{principal("g1")}}},
				ExtraByProvider: map[string]map[string][]string{"okta": {"principalid": {"p"}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			applyRefreshChanges(tt.before, tt.refreshed, tt.current)

			assert.Equal(t, tt.want, tt.current)
		})
	}
}
