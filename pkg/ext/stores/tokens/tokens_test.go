package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"
)

var (
	// enabledUser
	enabledUser = &v3.User{
		Enabled: pointer.Bool(true),
	}
	// use for disabled tokens

	// properSecret is the backend secret matching the properToken
	properPrincipal = ext.TokenPrincipal{
		Name:        "world",
		Provider:    "somebody",
		DisplayName: "myself",
		LoginName:   "hello",
	}
	properPrincipalBytes, _ = json.Marshal(properPrincipal)
	properSecret            = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bogus",
		},
		Data: map[string][]byte{
			FieldDescription:    []byte(""),
			FieldEnabled:        []byte("false"),
			FieldHash:           []byte("kla9jkdmj"),
			FieldKind:           []byte(IsLogin),
			FieldLastUpdateTime: []byte("13:00:05"),
			FieldPrincipal:      properPrincipalBytes,
			FieldTTL:            []byte("4000"),
			FieldUID:            []byte("2905498-kafld-lkad"),
			FieldUserID:         []byte("lkajdlksjlkds"),
		},
	}
	// missing user-id - for list tests
	badSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bogus",
		},
		Data: map[string][]byte{
			FieldDescription:    []byte(""),
			FieldEnabled:        []byte("false"),
			FieldHash:           []byte("kla9jkdmj"),
			FieldKind:           []byte(IsLogin),
			FieldLastUpdateTime: []byte("13:00:05"),
			FieldPrincipal:      properPrincipalBytes,
			FieldTTL:            []byte("4000"),
			FieldUID:            []byte("2905498-kafld-lkad"),
		},
	}
	// properToken is the token matching what is stored in the properSecret
	properToken = ext.Token{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Token",
			APIVersion: "ext.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bogus",
			UID:  types.UID("2905498-kafld-lkad"),
		},
		Spec: ext.TokenSpec{
			UserID:        "lkajdlksjlkds",
			Description:   "",
			TTL:           4000,
			Enabled:       pointer.Bool(false),
			Kind:          "session",
			UserPrincipal: properPrincipal,
		},
		Status: ext.TokenStatus{
			Value:          "",
			Hash:           "kla9jkdmj",
			Expired:        true,
			ExpiresAt:      "0001-01-01T00:00:04Z",
			LastUpdateTime: "13:00:05",
		},
	}

	// Note: Setup is done in `init()` below.
	properTokenCurrent ext.Token

	dummyToken = ext.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bogus",
		},
	}

	// ttlPlusSecret is the properSecret with extended ttl
	ttlPlusSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bogus",
		},
		Data: map[string][]byte{
			FieldEnabled:        []byte("false"),
			FieldHash:           []byte("kla9jkdmj"),
			FieldKind:           []byte(IsLogin),
			FieldLastUpdateTime: []byte("this is a fake now"),
			FieldPrincipal:      properPrincipalBytes,
			FieldTTL:            []byte("5000"),
			FieldUID:            []byte("2905498-kafld-lkad"),
			FieldUserID:         []byte("lkajdlksjlkds"),
		},
	}
	// ttlSubSecret is the properSecret with reduced ttl
	ttlSubSecret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bogus",
		},
		Data: map[string][]byte{
			FieldEnabled:        []byte("false"),
			FieldHash:           []byte("kla9jkdmj"),
			FieldKind:           []byte(IsLogin),
			FieldLastUpdateTime: []byte("this is a fake now"),
			FieldPrincipal:      properPrincipalBytes,
			FieldTTL:            []byte("3000"),
			FieldUID:            []byte("2905498-kafld-lkad"),
			FieldUserID:         []byte("lkajdlksjlkds"),
		},
	}

	someerror                = fmt.Errorf("bogus")
	authProviderMissingError = fmt.Errorf("auth provider missing")
	hashMissingError         = fmt.Errorf("token hash missing")
	kubeIDMissingError       = fmt.Errorf("kube uid missing")
	lastUpdateMissingError   = fmt.Errorf("last update time missing")
	principalIDMissingError  = fmt.Errorf("principal id missing")
	userIDMissingError       = fmt.Errorf("user id missing")
	invalidContext           = fmt.Errorf("context has no user info")

	bogusNotFoundError       = apierrors.NewNotFound(GVR.GroupResource(), "bogus")
	emptyNotFoundError       = apierrors.NewNotFound(GVR.GroupResource(), "")
	createUserMismatch       = apierrors.NewBadRequest("unable to create token for other user")
	helloAlreadyExistsError  = apierrors.NewAlreadyExists(GVR.GroupResource(), "hello")
	invalidNameError         = apierrors.NewBadRequest("Token is invalid: metadata.name: Locked by system. Do not set.")
	invalidGenerateNameError = apierrors.NewBadRequest("Token is invalid: metadata.generateName: Locked by system. Do not set.")

	parseBoolError error
	parseIntError  error
)

func init() {
	_, parseBoolError = strconv.ParseBool("")
	_, parseIntError = strconv.ParseInt("", 10, 64)

	properTokenCurrent = properToken
	properTokenCurrent.Status.Current = true
}

