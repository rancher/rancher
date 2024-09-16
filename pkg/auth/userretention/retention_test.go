package userretention

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

func TestLastLoginTime(t *testing.T) {
	now := time.Now()
	zeroLastLogin := &metav1.Time{Time: time.Time{}}

	tests := []struct {
		desc             string
		lastLoginDefault time.Time
		lastLogin        *metav1.Time
		want             time.Time
	}{
		{
			desc: "missing last login",
		},
		{
			desc:      "zero last login",
			lastLogin: zeroLastLogin,
		},
		{
			desc:             "missing last login with default",
			lastLoginDefault: now,
			want:             now,
		},
		{
			desc:             "zero last login with default",
			lastLogin:        zeroLastLogin,
			lastLoginDefault: now,
			want:             now,
		},
		{
			desc:      "last login is set with no default",
			lastLogin: &metav1.Time{Time: now},
			want:      now,
		},
		{
			desc:             "last login is set with default",
			lastLogin:        &metav1.Time{Time: now},
			lastLoginDefault: now.Add(-time.Hour),
			want:             now,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			settings := settings{defaultLastLogin: test.lastLoginDefault}
			attribs := &v3.UserAttribute{LastLogin: test.lastLogin}
			if want, got := test.want, lastLoginTime(settings, attribs); want != got {
				t.Errorf("Expected last login time %v got %v", want, got)
			}
		})
	}
}

