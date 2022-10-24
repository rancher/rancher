package common

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestEnsureClusterToken(t *testing.T) {
	// tokens have a token field - this is the constant value for this field
	const tokenTokenValue = "12347879"
	tests := []struct {
		name               string
		tokenName          string
		tokenEnabled       bool
		getErr             error
		updErr             error
		expectedName       string
		tokenEnableDesired bool
		errDesired         bool
	}{
		{
			// enable, already enabled, err enabling, err getting, err prefix
			name:               "enable token",
			tokenName:          "test-kubeconfig",
			tokenEnabled:       false,
			getErr:             nil,
			updErr:             nil,
			expectedName:       "test-kubeconfig:" + tokenTokenValue,
			tokenEnableDesired: true,
			errDesired:         false,
		},
		{
			name:               "already enabled token",
			tokenName:          "test-kubeconfig",
			tokenEnabled:       true,
			getErr:             nil,
			updErr:             nil,
			expectedName:       "test-kubeconfig:" + tokenTokenValue,
			tokenEnableDesired: false,
			errDesired:         false,
		},
		{
			name:               "error when updating token",
			tokenName:          "test-kubeconfig",
			tokenEnabled:       true,
			getErr:             fmt.Errorf("server temporarily unavailable"),
			updErr:             nil,
			expectedName:       "",
			tokenEnableDesired: false,
			errDesired:         true,
		},
		{
			name:               "error when getting token",
			tokenName:          "test-kubeconfig",
			tokenEnabled:       true,
			getErr:             nil,
			updErr:             fmt.Errorf("server temporarily unavailable"),
			expectedName:       "",
			tokenEnableDesired: false,
			errDesired:         true,
		},
		{
			name:               "error due to token name",
			tokenName:          "token-mytoken",
			tokenEnabled:       true,
			getErr:             nil,
			updErr:             nil,
			expectedName:       "",
			tokenEnableDesired: false,
			errDesired:         true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mockLister := NewMockTokenLister()
			mockPutter := NewMockTokenPutter()
			tokenInterface := fakes.TokenInterfaceMock{}
			tokenInterface.UpdateFunc = mockPutter.UpdateToken
			token := &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: test.tokenName,
				},
				Token:   tokenTokenValue,
				Enabled: &test.tokenEnabled,
			}
			if test.getErr != nil {
				mockLister.AddError(token.Name, test.getErr)
			} else if test.updErr != nil {
				mockPutter.AddError(token.Name, test.updErr)
			} else {
				mockLister.AddToken(token)
			}
			manager := userManager{
				tokenLister: mockLister,
				tokens:      &tokenInterface,
			}
			result, err := manager.EnsureToken(test.tokenName, "", "", "tUser")
			if test.errDesired {
				assert.Error(t, err, "expected error but did not get one")
			} else {
				assert.NoError(t, err, "did not expect error but found one")
				assert.Equal(t, test.expectedName, result, "output token name was not as expected")
				if test.tokenEnableDesired {
					tokens := mockPutter.GetUpdTokens()
					var updToken *v3.Token
					for _, newToken := range tokens {
						if newToken.Name == token.Name {
							updToken = newToken
						}
					}
					assert.NotNil(t, updToken, "expected token to be updated")
					assert.True(t, *updToken.Enabled, "expected token to be enabled")
				}
			}
		})
	}
}

type mockTokenLister struct {
	tokens      []*v3.Token
	errorTokens map[string]error
}

func NewMockTokenLister() *mockTokenLister {
	return &mockTokenLister{
		tokens:      []*v3.Token{},
		errorTokens: map[string]error{},
	}
}

func (m *mockTokenLister) List(namespace string, selector labels.Selector) (ret []*v3.Token, err error) {
	return m.tokens, nil
}

func (m *mockTokenLister) Get(namespace, name string) (*v3.Token, error) {
	for _, token := range m.tokens {
		if token.Name == name {
			return token, nil
		}
	}
	if err, ok := m.errorTokens[name]; ok {
		return nil, err
	}
	return nil, fmt.Errorf("token %s not found", name)
}

func (m *mockTokenLister) AddToken(token *v3.Token) {
	tokenIdx := -1
	for idx, setToken := range m.tokens {
		if setToken.Name == token.Name {
			tokenIdx = idx
		}
	}
	if tokenIdx >= 1 {
		m.tokens[tokenIdx] = token
	} else {
		m.tokens = append(m.tokens, token)
	}
}

func (m *mockTokenLister) AddError(name string, err error) {
	m.errorTokens[name] = err
}

type mockTokenPutter struct {
	errorTokens map[string]error
	updTokens   []*v3.Token
}

func NewMockTokenPutter() *mockTokenPutter {
	return &mockTokenPutter{
		errorTokens: map[string]error{},
		updTokens:   []*v3.Token{},
	}
}

func (m *mockTokenPutter) UpdateToken(in *v3.Token) (*v3.Token, error) {
	if err, ok := m.errorTokens[in.Name]; ok {
		return nil, err
	}
	m.updTokens = append(m.updTokens, in)
	return in, nil
}

func (m *mockTokenPutter) AddError(name string, err error) {
	m.errorTokens[name] = err
}

func (m *mockTokenPutter) GetUpdTokens() []*v3.Token {
	return m.updTokens
}