func Test_ttlGreater(t *testing.T) {

	tests := []struct {
		name string
		a    int64
		b    int64
		want bool
	}{
		{
			name: "infinities",
			a:    -1,
			b:    -2,
			want: false,
		},
		{
			name: "left infinite",
			a:    -3,
			b:    40,
			want: true,
		},
		{
			name: "right infinite",
			a:    40,
			b:    -2,
			want: false,
		},
		{
			name: "plain left > right",
			a:    10,
			b:    1,
			want: true,
		},
		{
			name: "plain left <= right",
			a:    10,
			b:    11,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			have := ttlGreater(test.a, test.b)
			assert.Equal(t, have, test.want)
		})
	}
}

func Test_Store_New(t *testing.T) {
	t.Parallel()

	store := &Store{}
	obj := store.New()
	require.NotNil(t, obj)
	assert.IsType(t, &ext.Token{}, obj)
}

func Test_Store_NewList(t *testing.T) {
	store := &Store{}
	obj := store.NewList()
	require.NotNil(t, obj)
	require.IsType(t, &ext.TokenList{}, obj)

	list := obj.(*ext.TokenList)
	assert.Nil(t, list.Items)
}

func Test_Store_GetSingularName(t *testing.T) {
	store := &Store{}
	assert.Equal(t, SingularName, store.GetSingularName())
}

func Test_Store_NamespaceScoped(t *testing.T) {
	store := &Store{}
	assert.False(t, store.NamespaceScoped())
}

func Test_Store_GroupVersionKind(t *testing.T) {
	store := &Store{}
	assert.Equal(t, GVK, store.GroupVersionKind(ext.SchemeGroupVersion))
}

func Test_Store_Delete(t *testing.T) {
	// The majority of the code is tested later, in Test_SystemStore_Delete
	// Here we test the actions and checks done before delegation to the
	// embedded system store
	t.Run("failed to get secret, arbitrary error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "laber"}, false, true, nil)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(nil, someerror)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		_, ok, err := store.Delete(context.TODO(), "bogus", nil, &metav1.DeleteOptions{})

		assert.False(t, ok)
		assert.Equal(t, apierrors.NewInternalError(fmt.Errorf("failed to retrieve token bogus: %w",
			someerror)), err)
	})

	t.Run("failed to get secret, not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "laber"}, false, true, nil)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(nil, apierrors.NewNotFound(GVR.GroupResource(), "bogus"))

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		_, ok, err := store.Delete(context.TODO(), "bogus", nil, &metav1.DeleteOptions{})

		assert.False(t, ok)
		assert.Equal(t, apierrors.NewNotFound(GVR.GroupResource(), "bogus"), err)
	})

	t.Run("user info missing from context", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: ""}, false, false, apierrors.NewInternalError(invalidContext))
		secrets.EXPECT().Cache().Return(nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		_, ok, err := store.Delete(context.TODO(), "bogus", nil, &metav1.DeleteOptions{})

		assert.False(t, ok)
		assert.Equal(t, apierrors.NewInternalError(invalidContext), err)
	})

	t.Run("not owned, no permission, not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdl/ksjlkds"}, false, true, nil)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(&properSecret, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		_, ok, err := store.Delete(context.TODO(), "bogus", nil, &metav1.DeleteOptions{})

		assert.False(t, ok)
		assert.Equal(t, bogusNotFoundError, err)
	})

	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil).Times(2)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(&properSecret, nil)
		secrets.EXPECT().
			Delete("cattle-tokens", "bogus", gomock.Any()).
			Return(nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		_, ok, err := store.Delete(context.TODO(), "bogus", nil, &metav1.DeleteOptions{})

		assert.True(t, ok)
		assert.Nil(t, err)
	})
}

func Test_Store_Get(t *testing.T) {
	// The majority of the code is tested later, in Test_SystemStore_Get
	// Here we only test the permission checks done after delegation to the
	// embedded system store
	t.Run("not owned, no permission, not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdl/ksjlkds"}, false, true, nil)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(&properSecret, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		tok, err := store.Get(context.TODO(), "bogus", &metav1.GetOptions{})

		assert.Equal(t, bogusNotFoundError, err)
		assert.Nil(t, tok)
	})

	t.Run("ok, not current", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(&properSecret, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		tok, err := store.Get(context.TODO(), "bogus", &metav1.GetOptions{})

		assert.Nil(t, err)
		assert.Equal(t, &properToken, tok)
	})

	t.Run("ok, current", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		auth.EXPECT().SessionID(gomock.Any()).Return("bogus")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)
		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().
			Get("cattle-tokens", "bogus").
			Return(&properSecret, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		tok, err := store.Get(context.TODO(), "bogus", &metav1.GetOptions{})

		assert.Nil(t, err)
		assert.Equal(t, &properTokenCurrent, tok)
	})
}