func TestRetentionIsDisabledByDefault(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Fatal("Unexpected panic")
		}
	}()

	r := Retention{
		readSettings: readSettings,
	}

	err := r.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestRetentionNormalRun(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	disableInactiveUserAfter := 55 * time.Minute
	deleteInactiveUserAfter := 115 * time.Minute
	lastLoginDefault := now.Add(-time.Hour)

	userAttributes := map[string]*v3.UserAttribute{
		"u-cx7gc": {
			// LastLogin is missing.
		},
		"u-ckrl4grxg5": {
			LastLogin: &metav1.Time{Time: now.Add(-time.Hour)},
		},
		"u-mo773yttt4": {
			LastLogin: &metav1.Time{Time: now.Add(-time.Hour)},
		},
		"u-yypnjwjmkq": {
			LastLogin: &metav1.Time{Time: now.Add(-2 * time.Hour)},
		},
		"u-evhs6gb54u": {
			LastLogin:    &metav1.Time{Time: now.Add(-2 * time.Hour)},
			DisableAfter: &metav1.Duration{Duration: 4 * time.Hour},
			DeleteAfter:  &metav1.Duration{Duration: 5 * time.Hour},
		},
		"u-f5ugvctlrk": {
			LastLogin:    &metav1.Time{Time: now.Add(-10 * time.Hour)},
			DisableAfter: &metav1.Duration{Duration: 0},
			DeleteAfter:  &metav1.Duration{Duration: 0},
		},
	}

	users := map[string]*v3.User{
		"user-phs88": { // Default admin.
			ObjectMeta: metav1.ObjectMeta{
				Name: "user-phs88",
			},
			PrincipalIDs: []string{"local://user-phs88", "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space"},
			Username:     "admin",
			Enabled:      pointer.Bool(true),
		},
		"u-cx7gc": { // Local user.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-cx7gc",
			},
			PrincipalIDs: []string{"local://u-cx7gc"},
			Enabled:      pointer.Bool(true),
		},
		"u-ckrl4grxg5": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-ckrl4grxg5",
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser2,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-ckrl4grxg5"},
			Enabled:      pointer.Bool(true),
		},
		"u-mo773yttt4": { // Already disabled.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-mo773yttt4",
				Labels: map[string]string{
					LastLoginLabelKey:    toEpochTimeString(userAttributes["u-mo773yttt4"].LastLogin.Time),
					DisableAfterLabelKey: toEpochTimeString(userAttributes["u-mo773yttt4"].LastLogin.Add(disableInactiveUserAfter)),
					DeleteAfterLabelKey:  toEpochTimeString(userAttributes["u-mo773yttt4"].LastLogin.Add(deleteInactiveUserAfter)),
				},
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser3,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-mo773yttt4"},
			Enabled:      pointer.Bool(false),
		},
		"u-yypnjwjmkq": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-yypnjwjmkq",
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-yypnjwjmkq"},
			Enabled:      pointer.Bool(true),
		},
		"u-evhs6gb54u": { // A user with disable and delete overrides.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-evhs6gb54u",
				Labels: map[string]string{
					LastLoginLabelKey:    toEpochTimeString(userAttributes["u-evhs6gb54u"].LastLogin.Time),
					DisableAfterLabelKey: toEpochTimeString(userAttributes["u-evhs6gb54u"].LastLogin.Add(userAttributes["u-evhs6gb54u"].DisableAfter.Duration)), // Should stay intact after the retention run.
					DeleteAfterLabelKey:  toEpochTimeString(userAttributes["u-evhs6gb54u"].LastLogin.Add(userAttributes["u-evhs6gb54u"].DeleteAfter.Duration)),  // Should stay intact after the retention run.
				},
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser5,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-evhs6gb54u"},
			Enabled:      pointer.Bool(true),
		},
		"u-f5ugvctlrk": { // A user that should be retained indefinitely.
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-f5ugvctlrk",
				Labels: map[string]string{
					DisableAfterLabelKey: toEpochTimeString(now), // Should be removed after the retention run.
					DeleteAfterLabelKey:  toEpochTimeString(now), // Should be removed after the retention run.
				},
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser6,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-f5ugvctlrk"},
			Enabled:      pointer.Bool(true),
		},
	}
	var (
		deleted []string
		updated []*v3.User
	)

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(1).DoAndReturn(func(selector labels.Selector) ([]*v3.User, error) {
		result := make([]*v3.User, 0, len(users))
		for _, user := range users {
			result = append(result, user.DeepCopy())
		}
		return result, nil
	})

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.User, error) {
		if user, ok := users[name]; ok {
			return user.DeepCopy(), nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})
	usersClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(user *v3.User) (*v3.User, error) {
		u := user.DeepCopy()
		updated = append(updated, u)
		return u, nil
	})
	usersClient.EXPECT().Delete(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, options *metav1.DeleteOptions) error {
		deleted = append(deleted, name)
		return nil
	})

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		if attr, ok := userAttributes[name]; ok {
			return attr, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})

	retention := Retention{
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter:     disableInactiveUserAfter,
				deleteAfter:      deleteInactiveUserAfter,
				defaultLastLogin: lastLoginDefault,
			}, nil
		},
	}

	err := retention.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(deleted)
	if want, got := []string{"u-yypnjwjmkq"}, deleted; !reflect.DeepEqual(want, got) {
		t.Errorf("Expected deleted\n%v\ngot\n%v", want, got)
	}

	if want, got := 3, len(updated); want != got {
		t.Errorf("Expected %d updated users got %d", want, got)
	}

	for _, user := range updated {
		switch user.Name {
		case "u-f5ugvctlrk":
			// Make sure retention labels were removed because of the overrides.
			if want, got := 1, len(user.Labels); want != got {
				t.Errorf("[user %s] Expected %d labels got %d", user.Name, want, got)
			}
			if want, got := strconv.FormatInt(userAttributes[user.Name].LastLogin.Unix(), 10), user.Labels[LastLoginLabelKey]; want != got {
				t.Errorf("[user %s] Expected last-login label %s got %s", user.Name, want, got)
			}
		case "u-cx7gc":
			if want, got := false, pointer.BoolDeref(user.Enabled, true); want != got {
				t.Errorf("[user %s] Expected enabled %t got %t", user.Name, want, got)
			}
			if want, got := 3, len(user.Labels); want != got {
				t.Fatalf("[user %s] Expected %d labels got %d", user.Name, want, got)
			}
			if want, got := toEpochTimeString(lastLoginDefault.Add(disableInactiveUserAfter)), user.Labels[DisableAfterLabelKey]; want != got {
				t.Errorf("[user %s] Expected disable-after label %s got %s", user.Name, want, got)
			}
			if want, got := toEpochTimeString(lastLoginDefault.Add(deleteInactiveUserAfter)), user.Labels[DeleteAfterLabelKey]; want != got {
				t.Errorf("[user %s] Expected delete-after label %s got %s", user.Name, want, got)
			}
			if want, got := toEpochTimeString(lastLoginDefault), user.Labels[LastLoginLabelKey]; want != got {
				t.Errorf("[user %s] Expected last-login label %s got %s", user.Name, want, got)
			}
		case "u-ckrl4grxg5":
			if want, got := false, pointer.BoolDeref(user.Enabled, true); want != got {
				t.Errorf("[user %s] Expected enabled %t got %t", user.Name, want, got)
			}
			if want, got := 3, len(user.Labels); want != got {
				t.Fatalf("[user %s] Expected %d labels got %d", user.Name, want, got)
			}
			if want, got := toEpochTimeString(now.Add(-time.Hour+disableInactiveUserAfter)), user.Labels[DisableAfterLabelKey]; want != got {
				t.Errorf("[user %s] Expected disable-after label %s got %s", user.Name, want, got)
			}
			if want, got := toEpochTimeString(now.Add(-time.Hour+deleteInactiveUserAfter)), user.Labels[DeleteAfterLabelKey]; want != got {
				t.Errorf("[user %s] Expected delete-after label %s got %s", user.Name, want, got)
			}
			if want, got := toEpochTimeString(userAttributes[user.Name].LastLogin.Time), user.Labels[LastLoginLabelKey]; want != got {
				t.Errorf("[user %s] Expected last-login label %s got %s", user.Name, want, got)
			}
		default:
			t.Errorf("[user %s] Unexpected update", user.Name)
		}
	}
}

