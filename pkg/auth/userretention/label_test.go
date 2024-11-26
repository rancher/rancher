package userretention

import (
	"context"
	"fmt"
	"testing"
	"time"

	management "github.com/rancher/rancher/pkg/apis/management.cattle.io"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestEnsureForAttributes(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	userID := "u-abcdef"
	user := &v3.User{
		ObjectMeta: metav1.ObjectMeta{Name: userID},
	}
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: userID},
		LastLogin:  &metav1.Time{Time: now},
	}

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(2).DoAndReturn(func(name string) (*v3.User, error) {
		return user, nil
	})
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).Times(0)
	usersClient.EXPECT().Update(gomock.Any()).Times(2)

	disableAfter := time.Duration(time.Hour)
	deleteAfter := time.Duration(2 * time.Hour)
	labeler := &UserLabeler{
		ctx:       context.Background(),
		userCache: usersCacheClient,
		users:     usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: disableAfter,
				deleteAfter:  deleteAfter,
			}, nil
		},
	}

	err := labeler.EnsureForAttributes(attribs)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := toEpochTimeString(now), user.Labels[LastLoginLabelKey]; want != got {
		t.Errorf("Expected last-login label %s got %s", want, got)
	}
	if want, got := toEpochTimeString(now.Add(disableAfter)), user.Labels[DisableAfterLabelKey]; want != got {
		t.Errorf("Expected disable-after label %s got %s", want, got)
	}
	if want, got := toEpochTimeString(now.Add(deleteAfter)), user.Labels[DeleteAfterLabelKey]; want != got {
		t.Errorf("Expected delete-after label %s got %s", want, got)
	}

	// The default admin should only have last-login label.
	user.Username = "admin"

	err = labeler.EnsureForAttributes(attribs)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 1, len(user.Labels); want != got {
		t.Fatalf("Expected labels %d got %d", want, got)
	}

	if want, got := toEpochTimeString(now), user.Labels[LastLoginLabelKey]; want != got {
		t.Errorf("Expected last-login label %s got %s", want, got)
	}
}

func TestEnsureForAttributesZeroLastLogin(t *testing.T) {
	userID := "u-abcdef"
	user := &v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
	}
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
	}

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(2).DoAndReturn(func(name string) (*v3.User, error) {
		return user, nil
	})
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).Times(0)
	usersClient.EXPECT().Update(gomock.Any()).Times(1)

	disableAfter := time.Duration(time.Hour)
	deleteAfter := time.Duration(2 * time.Hour)
	labeler := &UserLabeler{
		ctx:       context.Background(),
		userCache: usersCacheClient,
		users:     usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: disableAfter,
				deleteAfter:  deleteAfter,
			}, nil
		},
	}

	// There should be no retention labels set.
	err := labeler.EnsureForAttributes(attribs)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 0, len(user.Labels); want != got {
		t.Errorf("Expected labels %d got %d", want, got)
	}

	// All existing retention labels should be removed.
	user.Labels = map[string]string{
		LastLoginLabelKey:    "some-time",
		DisableAfterLabelKey: "some-time",
		DeleteAfterLabelKey:  "some-time",
	}
	err = labeler.EnsureForAttributes(attribs)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 0, len(user.Labels); want != got {
		t.Errorf("Expected labels %d got %d", want, got)
	}
}

func TestEnsureForAttributesConflictOnUpdate(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	userID := "u-abcdef"
	user := &v3.User{
		ObjectMeta: metav1.ObjectMeta{Name: userID},
	}
	unmodifiedUser := user.DeepCopy()
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: userID},
		LastLogin:  &metav1.Time{Time: now},
	}

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(1).DoAndReturn(func(name string) (*v3.User, error) {
		return user, nil
	})
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(name string, options metav1.GetOptions) (*v3.User, error) {
		return unmodifiedUser, nil
	})
	var userUpdateTry int
	usersClient.EXPECT().Update(gomock.Any()).Times(2).DoAndReturn(func(user *v3.User) (*v3.User, error) {
		defer func() { userUpdateTry++ }()

		if userUpdateTry == 0 {
			return nil, apierrors.NewConflict(schema.GroupResource{
				Group:    management.GroupName,
				Resource: v3.UserResourceName,
			}, user.Name, fmt.Errorf("some error"))
		}

		user = user.DeepCopy()
		return user, nil
	})

	disableAfter := time.Duration(time.Hour)
	deleteAfter := time.Duration(2 * time.Hour)
	labeler := &UserLabeler{
		ctx:       context.Background(),
		userCache: usersCacheClient,
		users:     usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: disableAfter,
				deleteAfter:  deleteAfter,
			}, nil
		},
	}

	err := labeler.EnsureForAttributes(attribs)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := toEpochTimeString(now), user.Labels[LastLoginLabelKey]; want != got {
		t.Errorf("Expected last-login label %s got %s", want, got)
	}
	if want, got := toEpochTimeString(now.Add(disableAfter)), user.Labels[DisableAfterLabelKey]; want != got {
		t.Errorf("Expected disable-after label %s got %s", want, got)
	}
	if want, got := toEpochTimeString(now.Add(deleteAfter)), user.Labels[DeleteAfterLabelKey]; want != got {
		t.Errorf("Expected delete-after label %s got %s", want, got)
	}
}