func Test_Store_Watch(t *testing.T) {
	// This test suite is a bit special, as it is not table driven like all other suites coming
	// after it.  This is done because we need stronger control about the environment the
	// various store calls are in, i.e. the channels involved, the context, and the goroutine
	// internal to `Watch`.

	t.Run("backend watch creation error fails early", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(nil, someerror)

		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		todo, cancel := context.WithCancel(context.TODO())
		defer cancel()

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(todo, &metav1.ListOptions{})
		assert.Error(t, err)
		assert.Nil(t, consumer)
	})

	t.Run("context cancellation does not close watch channel", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		watcher := NewWatcherFor(watch.Event{Object: &properSecret, Type: watch.Bookmark})
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		todo, cancel := context.WithCancel(context.TODO())

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(todo, &metav1.ListOptions{})
		assert.Nil(t, err)

		// receive bookmark event
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		cancel()
		consumer.Stop()
	})

	t.Run("closing backend channel does not close watch channel", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		watcher := NewWatcherFor(watch.Event{Object: &properSecret, Type: watch.Bookmark})
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		watcher.Done() // close backend channel

		// still receive bookmark event
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		consumer.Stop()
	})

	t.Run("event for non-secret is ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		// bad event to ignore, plus bookmark to see
		watcher := NewWatcherFor(
			watch.Event{Object: &corev1.Namespace{}},
			watch.Event{Object: &properSecret, Type: watch.Bookmark},
		)
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		// receive bookmark event, preceding bad event is swallowed
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		watcher.Done() // close backend channel - no further events
		consumer.Stop()
	})

	t.Run("event for broken secret is ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		// bad event to ignore, plus bookmark to see
		watcher := NewWatcherFor(
			watch.Event{Object: &corev1.Secret{}},
			watch.Event{Object: &properSecret, Type: watch.Bookmark},
		)
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		// receive bookmark event, preceding bad event is swallowed
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		watcher.Done() // close backend channel - no further events
		consumer.Stop()
	})

	t.Run("no events for non-owned secret", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		// return fake bookmark for easy channel management
		watcher := NewWatcherFor(watch.Event{Object: &properSecret, Type: watch.Bookmark})
		// Expect a watch() call with filter for user
		secrets.EXPECT().Watch("cattle-tokens", metav1.ListOptions{
			LabelSelector: UserIDLabel + "=lkajdl/ksjlkds",
		}).Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdl/ksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		// receive fake bookmark event for easy channel management
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		watcher.Done() // close backend channel
		consumer.Stop()
	})

	t.Run("receive event for owned secret, not current", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		watcher := NewWatcherFor(watch.Event{Object: &properSecret, Type: watch.Modified})
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		event, more := (<-consumer.ResultChan()) // receive update event
		assert.True(t, more)
		assert.Equal(t, watch.Modified, event.Type)
		assert.Equal(t, &properToken, event.Object)

		watcher.Done() // close backend channel
		consumer.Stop()
	})

	t.Run("receive event for owned secret, current", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		watcher := NewWatcherFor(watch.Event{Object: &properSecret, Type: watch.Modified})
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("bogus")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		event, more := (<-consumer.ResultChan()) // receive update event
		assert.True(t, more)
		assert.Equal(t, watch.Modified, event.Type)
		assert.Equal(t, &properTokenCurrent, event.Object)

		watcher.Done() // close backend channel
		consumer.Stop()
	})

	t.Run("event for error is ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		// bad event to ignore, plus bookmark to see
		watcher := NewWatcherFor(
			watch.Event{Object: &corev1.Namespace{}, Type: watch.Error},
			watch.Event{Object: &properSecret, Type: watch.Bookmark},
		)
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		// receive bookmark event, preceding bad event is swallowed
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		watcher.Done() // close backend channel - no further events
		consumer.Stop()
	})

	t.Run("event for bad bookmark is ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		// bad event to ignore, plus bookmark to see
		watcher := NewWatcherFor(
			watch.Event{Object: &corev1.Namespace{}, Type: watch.Bookmark},
			watch.Event{Object: &properSecret, Type: watch.Bookmark},
		)
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		// receive bookmark event, preceding bad event is swallowed
		event, more := (<-consumer.ResultChan())
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		watcher.Done() // close backend channel - no further events
		consumer.Stop()
	})

	t.Run("receive event for bookmark", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
		auth := NewMockauthHandler(ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		watcher := NewWatcherFor(watch.Event{Object: &properSecret, Type: watch.Bookmark})
		secrets.EXPECT().Watch("cattle-tokens", gomock.Any()).
			Return(watcher, nil)

		auth.EXPECT().SessionID(gomock.Any()).Return("")
		auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)

		store := New(nil, nil, nil, secrets, users, nil, nil, nil, nil, auth)
		consumer, err := store.watch(context.TODO(), &metav1.ListOptions{})
		assert.Nil(t, err)

		event, more := (<-consumer.ResultChan()) // receive bookmark event
		assert.True(t, more)
		assert.Equal(t, watch.Bookmark, event.Type)
		assert.Equal(t, &ext.Token{ObjectMeta: metav1.ObjectMeta{ResourceVersion: ""}}, event.Object)

		watcher.Done() // close backend channel
		consumer.Stop()
	})
}