func TestRetentionDryRun(t *testing.T) {
	users := map[string]*v3.User{
		"u-ckrl4grxg5": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-ckrl4grxg5",
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser2,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-ckrl4grxg5"},
			Enabled:      pointer.Bool(true),
		},
		"u-mo773yttt4": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-mo773yttt4",
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser3,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-mo773yttt4"},
			Enabled:      pointer.Bool(true),
		},
	}
	userAttributes := map[string]*v3.UserAttribute{
		"u-ckrl4grxg5": {
			LastLogin: &metav1.Time{Time: time.Now().Add(-time.Hour)},
		},
		"u-mo773yttt4": {
			LastLogin: &metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
		},
	}

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(1).DoAndReturn(func(selector labels.Selector) ([]*v3.User, error) {
		result := make([]*v3.User, 0, len(users))
		for _, user := range users {
			result = append(result, user)
		}
		return result, nil
	})

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.User, error) {
		if user, ok := users[name]; ok {
			return user, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})
	usersClient.EXPECT().Update(gomock.Any()).Times(0)
	usersClient.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		if attr, ok := userAttributes[name]; ok {
			return attr, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})

	retention := Retention{
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: 55 * time.Minute,
				deleteAfter:  115 * time.Minute,
				dryRun:       true,
			}, nil
		},
	}

	err := retention.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for id, user := range users {
		if want, got := true, pointer.BoolDeref(user.Enabled, false); want != got {
			t.Errorf("Expected Enabled for user %s %t got %t", id, want, got)
		}
		if want, got := 0, len(user.Labels); want != got {
			t.Errorf("Expected labels for user %s %d got %d", id, want, got)
		}
	}
}

