package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestCheckAndRefreshIgnoresNotFound covers the case where the user attribute
// is deleted between the lister-based gating check and the fresh API Get inside
// the retry loop. The trigger must no-op silently instead of bubbling an error
// that would cause the token sync to requeue.
func TestCheckAndRefreshIgnoresNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)

	userAttribute := &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "abcd"},
		// ExtraByProvider == nil so the gating check (`needsRefresh`)
		// returns true and we enter `triggerRefresh`.
	}

	userAttributesMock := wranglerfake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributesMock.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
	)
	// No Update expectation: NotFound on the Get must short-circuit.

	userAttributesListerMock := wranglerfake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributesListerMock.EXPECT().Get(gomock.Any()).Return(userAttribute, nil).AnyTimes()

	refresher := UserAttributeRefresher{
		userAttributes:       userAttributesMock,
		userAttributesLister: userAttributesListerMock,
	}

	err := refresher.CheckAndRefresh("abcd")
	assert.NoError(t, err, "Must not error when the user attribute is gone")
}