func Test_Store_Create(t *testing.T) {
	tests := []struct {
		name       string                // test name
		err        error                 // expected op result, error
		tok        *ext.Token            // token input
		rtok       *ext.Token            // expected op result, created token
		opts       *metav1.CreateOptions // create options
		storeSetup func(                 // configure store backend clients
			space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			scache *fake.MockCacheInterface[*corev1.Secret],
			users *fake.MockNonNamespacedCacheInterface[*v3.User],
			token *fake.MockNonNamespacedCacheInterface[*v3.Token],
			cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
			timer *MocktimeHandler,
			hasher *MockhashHandler,
			auth *MockauthHandler)
	}{
		{
			name: "user name mismatch",
			err:  createUserMismatch,
			tok:  &ext.Token{Spec: ext.TokenSpec{UserID: "other"}},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "lkajdlksjlkds"}, false, true, nil)
			},
		},
		// token generation and hash errors -- no mocking -- unable to induce and test
		{
			name: "user retrieval error",
			err:  apierrors.NewInternalError(fmt.Errorf("failed to retrieve user world: %w", someerror)),
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				users.EXPECT().Get("world").
					Return(nil, someerror)
			},
		},
		{
			name: "user disabled",
			err:  apierrors.NewBadRequest("operation references a disabled user"),
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				users.EXPECT().Get("world").
					Return(&v3.User{
						Enabled: pointer.Bool(false),
					}, nil)
			},
		},
		{
			name: "provider/principal retrieval error",
			err:  apierrors.NewInternalError(fmt.Errorf("unable to fetch unknown token session-token")),
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				// fail fetch of session token, v3 and ext
				auth.EXPECT().SessionID(gomock.Any()).
					Return("session-token")
				token.EXPECT().Get("session-token").
					Return(nil, someerror)
				scache.EXPECT().Get("cattle-tokens", "session-token").
					Return(nil, someerror)

				users.EXPECT().Get("world").
					Return(enabledUser, nil)
			},
		},
		{
			name: "generation or hash error",
			err:  someerror,
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				// session token fetch for user principal
				auth.EXPECT().SessionID(gomock.Any()).
					Return("session-token")
				token.EXPECT().Get("session-token").Return(&v3.Token{
					AuthProvider: "local",
					UserPrincipal: v3.Principal{
						ObjectMeta: metav1.ObjectMeta{Name: "local://world"},
					}}, nil)

				users.EXPECT().Get("world").
					Return(enabledUser, nil)

				hasher.EXPECT().MakeAndHashSecret().
					Return("", "", someerror)
			},
		},
		{
			name: "failed to create secret - some error",
			err:  apierrors.NewInternalError(fmt.Errorf("failed to store token: %w", someerror)),
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				// session token fetch for user principal
				auth.EXPECT().SessionID(gomock.Any()).
					Return("session-token")
				token.EXPECT().Get("session-token").Return(&v3.Token{
					AuthProvider: "local",
					UserPrincipal: v3.Principal{
						ObjectMeta: metav1.ObjectMeta{Name: "local://world"},
					}}, nil)

				users.EXPECT().Get("world").
					Return(&v3.User{
						DisplayName: "worldwide",
						Username:    "wide",
						Enabled:     pointer.Bool(true),
					}, nil)

				// Fake value and hash
				hasher.EXPECT().MakeAndHashSecret().Return("", "", nil)

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				secrets.EXPECT().Create(gomock.Any()).
					Return(nil, someerror)
			},
		},
		{
			name: "failed to create secret - already exists",
			err:  helloAlreadyExistsError,
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				// session token fetch for user principal
				auth.EXPECT().SessionID(gomock.Any()).
					Return("session-token")
				token.EXPECT().Get("session-token").Return(&v3.Token{
					AuthProvider: "local",
					UserPrincipal: v3.Principal{
						ObjectMeta: metav1.ObjectMeta{Name: "local://world"},
					}}, nil)

				users.EXPECT().Get("world").
					Return(&v3.User{
						DisplayName: "worldwide",
						Username:    "wide",
						Enabled:     pointer.Bool(true),
					}, nil)

				// Fake value and hash
				hasher.EXPECT().MakeAndHashSecret().Return("", "", nil)

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				secrets.EXPECT().Create(gomock.Any()).
					Return(nil, helloAlreadyExistsError)
			},
		},
		{
			name: "created secret reads back as bogus",
			err:  apierrors.NewInternalError(fmt.Errorf("failed to regenerate token bogus: %w", userIDMissingError)),
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				// session token fetch for user principal
				auth.EXPECT().SessionID(gomock.Any()).
					Return("session-token")
				token.EXPECT().Get("session-token").Return(&v3.Token{
					AuthProvider: "local",
					UserPrincipal: v3.Principal{
						ObjectMeta: metav1.ObjectMeta{Name: "local://world"},
					}}, nil)

				users.EXPECT().Get("world").
					Return(&v3.User{
						DisplayName: "worldwide",
						Username:    "wide",
						Enabled:     pointer.Bool(true),
					}, nil)

				// Fake value and hash
				hasher.EXPECT().MakeAndHashSecret().Return("", "", nil)

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// note: returning an arbitrary bad secret here.
				// no connection to the token spec which went into create
				secrets.EXPECT().Create(gomock.Any()).
					Return(&badSecret, nil)

				// on failure to read back the secret is deleted again
				secrets.EXPECT().
					Delete("cattle-tokens", "bogus", gomock.Any()).
					Return(nil)

			},
		},
		{
			name: "created secret ok",
			err:  nil,
			tok: &ext.Token{
				Spec: ext.TokenSpec{
					UserID: "world",
				},
			},
			opts: &metav1.CreateOptions{},
			storeSetup: func( // configure store backend clients
				space *fake.MockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList],
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				users *fake.MockNonNamespacedCacheInterface[*v3.User],
				token *fake.MockNonNamespacedCacheInterface[*v3.Token],
				cluster *fake.MockNonNamespacedCacheInterface[*v3.Cluster],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				auth.EXPECT().UserName(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&mockUser{name: "world"}, false, true, nil)

				// session token fetch for user principal
				auth.EXPECT().SessionID(gomock.Any()).
					Return("session-token")
				token.EXPECT().Get("session-token").Return(&v3.Token{
					AuthProvider: "local",
					UserPrincipal: v3.Principal{
						ObjectMeta: metav1.ObjectMeta{Name: "local://world"},
					}}, nil)

				users.EXPECT().Get("world").
					Return(&v3.User{
						DisplayName: "worldwide",
						Username:    "wide",
						Enabled:     pointer.Bool(true),
					}, nil)

				// Fake value and hash -- See rtok below
				hasher.EXPECT().MakeAndHashSecret().Return("94084kdlafj43", "", nil)

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// note: returning an arbitrary good secret here.
				// no connection to the token spec which went into create
				secrets.EXPECT().Create(gomock.Any()).
					Return(&properSecret, nil)
			},
			rtok: func() *ext.Token {
				copy := properToken.DeepCopy()
				copy.Status.Hash = ""
				copy.Status.Value = "94084kdlafj43"
				return copy
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// assemble and configure a store from mock clients ...
			nsCache := fake.NewMockNonNamespacedCacheInterface[*corev1.Namespace](ctrl)
			nsCache.EXPECT().Get(TokenNamespace).AnyTimes()

			scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			ucache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
			tcache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
			ccache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
			timer := NewMocktimeHandler(ctrl)
			hasher := NewMockhashHandler(ctrl)
			auth := NewMockauthHandler(ctrl)

			users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			users.EXPECT().Cache().Return(ucache)

			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secrets.EXPECT().Cache().Return(scache)

			store := New(nil, nil, nsCache, secrets, users, tcache, ccache, timer, hasher, auth)
			test.storeSetup(nil, secrets, scache, ucache, tcache, ccache, timer, hasher, auth)

			// perform test and validate results
			tok, err := store.create(context.TODO(), test.tok, test.opts)
			if test.err != nil {
				assert.Equal(t, test.err, err)
				assert.Nil(t, tok)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.rtok, tok)
			}
		})
	}
}