func TestRetentionRunNoLastLoginDefault(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	disableInactiveUserAfter := 55 * time.Minute
	deleteInactiveUserAfter := 115 * time.Minute

	userAttributes := map[string]*v3.UserAttribute{
		"u-cx7gc":      {},
		"u-ckrl4grxg5": {},
	}

	users := map[string]*v3.User{
		"u-cx7gc": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-cx7gc",
			},
			PrincipalIDs: []string{"local://u-cx7gc"},
		},
		"u-ckrl4grxg5": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-ckrl4grxg5",
				Labels: map[string]string{
					LastLoginLabelKey:    toEpochTimeString(now),
					DisableAfterLabelKey: toEpochTimeString(now.Add(disableInactiveUserAfter)),
					DeleteAfterLabelKey:  toEpochTimeString(now.Add(deleteInactiveUserAfter)),
				},
			},
			PrincipalIDs: []string{"activedirectory_user://CN=testuser2,CN=Users,DC=qa,DC=rancher,DC=space", "local://u-ckrl4grxg5"},
		},
	}
	var (
		deleted []string
		updated []*v3.User
	)

	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(1).DoAndReturn(func(selector labels.Selector) ([]*v3.User, error) {
		result := make([]*v3.User, 0, len(users))
		for _, user := range users {
			result = append(result, user.DeepCopy())
		}
		return result, nil
	})

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, opts metav1.GetOptions) (*v3.User, error) {
		if user, ok := users[name]; ok {
			return user.DeepCopy(), nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})
	usersClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(user *v3.User) (*v3.User, error) {
		u := user.DeepCopy()
		updated = append(updated, u)
		return u, nil
	})
	usersClient.EXPECT().Delete(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(name string, options *metav1.DeleteOptions) error {
		deleted = append(deleted, name)
		return nil
	})

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		if attr, ok := userAttributes[name]; ok {
			return attr, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
	})

	retention := Retention{
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: disableInactiveUserAfter,
				deleteAfter:  deleteInactiveUserAfter,
			}, nil
		},
	}

	err := retention.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if want, got := 0, len(deleted); want != got {
		t.Errorf("Expected %d deleted users got %d", want, got)
	}

	if want, got := 1, len(updated); want != got {
		t.Errorf("Expected %d updated users got %d", want, got)
	}

	for _, user := range updated {
		switch user.Name {
		case "u-cx7gc", "u-ckrl4grxg5":
			if want, got := 0, len(user.Labels); want != got {
				t.Fatalf("[user %s] Expected %d labels got %d", user.Name, want, got)
			}
		default:
			t.Errorf("[user %s] Unexpected update", user.Name)
		}
	}
}

func TestRetentionRunErrorReadingSettings(t *testing.T) {
	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).Times(0)
	usersClient.EXPECT().Update(gomock.Any()).Times(0)
	usersClient.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).Times(0)

	readSettingsErr := fmt.Errorf("error reading settings")
	retention := Retention{
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{}, readSettingsErr
		},
	}

	err := retention.Run(context.Background())
	if err == nil {
		t.Fatal("Expected error got nil")
	}

	if !strings.Contains(err.Error(), readSettingsErr.Error()) {
		t.Error("Unexpected error message")
	}
}

func TestRetentionRunContextCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)

	usersCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	usersCacheClient.EXPECT().List(gomock.Any()).Times(0)

	usersClient := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
	usersClient.EXPECT().Get(gomock.Any(), gomock.Any()).Times(0)
	usersClient.EXPECT().Update(gomock.Any()).Times(0)
	usersClient.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	userAttributeCacheClient := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCacheClient.EXPECT().Get(gomock.Any()).Times(0)

	retention := Retention{
		userAttributeCache: userAttributeCacheClient,
		userCache:          usersCacheClient,
		users:              usersClient,
		readSettings: func() (settings, error) {
			return settings{
				disableAfter: 55 * time.Minute,
				deleteAfter:  115 * time.Minute,
			}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := retention.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
}
