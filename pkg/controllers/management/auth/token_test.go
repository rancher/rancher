package auth

import (
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	tokens2 "github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type tokenTestCase struct {
	inputToken                  *v3.Token
	expectedOutputToken         *v3.Token
	inputUserAttribute          *v3.UserAttribute
	expectedOutputUserAttribute *v3.UserAttribute
	enableHashing               bool
	description                 string
}

func TestSync(t *testing.T) {
	tokens := make(map[string]*v3.Token)
	userAttributes := make(map[string]*v3.UserAttribute)

	testTokenController := TokenController{
		tokens: &fakes.TokenInterfaceMock{
			UpdateFunc: func(token *v3.Token) (*v3.Token, error) {
				tokens[token.Name] = token.DeepCopy()
				return token, nil
			},
			GetFunc: func(name string, opts metav1.GetOptions) (*v3.Token, error) {
				token, ok := tokens[name]
				if ok {
					return token, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
				return userAttribute, nil
			},
		},
		userAttributesLister: &fakes.UserAttributeListerMock{
			GetFunc: func(namespace string, name string) (*v3.UserAttribute, error) {
				userAttribute, ok := userAttributes[name]
				if ok {
					return userAttribute, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
	}
	testCases := populateTestCases(tokens, userAttributes)
	for _, testcase := range testCases {
		testErr := fmt.Sprintf("test case failed: %s", testcase.description)
		if testcase.enableHashing {
			features.TokenHashing.Set(true)
		}
		returnToken, _ := testTokenController.sync(testcase.inputToken.Name, testcase.inputToken)
		storedToken, _ := testTokenController.tokens.Get(testcase.inputToken.Name, metav1.GetOptions{})
		assert.Equalf(t, returnToken, storedToken, fmt.Sprintf("%s", testcase.inputToken.Name), testErr)
		features.TokenHashing.Set(false)
		if testcase.enableHashing {
			tokenVal := returnToken.(*v3.Token).Token
			assert.NotEqualf(t, tokenVal, testcase.inputToken.Token, testErr)
			hasher, err := hashers.GetHasherForHash(tokenVal)
			assert.Nil(t, err)
			assert.Nil(t, hasher.VerifyHash(tokenVal, testcase.inputToken.Token))
			testcase.expectedOutputToken.Token = ""
			returnToken.(*v3.Token).Token = ""
		}
		assert.Equalf(t, testcase.expectedOutputToken, returnToken, fmt.Sprintf("%s", testcase.inputToken.Name), testErr)
		if testcase.inputUserAttribute == nil {
			continue
		}
		returnUserAttribute, _ := testTokenController.userAttributesLister.Get("", testcase.inputUserAttribute.Name)
		assert.Equalf(t, testcase.expectedOutputUserAttribute, returnUserAttribute, fmt.Sprintf("%s", testcase.inputToken.Name), testErr)
	}

	// test error from token update
	testTokenErrorUpdateController := TokenController{
		tokens: &fakes.TokenInterfaceMock{
			UpdateFunc: func(token *v3.Token) (*v3.Token, error) {
				return nil, errors.NewServiceUnavailable("test reason")
			},
			GetFunc: func(name string, opts metav1.GetOptions) (*v3.Token, error) {
				token, ok := tokens[name]
				if ok {
					return token, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
				return userAttribute, nil
			},
		},
		userAttributesLister: &fakes.UserAttributeListerMock{
			GetFunc: func(namespace string, name string) (*v3.UserAttribute, error) {
				userAttribute, ok := userAttributes[name]
				if ok {
					return userAttribute, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
	}
	genericTestToken := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		Token: "1234",
		// TTLMillis not being 0 while ExpiresAt is "" should trigger an update
		TTLMillis: 300,
	}
	_, err := testTokenErrorUpdateController.sync(genericTestToken.Name, genericTestToken)
	assert.NotNilf(t, err, "handler should return err when token client's update function returns error")

	// test error from userattribute update
	testUserAttributeErrorUpdateController := TokenController{
		tokens: &fakes.TokenInterfaceMock{
			UpdateFunc: func(token *v3.Token) (*v3.Token, error) {
				tokens[token.Name] = token.DeepCopy()
				return token, nil
			},
			GetFunc: func(name string, opts metav1.GetOptions) (*v3.Token, error) {
				token, ok := tokens[name]
				if ok {
					return token, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				return nil, errors.NewServiceUnavailable("test reason")
			},
		},
		userAttributesLister: &fakes.UserAttributeListerMock{
			GetFunc: func(namespace string, name string) (*v3.UserAttribute, error) {
				userAttribute, ok := userAttributes[name]
				if ok {
					return userAttribute, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
	}
	genericTestToken = &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		// UserID not being "" should trigger userattribute refresh check
		UserID: "abcd",
	}
	userAttributes = map[string]*v3.UserAttribute{
		// ExtraByProvider being nil should trigger a userattribute update
		"abcd": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "testuser",
			},
		},
	}
	_, err = testUserAttributeErrorUpdateController.sync(genericTestToken.Name, genericTestToken)
	assert.NotNilf(t, err, "handler should return err when userattribute client's update function returns error")

	// test non-notfound error from userattribute lister get
	testUserAttributeErrorGetController := TokenController{
		tokens: &fakes.TokenInterfaceMock{
			UpdateFunc: func(token *v3.Token) (*v3.Token, error) {
				tokens[token.Name] = token.DeepCopy()
				return token, nil
			},
			GetFunc: func(name string, opts metav1.GetOptions) (*v3.Token, error) {
				token, ok := tokens[name]
				if ok {
					return token, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
				return userAttribute, nil
			},
		},
		userAttributesLister: &fakes.UserAttributeListerMock{
			GetFunc: func(namespace string, name string) (*v3.UserAttribute, error) {
				return nil, errors.NewServiceUnavailable("test reason")
			},
		},
	}
	genericTestToken = &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		// UserID not being "" should trigger userattribute refresh check
		UserID: "abcd",
	}
	_, err = testUserAttributeErrorGetController.sync(genericTestToken.Name, genericTestToken)
	assert.NotNilf(t, err, "handler should return err when userattribute lister's get function returns non-notfound error")

	// test notfound error from userattribute lister get
	testUserAttributeErrorGetController = TokenController{
		tokens: &fakes.TokenInterfaceMock{
			UpdateFunc: func(token *v3.Token) (*v3.Token, error) {
				tokens[token.Name] = token.DeepCopy()
				return token, nil
			},
			GetFunc: func(name string, opts metav1.GetOptions) (*v3.Token, error) {
				token, ok := tokens[name]
				if ok {
					return token, nil
				}
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			UpdateFunc: func(userAttribute *v3.UserAttribute) (*v3.UserAttribute, error) {
				userAttributes[userAttribute.Name] = userAttribute.DeepCopy()
				return userAttribute, nil
			},
		},
		userAttributesLister: &fakes.UserAttributeListerMock{
			GetFunc: func(namespace string, name string) (*v3.UserAttribute, error) {
				return nil, errors.NewNotFound(schema.GroupResource{}, name)
			},
		},
	}
	genericTestToken = &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testtoken",
		},
		// UserID not being "" should trigger userattribute refresh check
		UserID: "abcd",
	}
	_, err = testUserAttributeErrorGetController.sync(genericTestToken.Name, genericTestToken)
	assert.Nil(t, err, "handler should not return err when userattribute lister's get function returns notfound error")
}

func populateTestCases(tokens map[string]*v3.Token, userAttributes map[string]*v3.UserAttribute) []tokenTestCase {
	timeNow := metav1.NewTime(time.Now())
	hashedToken, _ := hashers.GetHasher().CreateHash("1234")
	testCases := []tokenTestCase{
		{
			inputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"1", "2", "3"},
				},
			},
			expectedOutputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"1", "2", "3"},
				},
			},
			description: "Base case that confirms no changes are made to a token that does not have the" +
				" \"controller.cattle.io/cat-token-controller\" finalizer.",
		},
		{
			inputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"1", "controller.cattle.io/cat-token-controller", "3"},
				},
			},
			expectedOutputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"1", "controller.cattle.io/cat-token-controller", "3"},
				},
			},
			description: "Tests that the \"controller.cattle.io/cat-token-controller\" finalizer is not removed if the token does" +
				"not have a deltion timestamp.",
		},
		{
			inputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers:        []string{"1", "controller.cattle.io/cat-token-controller", "3"},
					DeletionTimestamp: &timeNow,
				},
			},
			expectedOutputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers:        []string{"1", "3"},
					DeletionTimestamp: &timeNow,
				},
			},
			description: "Tests the the \"controller.cattle.io/cat-token-controller\" is removed if token possesses" +
				" deletion timestamp.",
		},
		{
			inputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: timeNow,
				},
				TTLMillis: 300,
			},
			expectedOutputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: timeNow,
				},
				TTLMillis: 300,
				ExpiresAt: timeNow.Add(300 * time.Millisecond).UTC().Format(time.RFC3339),
			},
		},
		{
			inputToken:          &v3.Token{UserID: "testuser"},
			expectedOutputToken: &v3.Token{UserID: "testuser"},
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
			description: "Tests that UserAttribute is trigger for a refresh if it is missing info that can" +
				"potentially be provided by the token.",
		},
		{
			inputToken:          &v3.Token{UserID: "testuser2"},
			expectedOutputToken: &v3.Token{UserID: "testuser2"},
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
			description: "Tests that UserAttribute is not triggered for a refresh if it is not missing info that can" +
				"potentially be provided by the token.",
		},
		{
			inputToken: &v3.Token{
				Token: "1234",
			},
			expectedOutputToken: &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{tokens2.TokenHashed: "true"},
				},
				Token: hashedToken,
			},
			enableHashing: true,
			description:   "",
		},
	}
	for index, testCase := range testCases {
		id := fmt.Sprintf("test%d", index)
		testCase.inputToken.Name = id
		testCase.expectedOutputToken.Name = id
		tokens[id] = testCase.inputToken.DeepCopy()
		if testCase.inputUserAttribute == nil {
			continue
		}
		userAttributes[testCase.inputUserAttribute.Name] = testCase.inputUserAttribute.DeepCopy()
	}
	return testCases
}