func Test_SystemStore_List(t *testing.T) {
	tests := []struct {
		name       string              // test name
		user       string              // user making request
		isadmin    bool                // flag, user is admin
		session    string              // name of authenticating token
		opts       *metav1.ListOptions // list options
		err        error               // expected op result, error
		toks       *ext.TokenList      // expected op result, token list
		storeSetup func(               // configure store backend clients
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList])
	}{
		{
			name: "some arbitrary error",
			user: "",
			opts: &metav1.ListOptions{},
			err:  apierrors.NewInternalError(fmt.Errorf("failed to list tokens: %w", someerror)),
			toks: nil,
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return(nil, someerror)
			},
		},
		{
			name: "ok, empty",
			user: "",
			opts: &metav1.ListOptions{},
			err:  nil,
			toks: &ext.TokenList{Items: []ext.Token{}},
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return(&corev1.SecretList{}, nil)
			},
		},
		{
			name: "ok, not empty, not current",
			user: "lkajdlksjlkds",
			opts: &metav1.ListOptions{},
			err:  nil,
			toks: &ext.TokenList{
				ListMeta: metav1.ListMeta{
					ResourceVersion:    "1",
					Continue:           "true",
					RemainingItemCount: pointer.Int64(2),
				},
				Items: []ext.Token{
					properToken,
				},
			},
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return(&corev1.SecretList{
						ListMeta: metav1.ListMeta{
							ResourceVersion:    "1",
							Continue:           "true",
							RemainingItemCount: pointer.Int64(2),
						},
						Items: []corev1.Secret{
							properSecret,
						},
					}, nil)
			},
		},
		{
			name:    "ok, not empty, current",
			user:    "lkajdlksjlkds",
			session: "bogus",
			opts:    &metav1.ListOptions{},
			err:     nil,
			toks: &ext.TokenList{
				ListMeta: metav1.ListMeta{
					ResourceVersion:    "",
					Continue:           "",
					RemainingItemCount: nil,
				},
				Items: []ext.Token{
					properTokenCurrent,
				},
			},
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return(&corev1.SecretList{
						Items: []corev1.Secret{
							properSecret,
						},
					}, nil)
			},
		},
		{
			name: "ok, ignore broken secrets",
			user: "lkajdlksjlkds",
			opts: &metav1.ListOptions{},
			err:  nil,
			toks: &ext.TokenList{
				ListMeta: metav1.ListMeta{
					ResourceVersion:    "",
					Continue:           "",
					RemainingItemCount: nil,
				},
				Items: []ext.Token{
					properToken,
				},
			},
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					List("cattle-tokens", gomock.Any()).
					Return(&corev1.SecretList{
						Items: []corev1.Secret{
							properSecret,
							badSecret,
						},
					}, nil)
			},
		},
		{
			name: "ok, ignore non-owned results",
			user: "other",
			opts: &metav1.ListOptions{},
			err:  nil,
			toks: &ext.TokenList{Items: []ext.Token{}},
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					List("cattle-tokens", metav1.ListOptions{
						LabelSelector: UserIDLabel + "=other",
					}).Return(&corev1.SecretList{Items: []corev1.Secret{}}, nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// assemble and configure store from mock clients ...
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(nil)

			store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)
			test.storeSetup(secrets)

			// perform test and validate results
			toks, err := store.list(test.isadmin, test.user, test.session, test.opts)
			if test.err != nil {
				assert.Equal(t, test.err, err)
				assert.Nil(t, toks)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.toks, toks)
			}
		})
	}
}

