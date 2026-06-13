package auth

import (
	"fmt"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type extTokenTestCase struct {
	description                 string
	inputToken                  *ext.Token
	inputUserAttribute          *v3.UserAttribute
	expectedOutputUserAttribute *v3.UserAttribute
}

func TestExtOnChange(t *testing.T) {
	tokens := make(map[string]*ext.Token)
	userAttributes := make(map[string]*v3.UserAttribute)

	ctrl := gomock.NewController(t)

	// setup userAttribute mock instance
	userAttributesMock := wranglerfake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributesMock.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
			userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
			return userAttribute, nil
		},
	).AnyTimes()
	userAttributesMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
			userAttribute, ok := userAttributes[name]
			if ok {
				return userAttribute, nil
			}
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
	).AnyTimes()

	// setup userAttributesLister mock instance
	userAttributesListerMock := wranglerfake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributesListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(
		func(name string) (*v3.UserAttribute, error) {
			userAttribute, ok := userAttributes[name]
			if ok {
				return userAttribute, nil
			}
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
	).AnyTimes()

	testTokenController := ExtTokenController{
		userAttrRefresher: UserAttributeRefresher{
			userAttributes:       userAttributesMock,
			userAttributesLister: userAttributesListerMock,
		},
	}

	testCases := populateExtTestCases(tokens, userAttributes)
	for _, testcase := range testCases {
		testErr := fmt.Sprintf("test case failed: %s", testcase.description)
		returnToken, err := testTokenController.onChange(testcase.inputToken.Name, testcase.inputToken)
		assert.NoErrorf(t, err, "%s: unexpected onChange error", testErr)
		assert.Equalf(t, testcase.inputToken, returnToken, "%s: token", testErr)

		returnUserAttribute, _ := testTokenController.userAttrRefresher.userAttributesLister.Get(testcase.inputUserAttribute.Name)
		assert.Equalf(t, testcase.expectedOutputUserAttribute, returnUserAttribute, "%s: %s", testErr, testcase.inputUserAttribute.Name)
	}

	// test error from userattribute update
	// setup userAttribute mock instance
	userAttributesMock = wranglerfake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributesMock.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
			return nil, errors.NewServiceUnavailable("test reason")
		},
	).AnyTimes()
	userAttributesMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, opts metav1.GetOptions) (*v3.UserAttribute, error) {
			userAttribute, ok := userAttributes[name]
			if ok {
				return userAttribute, nil
			}
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
	).AnyTimes()

	// setup userAttributesLister mock instance
	userAttributesListerMock = wranglerfake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributesListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(
		func(name string) (*v3.UserAttribute, error) {
			userAttribute, ok := userAttributes[name]
			if ok {
				return userAttribute, nil
			}
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
	).AnyTimes()

	testUserAttributeErrorUpdateController := ExtTokenController{
		userAttrRefresher: UserAttributeRefresher{
			userAttributes:       userAttributesMock,
			userAttributesLister: userAttributesListerMock,
		},
	}

	genericTestToken := &ext.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		// UserID not being "" should trigger userattribute refresh check
		Spec: ext.TokenSpec{UserID: "abcd"},
	}
	userAttributes = map[string]*v3.UserAttribute{
		// ExtraByProvider being nil should trigger a userattribute update
		"abcd": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "abcd",
			},
		},
	}
	_, err := testUserAttributeErrorUpdateController.onChange(genericTestToken.Name, genericTestToken)
	assert.NotNilf(t, err, "handler should return err when userattribute client's update function returns error")

	// test non-notfound error from userattribute lister get
	// setup userAttribute mock instance
	userAttributesMock = wranglerfake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributesMock.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
			userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
			return userAttribute, nil
		},
	).AnyTimes()

	// setup userAttributesLister mock instance
	userAttributesListerMock = wranglerfake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributesListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(
		func(name string) (*v3.UserAttribute, error) {
			return nil, errors.NewServiceUnavailable("test reason")
		},
	)

	testUserAttributeErrorGetController := ExtTokenController{
		userAttrRefresher: UserAttributeRefresher{
			userAttributes:       userAttributesMock,
			userAttributesLister: userAttributesListerMock,
		},
	}

	genericTestToken = &ext.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		// UserID not being "" should trigger userattribute refresh check
		Spec: ext.TokenSpec{UserID: "abcd"},
	}
	_, err = testUserAttributeErrorGetController.onChange(genericTestToken.Name, genericTestToken)
	assert.NotNilf(t, err, "handler should return err when userattribute lister's get function returns non-notfound error")

	// test notfound error from userattribute lister get
	// setup userAttribute mock instance
	userAttributesMock = wranglerfake.NewMockNonNamespacedControllerInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl)
	userAttributesMock.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
			userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
			return userAttribute, nil
		},
	).AnyTimes()

	// setup userAttributesLister mock instance
	userAttributesListerMock = wranglerfake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributesListerMock.EXPECT().Get(gomock.Any()).DoAndReturn(
		func(name string) (*v3.UserAttribute, error) {
			return nil, errors.NewNotFound(schema.GroupResource{}, name)
		},
	)

	testUserAttributeErrorGetController = ExtTokenController{
		userAttrRefresher: UserAttributeRefresher{
			userAttributes:       userAttributesMock,
			userAttributesLister: userAttributesListerMock,
		},
	}

	genericTestToken = &ext.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		// UserID not being "" should trigger userattribute refresh check
		Spec: ext.TokenSpec{UserID: "abcd"},
	}
	_, err = testUserAttributeErrorGetController.onChange(genericTestToken.Name, genericTestToken)
	assert.Nil(t, err, "handler should not return err when userattribute lister's get function returns notfound error")
}

func populateExtTestCases(tokens map[string]*ext.Token, userAttributes map[string]*v3.UserAttribute) []extTokenTestCase {
	testCases := []extTokenTestCase{
		{
			inputToken: &ext.Token{Spec: ext.TokenSpec{UserID: "testuser"}},
			inputUserAttribute: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
				},
			},
			expectedOutputUserAttribute: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser",
				},
				NeedsRefresh: true,
			},
			description: "Verify that UserAttribute is triggered for a refresh if it is missing info that can" +
				" potentially be provided by the token.",
		},
		{
			inputToken: &ext.Token{Spec: ext.TokenSpec{UserID: "testuser2"}},
			inputUserAttribute: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser2",
				},
				ExtraByProvider: map[string]map[string][]string{"something": {"something": []string{"something"}}},
			},
			expectedOutputUserAttribute: &v3.UserAttribute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testuser2",
				},
				ExtraByProvider: map[string]map[string][]string{"something": {"something": []string{"something"}}},
			},
			description: "Verify that UserAttribute is not triggered for a refresh if it is not missing info" +
				" that can potentially be provided by the token.",
		},
	}
	for index, testCase := range testCases {
		id := fmt.Sprintf("test%d", index)
		testCase.inputToken.Name = id
		tokens[id] = testCase.inputToken.DeepCopy()
		if testCase.inputUserAttribute == nil {
			continue
		}
		userAttributes[testCase.inputUserAttribute.Name] = testCase.inputUserAttribute.DeepCopy()
	}
	return testCases
}