func TestEnsureForAttributesErrorReadingSettings(t *testing.T) {
	userID := "u-abcdef"
	attribs := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
		},
	}

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(0)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).Times(0)
	usersClient.EXPECT().Update(gomock.Any()).Times(0)

	labeler := &UserLabeler{
		ctx:       context.Background(),
		userCache: usersCacheClient,
		users:     usersClient,
		readSettings: func() (settings, error) {
			return settings{}, fmt.Errorf("invalid settings")
		},
	}

	// There should be no error returned.
	err := labeler.EnsureForAttributes(attribs)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureForAll(t *testing.T) {
	override := 5 * time.Hour

	now := time.Now().Truncate(time.Second)
	users := []*v3.User{
		{ // Regular user with no overrides.
			ObjectMeta: metav1.ObjectMeta{Name: "u-abcdef"},
		},
		{ // Regular user with stale labels.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-bcdefa",
				Labels: map[string]string{
					LastLoginLabelKey:    toEpochTimeString(now.Add(-24 * time.Hour)),
					DisableAfterLabelKey: toEpochTimeString(now.Add(-24 * time.Hour)),
					DeleteAfterLabelKey:  toEpochTimeString(now.Add(-24 * time.Hour)),
				},
			},
		},
		{ // Regular user with overrides to increase retention period.
			ObjectMeta: metav1.ObjectMeta{Name: "u-cdefab"},
		},
		{ // Regular user with overrides that disables retention.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-defabc",
				Labels: map[string]string{ // Should be removed by the labeler.
					DisableAfterLabelKey: toEpochTimeString(now.Add(-24 * time.Hour)),
					DeleteAfterLabelKey:  toEpochTimeString(now.Add(-24 * time.Hour)),
				},
			},
		},
		{ // Default admin.
			ObjectMeta: metav1.ObjectMeta{Name: "user-phs88"},
			Username:   "admin",
		},
		{ // Regular user, no attributes.
			ObjectMeta: metav1.ObjectMeta{Name: "u-efabcd"},
		},
		{ // System user.
			ObjectMeta:   metav1.ObjectMeta{Name: "u-ixoqm74x7r"},
			PrincipalIDs: []string{"system://c-xpqsb"},
		},
		{ // Regular user, no LastLogin and previously set retention labels.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-fabcde",
				Labels: map[string]string{
					LastLoginLabelKey:    toEpochTimeString(now.Add(-24 * time.Hour)),
					DisableAfterLabelKey: toEpochTimeString(now.Add(-24 * time.Hour)),
					DeleteAfterLabelKey:  toEpochTimeString(now.Add(-24 * time.Hour)),
				},
			},
		},
	}
	attribs := []*v3.UserAttribute{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "u-abcdef"},
			LastLogin:  &metav1.Time{Time: now},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "u-bcdefa"},
			LastLogin:  &metav1.Time{Time: now},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-cdefab"},
			LastLogin:    &metav1.Time{Time: now},
			DisableAfter: &metav1.Duration{Duration: override},
			DeleteAfter:  &metav1.Duration{Duration: override},
		},
		{
			ObjectMeta:   metav1.ObjectMeta{Name: "u-defabc"},
			LastLogin:    &metav1.Time{Time: now},
			DisableAfter: &metav1.Duration{Duration: 0},
			DeleteAfter:  &metav1.Duration{Duration: 0},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "user-phs88"},
			LastLogin:  &metav1.Time{Time: now},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "u-fabcde"},
		},
	}

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(0)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(1).DoAndReturn(func(selector labels.Selector) ([]*v3.User, error) {
		return users, nil
	})

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Update(gomock.Any()).AnyTimes()

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		for _, attr := range attribs {
			if attr.Name == name {
				return attr.DeepCopy(), nil
			}
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})

	disableAfter := time.Duration(time.Hour)
	deleteAfter := time.Duration(2 * time.Hour)
	labeler := &UserLabeler{
		ctx:                context.Background(),
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: disableAfter,
				deleteAfter:  deleteAfter,
			}, nil
		},
	}

	err := labeler.EnsureForAll()
	if err != nil {
		t.Fatal(err)
	}

	var i int

	// The first two users should have identical labels.
	for i = 0; i < 2; i++ {
		if want, got := toEpochTimeString(now), users[i].Labels[LastLoginLabelKey]; want != got {
			t.Errorf("Expected last-login label %s got %s for user %s", want, got, users[i].Name)
		}
		if want, got := toEpochTimeString(now.Add(disableAfter)), users[i].Labels[DisableAfterLabelKey]; want != got {
			t.Errorf("Expected disable-after label %s got %s for user %s", want, got, users[i].Name)
		}
		if want, got := toEpochTimeString(now.Add(deleteAfter)), users[i].Labels[DeleteAfterLabelKey]; want != got {
			t.Errorf("Expected delete-after label %s got %s for user %s", want, got, users[i].Name)
		}
	}

	// The third should have increased retention period.
	i = 2
	if want, got := toEpochTimeString(now), users[2].Labels[LastLoginLabelKey]; want != got {
		t.Errorf("Expected last-login label %s got %s for user %s", want, got, users[i].Name)
	}
	if want, got := toEpochTimeString(now.Add(override)), users[2].Labels[DisableAfterLabelKey]; want != got {
		t.Errorf("Expected disable-after label %s got %s for user %s", want, got, users[i].Name)
	}
	if want, got := toEpochTimeString(now.Add(override)), users[2].Labels[DeleteAfterLabelKey]; want != got {
		t.Errorf("Expected delete-after label %s got %s for user %s", want, got, users[i].Name)
	}

	// The fourth and fifth should only have last-login label and no retention labels due to overrides.
	for i = 3; i < 5; i++ {
		if want, got := 1, len(users[i].Labels); want != got {
			t.Errorf("Expected labels %d got %d for user %s", want, got, users[i].Name)
		}
		if want, got := toEpochTimeString(now), users[i].Labels[LastLoginLabelKey]; want != got {
			t.Errorf("Expected last-login label %s got %s for user %s", want, got, users[i].Name)
		}
	}

	// The last three should have no labels.
	for i = 5; i < 8; i++ {
		if want, got := 0, len(users[i].Labels); want != got {
			t.Errorf("Expected labels %d got %d for user %s", want, got, users[i].Name)
		}
	}
}

func TestEnsureForAllContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context right away.

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(0)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Update(gomock.Any()).Times(0)

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).Times(0)

	labeler := &UserLabeler{
		ctx:                ctx,
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
	}

	err := labeler.EnsureForAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureForAllContextCancelledInFlight(t *testing.T) {
	users := []*v3.User{
		{ // Regular user with no overrides.
			ObjectMeta: metav1.ObjectMeta{Name: "u-abcdef"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(0)
	usersCacheClient.EXPECT().List(gomock.Any()).AnyTimes().DoAndReturn(func(selector labels.Selector) ([]*v3.User, error) {
		cancel() // Cancel the context before processing individual users.
		return users, nil
	})

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Update(gomock.Any()).Times(0)

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).Times(0)

	disableAfter := time.Duration(time.Hour)
	deleteAfter := time.Duration(2 * time.Hour)
	labeler := &UserLabeler{
		ctx:                ctx,
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: disableAfter,
				deleteAfter:  deleteAfter,
			}, nil
		},
	}

	err := labeler.EnsureForAll()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureForAllErrorReadingSettings(t *testing.T) {
	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().Get(gomock.Any()).Times(0)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Update(gomock.Any()).Times(0)

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).Times(0)

	labeler := &UserLabeler{
		ctx:                context.Background(),
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{}, fmt.Errorf("invalid settings")
		},
	}

	err := labeler.EnsureForAll()
	if err != nil {
		t.Fatal(err)
	}
}