func Test_SystemStore_Delete(t *testing.T) {
	tests := []struct {
		name       string                // test name
		token      string                // name of token to delete
		opts       *metav1.DeleteOptions // delete options
		err        error                 // expected op result, error
		storeSetup func(                 // configure store backend clients
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList])
	}{
		{
			name:  "secret not found is ok",
			token: "bogus",
			opts:  &metav1.DeleteOptions{},
			err:   nil,
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					Delete("cattle-tokens", "bogus", gomock.Any()).
					Return(emptyNotFoundError)

			},
		},
		{
			name:  "secret other error is fail",
			token: "bogus",
			opts:  &metav1.DeleteOptions{},
			err:   apierrors.NewInternalError(fmt.Errorf("failed to delete token bogus: %w", someerror)),
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					Delete("cattle-tokens", "bogus", gomock.Any()).
					Return(someerror)
			},
		},
		{
			name:  "secret deleted is ok",
			token: "bogus",
			opts:  &metav1.DeleteOptions{},
			err:   nil,
			storeSetup: func(secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList]) {
				secrets.EXPECT().
					Delete("cattle-tokens", "bogus", gomock.Any()).
					Return(nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// assemble and configure store from mock clients ...
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(nil)

			store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)
			test.storeSetup(secrets)

			// perform test and validate results
			err := store.Delete(test.token, test.opts)
			if test.err != nil {
				assert.Equal(t, test.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_SystemStore_UpdateLastUsedAt(t *testing.T) {
	t.Run("patch last-used-at, ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// assemble and configure store from mock clients ...
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)

		var patchData []byte
		secrets.EXPECT().Patch("cattle-tokens", "atoken", types.JSONPatchType, gomock.Any()).
			DoAndReturn(func(space, name string, pt types.PatchType, data []byte, subresources ...any) (*ext.Token, error) {
				patchData = data
				return nil, nil
			}).Times(1)

		now, nerr := time.Parse(time.RFC3339, "2024-12-06T03:02:01Z")
		assert.NoError(t, nerr)

		err := store.UpdateLastUsedAt("atoken", now)
		assert.NoError(t, err)
		require.NotEmpty(t, patchData)
		require.Equal(t,
			`[{"op":"replace","path":"/data/last-used-at","value":"MjAyNC0xMi0wNlQwMzowMjowMVo="}]`,
			string(patchData))
	})

	t.Run("patch last-used-at, error", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// assemble and configure store from mock clients ...
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)

		secrets.EXPECT().Patch("cattle-tokens", "atoken", types.JSONPatchType, gomock.Any()).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		now, _ := time.Parse(time.RFC3339, "2024-12-06T03:00:00")
		err := store.UpdateLastUsedAt("atoken", now)
		assert.Equal(t, fmt.Errorf("some error"), err)
	})
}

func Test_SystemStore_UpdateLastActivitySeen(t *testing.T) {
	t.Run("patch last-activity-seen, ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// assemble and configure store from mock clients ...
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)

		var patchData []byte
		secrets.EXPECT().Patch("cattle-tokens", "atoken", types.JSONPatchType, gomock.Any()).
			DoAndReturn(func(space, name string, pt types.PatchType, data []byte, subresources ...any) (*ext.Token, error) {
				patchData = data
				return nil, nil
			}).Times(1)

		now, nerr := time.Parse(time.RFC3339, "2024-12-06T03:02:01Z")
		assert.NoError(t, nerr)

		err := store.UpdateLastActivitySeen("atoken", now)
		assert.NoError(t, err)
		require.NotEmpty(t, patchData)
		require.Equal(t,
			`[{"op":"replace","path":"/data/last-activity-seen","value":"MjAyNC0xMi0wNlQwMzowMjowMVo="}]`,
			string(patchData))
	})

	t.Run("patch last-activity-seen, error", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		// assemble and configure store from mock clients ...
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(nil)

		store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)

		secrets.EXPECT().Patch("cattle-tokens", "atoken", types.JSONPatchType, gomock.Any()).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		now, _ := time.Parse(time.RFC3339, "2024-12-06T03:00:00")
		err := store.UpdateLastActivitySeen("atoken", now)
		assert.Equal(t, fmt.Errorf("some error"), err)
	})
}

