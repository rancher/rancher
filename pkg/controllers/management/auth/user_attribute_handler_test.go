package auth

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	management "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	mgmtFakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

func TestSyncEnsureLastLoginLabel(t *testing.T) {
	userID := "u-abcdef"
	user := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		Enabled: pointer.BoolPtr(true),
	}

	var userUpdateCalledTimes int

	controller := UserAttributeController{
		userLister: &mgmtFakes.UserListerMock{
			GetFunc: func(namespace, name string) (*v3.User, error) {
				return user, nil
			},
		},
		users: &mgmtFakes.UserInterfaceMock{
			UpdateFunc: func(user *v3.User) (*v3.User, error) {
				userUpdateCalledTimes++
				user = user.DeepCopy()
				return user, nil
			},
		},
		userAttributes: &mgmtFakes.UserAttributeInterfaceMock{
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				return userAttribute.DeepCopy(), nil
			},
		},
	}

	newAttribs := func(lastLogin metav1.Time) *v3.UserAttribute {
		return &v3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{
				Name: userID,
			},
			LastLogin: lastLogin,
		}
	}

	// Make sure zero value time is ignored.
	now := time.Time{}
	_, err := controller.sync("", &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		LastLogin: metav1.NewTime(now),
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, userUpdateCalledTimes)

	// Make sure the label is created.
	now = time.Now().Add(-1 * time.Minute).Truncate(time.Second)

	_, err = controller.sync("", newAttribs(metav1.NewTime(now)))
	assert.NoError(t, err)

	assert.Contains(t, user.Labels, labelLastLoginKey)
	assert.Equal(t, strconv.FormatInt(now.Unix(), 10), user.Labels[labelLastLoginKey])
	assert.Equal(t, 1, userUpdateCalledTimes)

	// Make sure the label is updated.
	now = time.Now().Truncate(time.Second)

	_, err = controller.sync("", newAttribs(metav1.NewTime(now)))
	assert.NoError(t, err)

	assert.Contains(t, user.Labels, labelLastLoginKey)
	assert.Equal(t, strconv.FormatInt(now.Unix(), 10), user.Labels[labelLastLoginKey])
	assert.Equal(t, 2, userUpdateCalledTimes)

	// Make sure the user is not updated if the last login time remains the same.
	_, err = controller.sync("", newAttribs(metav1.NewTime(now)))
	assert.NoError(t, err)
	assert.Equal(t, 2, userUpdateCalledTimes)
}

func TestSyncProviderRefreshNoConflict(t *testing.T) {
	userID := "u-abcdef"
	user := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		Enabled: pointer.BoolPtr(true),
	}
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

	controller := UserAttributeController{
		userLister: &mgmtFakes.UserListerMock{
			GetFunc: func(namespace, name string) (*v3.User, error) {
				return user, nil
			},
		},
		users: &mgmtFakes.UserInterfaceMock{
			UpdateFunc: func(user *v3.User) (*v3.User, error) {
				user = user.DeepCopy()
				return user, nil
			},
		},
		userAttributes: &mgmtFakes.UserAttributeInterfaceMock{
			GetFunc: func(name string, opts metav1.GetOptions) (*apiv3.UserAttribute, error) {
				userAttributesGetCalledTimes++
				return attribs.DeepCopy(), nil
			},
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				userAttributesUpdateCalledTimes++
				attribs = userAttribute.DeepCopy()
				return attribs, nil
			},
		},
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalledTimes++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals = map[string]apiv3.Principals{"activedirectory": {}}
			a.ExtraByProvider = map[string]map[string][]string{"activedirectory": {}}
			return a, nil
		},
	}

	obj, err := controller.sync("", attribs)
	assert.NoError(t, err)

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
	user := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
		Enabled: pointer.BoolPtr(true),
	}
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
		Resource: apiv3.UserAttributeResourceName,
	}

	now := time.Now().Truncate(time.Second)

	controller := UserAttributeController{
		userLister: &mgmtFakes.UserListerMock{
			GetFunc: func(namespace, name string) (*v3.User, error) {
				return user, nil
			},
		},
		users: &mgmtFakes.UserInterfaceMock{
			UpdateFunc: func(user *v3.User) (*v3.User, error) {
				user = user.DeepCopy()
				return user, nil
			},
		},
		userAttributes: &mgmtFakes.UserAttributeInterfaceMock{
			GetFunc: func(name string, opts metav1.GetOptions) (*apiv3.UserAttribute, error) {
				userAttributesGetCalledTimes++

				a := attribs.DeepCopy()
				if userAttributesGetCalledTimes > 1 {
					a.LastLogin = metav1.NewTime(now)
				}

				return a, nil
			},
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				userAttributesUpdateCalledTimes++

				if userAttributesUpdateCalledTimes == 1 {
					return nil, apierrors.NewConflict(groupResource, userAttribute.Name, fmt.Errorf("some error"))
				}

				attribs = userAttribute.DeepCopy()
				return attribs, nil
			},
		},
		providerRefresh: func(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
			providerRefreshCalledTimes++
			a := attribs.DeepCopy()
			a.NeedsRefresh = false
			a.LastRefresh = now.Format(time.RFC3339)
			a.GroupPrincipals = map[string]apiv3.Principals{"activedirectory": {}}
			a.ExtraByProvider = map[string]map[string][]string{"activedirectory": {}}
			return a, nil
		},
	}

	obj, err := controller.sync("", attribs)
	assert.NoError(t, err)

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