func Test_SystemStore_Update(t *testing.T) {
	tests := []struct {
		name       string                // test name
		fullPerm   bool                  // permission level: full or not
		old        *ext.Token            // token to update, state before changes
		token      *ext.Token            // token to update, with changed fields
		opts       *metav1.UpdateOptions // update options
		rtok       *ext.Token            // expected op result, token
		err        error                 // expected op result, error
		storeSetup func(                 // configure store backend clients
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			scache *fake.MockCacheInterface[*corev1.Secret],
			timer *MocktimeHandler,
			hasher *MockhashHandler,
			auth *MockauthHandler)
	}{
		// Tests comparing inbound token against stored token, and rejecting changes to immutable fields
		{
			name:     "reject user id change",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.UserID = "dummy"
				return changed
			}(),
			err: apierrors.NewBadRequest("spec.userID is immutable"),
		},
		{
			name:     "reject principal change",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.UserPrincipal.DisplayName = "dummy"
				return changed
			}(),
			err: apierrors.NewBadRequest("spec.userprincipal is immutable"),
		},
		{
			name:     "reject kind change",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.Kind = ""
				return changed
			}(),
			err: apierrors.NewBadRequest("spec.kind is immutable"),
		},
		{
			name:     "reject cluster name change",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.ClusterName = "foo"
				return changed
			}(),
			err: apierrors.NewBadRequest("spec.clusterName is immutable"),
		},
		// Tests comparing inbound token against stored token, acceptable changes, and other errors
		{
			name:     "accept ttl extension (full permission)",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 5000
				return changed
			}(),
			rtok: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 5000
				changed.Status.LastUpdateTime = "this is a fake now"
				changed.Status.ExpiresAt = "0001-01-01T00:00:05Z"
				return changed
			}(),
			storeSetup: func(
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// Update: Return changed stored token
				secrets.EXPECT().
					Update(gomock.Any()).
					Return(&ttlPlusSecret, nil)
			},
			err: nil,
		},
		{
			name:     "accept ttl reduction (full permission)",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 3000
				return changed
			}(),
			rtok: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 3000
				changed.Status.LastUpdateTime = "this is a fake now"
				changed.Status.ExpiresAt = "0001-01-01T00:00:03Z"
				return changed
			}(),
			storeSetup: func(
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// Update: Return changed stored token
				secrets.EXPECT().
					Update(gomock.Any()).
					Return(&ttlSubSecret, nil)
			},
			err: nil,
		},
		{
			name:     "reject ttl extension (limited permission)",
			fullPerm: false,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 5000
				return changed
			}(),
			err: apierrors.NewBadRequest("forbidden to extend time-to-live"),
		},
		{
			name:     "accept ttl reduction (limited permission)",
			fullPerm: false,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 3000
				return changed
			}(),
			rtok: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 3000
				changed.Status.LastUpdateTime = "this is a fake now"
				changed.Status.ExpiresAt = "0001-01-01T00:00:03Z"
				return changed
			}(),
			storeSetup: func(
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// Update: Return changed stored token
				secrets.EXPECT().
					Update(gomock.Any()).
					Return(&ttlSubSecret, nil)
			},
			err: nil,
		},
		{
			name:     "fail to save changes",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 2000
				return changed
			}(),
			storeSetup: func(
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// Update: Fail
				secrets.EXPECT().
					Update(gomock.Any()).
					Return(nil, someerror)
			},
			err: apierrors.NewInternalError(fmt.Errorf("failed to save updated token: %w", someerror)),
		},
		{
			name:     "read back broken data after update",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 2000
				return changed
			}(),
			storeSetup: func(
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				reduced := properSecret.DeepCopy()
				delete(reduced.Data, "user-id")

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				// Update: Return broken data (piece missing)
				secrets.EXPECT().
					Update(gomock.Any()).
					Return(reduced, nil)
			},
			err: apierrors.NewInternalError(fmt.Errorf("failed to regenerate token: %w", userIDMissingError)),
		},
		{
			name:     "ok",
			fullPerm: true,
			opts:     &metav1.UpdateOptions{},
			old:      &properToken,
			token: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 2000
				return changed
			}(),
			rtok: func() *ext.Token {
				changed := properToken.DeepCopy()
				changed.Spec.TTL = 2000
				changed.Status.LastUpdateTime = "this is a fake now"
				changed.Status.ExpiresAt = "0001-01-01T00:00:02Z"
				return changed
			}(),
			storeSetup: func(
				secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
				scache *fake.MockCacheInterface[*corev1.Secret],
				timer *MocktimeHandler,
				hasher *MockhashHandler,
				auth *MockauthHandler) {

				// Fake current time
				timer.EXPECT().Now().Return("this is a fake now")

				changed := properSecret.DeepCopy()
				changed.Data["ttl"] = []byte("2000")
				changed.Data["last-update-time"] = []byte("this is a fake now")

				// Update: Return modified data
				secrets.EXPECT().
					Update(gomock.Any()).
					Return(changed, nil)
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// assemble and configure store from mock clients ...
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(scache)

			timer := NewMocktimeHandler(ctrl)
			hasher := NewMockhashHandler(ctrl)
			auth := NewMockauthHandler(ctrl)

			store := NewSystem(nil, nil, secrets, users, nil, nil, timer, hasher, auth)
			if test.storeSetup != nil {
				test.storeSetup(secrets, scache, timer, hasher, auth)
			}

			// perform test and validate results
			tok, err := store.update("", test.fullPerm, test.old, test.token, test.opts)

			if test.err != nil {
				assert.Equal(t, test.err, err)
				assert.Nil(t, tok)
			} else {
				assert.NoError(t, err)

				// Force equality on the fields update changes on semi-unpredictably on us
				// (ExpiresAt) -- Can we do this better ?
				// rtok.Status.ExpiresAt = test.token.Status.ExpiresAt

				assert.Equal(t, test.rtok, tok)
			}
		})
	}
}

func Test_SystemStore_Get(t *testing.T) {
	tests := []struct {
		name       string             // test name
		tokname    string             // name of token to retrieve
		session    string             // name of authenticating token
		opts       *metav1.GetOptions // retrieval options
		err        error              // expected op result, error
		tok        *ext.Token         // expected op result, token
		storeSetup func(              // configure store backend clients
			secrets *fake.MockCacheInterface[*corev1.Secret])
	}{
		{
			name: "backing secret not found",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(nil, bogusNotFoundError)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     bogusNotFoundError,
			tok:     nil,
		},
		{
			name: "some other error",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(nil, someerror)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err: apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w", "bogus",
				someerror)),
			tok: nil,
		},
		{
			name: "empty secret (no kube id)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(&corev1.Secret{}, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err: apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus",
				kubeIDMissingError)),
			tok: nil,
		},
		{
			name: "part-filled secret (no enabled)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				reduced := properSecret.DeepCopy()
				delete(reduced.Data, "enabled")

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", parseBoolError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (no ttl)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				reduced := properSecret.DeepCopy()
				delete(reduced.Data, "ttl")

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", parseIntError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (no hash)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				reduced := properSecret.DeepCopy()
				delete(reduced.Data, "hash")

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", hashMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (no auth provider)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				reduced := properSecret.DeepCopy()
				var up ext.TokenPrincipal
				json.Unmarshal(reduced.Data[FieldPrincipal], &up)
				up.Provider = ""
				reduced.Data[FieldPrincipal], _ = json.Marshal(up)

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", authProviderMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (no last update)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {

				reduced := properSecret.DeepCopy()
				delete(reduced.Data, "last-update-time")

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", lastUpdateMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (no principal id)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				reduced := properSecret.DeepCopy()
				var up ext.TokenPrincipal
				json.Unmarshal(reduced.Data[FieldPrincipal], &up)
				up.Name = ""
				reduced.Data[FieldPrincipal], _ = json.Marshal(up)

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", principalIDMissingError)),
			tok:     nil,
		},
		{
			name: "part-filled secret (no kube id)",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {

				reduced := properSecret.DeepCopy()
				delete(reduced.Data, "kube-uid")

				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(reduced, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", "bogus", kubeIDMissingError)),
			tok:     nil,
		},
		{
			name: "filled secret",
			storeSetup: func(secrets *fake.MockCacheInterface[*corev1.Secret]) {
				secrets.EXPECT().
					Get("cattle-tokens", "bogus").
					Return(&properSecret, nil)
			},
			tokname: "bogus",
			opts:    &metav1.GetOptions{},
			err:     nil,
			tok: &ext.Token{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Token",
					APIVersion: "ext.cattle.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "bogus",
					UID:  types.UID("2905498-kafld-lkad"),
				},
				Spec: ext.TokenSpec{
					UserID:        "lkajdlksjlkds",
					Description:   "",
					TTL:           4000,
					Enabled:       pointer.Bool(false),
					Kind:          "session",
					UserPrincipal: properPrincipal,
				},
				Status: ext.TokenStatus{
					Value:          "",
					Hash:           "kla9jkdmj",
					Expired:        true,
					ExpiresAt:      "0001-01-01T00:00:04Z",
					LastUpdateTime: "13:00:05",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// assemble and configure store from mock clients ...
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(scache)

			store := NewSystem(nil, nil, secrets, users, nil, nil, nil, nil, nil)
			test.storeSetup(scache)

			// perform test and validate results
			tok, err := store.Get(test.tokname, test.session, test.opts)
			if test.err != nil {
				assert.Equal(t, test.err, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.tok, tok)
		})
	}
}

// helpers

// implementation of the k8s user info interface for mocking

type mockUser struct {
	name string
}

func (u *mockUser) GetName() string {
	return u.name
}

func (u *mockUser) GetUID() string {
	return ""
}

func (u *mockUser) GetGroups() []string {
	return []string{}
}

func (u *mockUser) GetExtra() map[string][]string {
	return map[string][]string{}
}

// implementation of the k8s watch interface for mocking

func NewWatcherFor(e ...watch.Event) *mockWatch {
	ch := make(chan watch.Event, len(e))
	for _, ev := range e {
		ch <- ev
	}
	return &mockWatch{
		ch: ch,
	}
}

type mockWatch struct {
	ch chan watch.Event
}

func (w *mockWatch) Done() {
	close(w.ch)
}

func (w *mockWatch) ResultChan() <-chan watch.Event {
	return w.ch
}

func (w *mockWatch) Stop() {
}
