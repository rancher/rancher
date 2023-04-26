package migration

import (
	"context"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"testing"
)

const (
	testNamespace = "test-cluster"
)

func Test_adMigration_migrate(t *testing.T) {
	var tests = []struct {
		name                       string
		users                      v3.UserInterface
		userLister                 v3.UserLister
		tokens                     v3.TokenInterface
		prtbs                      v3.ProjectRoleTemplateBindingInterface
		crtbs                      v3.ClusterRoleTemplateBindingInterface
		crtbIndexer                cache.Indexer
		prtbIndexer                cache.Indexer
		tokenIndexer               cache.Indexer
		authConfigs                v3.AuthConfigInterface
		wantErr                    bool
		userList                   []*v3.User
		principal                  v3.Principal
		existingTokens             []*v3.Token
		existingCrtbs              []*v3.ClusterRoleTemplateBinding
		existingPrtbs              []*v3.ProjectRoleTemplateBinding
		wantCRTBCreateCalledTimes  int
		wantCRTBDeleteCalledTimes  int
		wantPRTBCreateCalledTimes  int
		wantPRTBDeleteCalledTimes  int
		wantTokenUpdateCalledTimes int
		wantUserUpdateCalledTimes  int
	}{
		{
			name: "migrate user along with crtb prtb tokens",
			userList: []*v3.User{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "testuser4",
					},
					PrincipalIDs: []string{
						"activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
					},
				},
			},
			principal: v3.Principal{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name: "testuser4",
				},
				DisplayName:   "testuser4",
				LoginName:     "testuser4",
				PrincipalType: "user",
				Provider:      "activedirectory",
			},
			existingTokens: []*v3.Token{
				{
					UserID: "testuser4",
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "activedirectory://user-abcde",
							common.UserAttributeUserName:    "testuser4",
						},
					},
				},
			},
			existingCrtbs: []*v3.ClusterRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-2",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-3",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "anyprincipal",
				},
			},
			existingPrtbs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-2",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-3",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "anyprincipal",
				},
			},
			wantCRTBCreateCalledTimes:  2,
			wantCRTBDeleteCalledTimes:  2,
			wantPRTBDeleteCalledTimes:  2,
			wantPRTBCreateCalledTimes:  2,
			wantTokenUpdateCalledTimes: 1,
			wantUserUpdateCalledTimes:  1,
		},
		{
			name: "migrate user another user is already migrated",
			userList: []*v3.User{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "testuser4",
					},
					PrincipalIDs: []string{
						"activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "migrateduser4",
					},
					PrincipalIDs: []string{
						"activedirectory_user://alreadymigrateduser",
					},
				},
			},
			principal: v3.Principal{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name: "testuser4",
				},
				DisplayName:   "testuser4",
				LoginName:     "testuser4",
				PrincipalType: "user",
				Provider:      "activedirectory",
			},
			existingTokens: []*v3.Token{
				{
					UserID: "testuser4",
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "activedirectory://user-abcde",
							common.UserAttributeUserName:    "testuser4",
						},
					},
				},
			},
			existingCrtbs: []*v3.ClusterRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-2",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-3",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "anyprincipal",
				},
			},
			existingPrtbs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-2",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser4,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-3",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "anyprincipal",
				},
			},
			wantCRTBCreateCalledTimes:  2,
			wantCRTBDeleteCalledTimes:  2,
			wantPRTBDeleteCalledTimes:  2,
			wantPRTBCreateCalledTimes:  2,
			wantTokenUpdateCalledTimes: 1,
			wantUserUpdateCalledTimes:  1,
		},
		{
			name: "user principal does not need migration",
			userList: []*v3.User{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "testuser4",
					},
					PrincipalIDs: []string{
						"activedirectory_user://bfb34c007dc2c843adcc74ac3e27df21",
					},
				},
			},
			wantCRTBCreateCalledTimes:  0,
			wantCRTBDeleteCalledTimes:  0,
			wantPRTBDeleteCalledTimes:  0,
			wantPRTBCreateCalledTimes:  0,
			wantTokenUpdateCalledTimes: 0,
			wantUserUpdateCalledTimes:  0,
		},
		{
			name: "non active directory user does not need migration",
			userList: []*v3.User{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "githubusername",
					},
					PrincipalIDs: []string{
						"github_user://githubid",
					},
				},
			},
			wantCRTBCreateCalledTimes:  0,
			wantCRTBDeleteCalledTimes:  0,
			wantPRTBDeleteCalledTimes:  0,
			wantPRTBCreateCalledTimes:  0,
			wantTokenUpdateCalledTimes: 0,
			wantUserUpdateCalledTimes:  0,
		},
	}
	for _, tt := range tests {
		indexers := map[string]cache.IndexFunc{
			tokenByUserRefKey:            tokensByUserRefKey,
			crtbsByPrincipalAndUserIndex: crtbByPrincipalAndUserIndex,
			prtbsByPrincipalAndUserIndex: prtbByPrincipalAndUserIndex,
		}
		t.Run(tt.name, func(t *testing.T) {
			tokenUpdateCalled := 0
			crtbDeleteCalled := 0
			crtbCreateCalled := 0
			prtbDeleteCalled := 0
			prtbCreateCalled := 0
			userUpdateCalled := 0
			mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			mockIndexer.AddIndexers(indexers)
			for _, obj := range tt.existingTokens {
				mockIndexer.Add(obj)
			}
			for _, obj := range tt.existingPrtbs {
				mockIndexer.Add(obj)
			}
			for _, obj := range tt.existingCrtbs {
				mockIndexer.Add(obj)
			}
			m := &adMigration{
				users: &fakes.UserInterfaceMock{
					UpdateFunc: func(in1 *v3.User) (*v3.User, error) {
						userUpdateCalled++
						return in1, nil
					},
				},
				userLister: &fakes.UserListerMock{
					ListFunc: func(namespace string, selector labels.Selector) ([]*v3.User, error) {
						return tt.userList, nil
					},
				},
				tokenIndexer: mockIndexer,
				tokens: &fakes.TokenInterfaceMock{
					UpdateFunc: func(in1 *v3.Token) (*v3.Token, error) {
						tokenUpdateCalled++
						return in1, nil
					},
				},
				prtbs: &fakes.ProjectRoleTemplateBindingInterfaceMock{
					DeleteNamespacedFunc: func(_, _ string, _ *v1.DeleteOptions) error {
						prtbDeleteCalled++
						return nil
					},
					CreateFunc: func(in1 *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
						prtbCreateCalled++
						return in1, nil
					},
				},
				prtbIndexer: mockIndexer,
				crtbs: &fakes.ClusterRoleTemplateBindingInterfaceMock{
					DeleteNamespacedFunc: func(_, _ string, _ *v1.DeleteOptions) error {
						crtbDeleteCalled++
						return nil
					},
					CreateFunc: func(in1 *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
						crtbCreateCalled++
						return in1, nil
					},
				},
				crtbIndexer: mockIndexer,
			}
			providers.Providers = map[string]common.AuthProvider{
				activedirectory.Name: &mockProvider{principal: tt.principal},
			}
			if err := m.migrate(); (err != nil) != tt.wantErr {
				t.Errorf("migrate() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.wantPRTBDeleteCalledTimes, prtbDeleteCalled)
			assert.Equal(t, tt.wantPRTBCreateCalledTimes, prtbCreateCalled)
			assert.Equal(t, tt.wantCRTBDeleteCalledTimes, crtbDeleteCalled)
			assert.Equal(t, tt.wantCRTBCreateCalledTimes, crtbCreateCalled)
			assert.Equal(t, tt.wantUserUpdateCalledTimes, userUpdateCalled)
			assert.Equal(t, tt.wantTokenUpdateCalledTimes, tokenUpdateCalled)
		})
	}
}

func Test_adMigration_migrateCRTB(t *testing.T) {
	type args struct {
		newPrincipalID string
		dn             string
	}
	var tests = []struct {
		name                  string
		crtbs                 v3.ClusterRoleTemplateBindingInterface
		crtbIndexer           cache.Indexer
		existingCrtbs         []*v3.ClusterRoleTemplateBinding
		args                  args
		wantErr               bool
		wantCreateCalledTimes int
		wantDeleteCalledTimes int
		createFail            bool
		deleteFail            bool
	}{
		{
			name:                  "update 2 crtbs with new principalID",
			wantErr:               false,
			wantCreateCalledTimes: 2,
			wantDeleteCalledTimes: 2,
			createFail:            false,
			deleteFail:            false,
			args: args{
				dn:             "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				newPrincipalID: "activedirectory_user://abcdef123456abcdef123456",
			},
			existingCrtbs: []*v3.ClusterRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-2",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-3",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "anyprincipal",
				},
			},
		},
		{
			name:                  "fail to create crtb with new principalID",
			wantErr:               true,
			wantCreateCalledTimes: 1,
			wantDeleteCalledTimes: 0,
			createFail:            true,
			deleteFail:            false,
			args: args{
				dn:             "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				newPrincipalID: "activedirectory_user://abcdef123456abcdef123456",
			},
			existingCrtbs: []*v3.ClusterRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
			},
		},
		{
			name:                  "fail to delete crtb with old principalID",
			wantErr:               true,
			wantCreateCalledTimes: 1,
			wantDeleteCalledTimes: 1,
			createFail:            false,
			deleteFail:            true,
			args: args{
				dn:             "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				newPrincipalID: "activedirectory_user://abcdef123456abcdef123456",
			},
			existingCrtbs: []*v3.ClusterRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "crtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
			},
		},
	}
	for _, tt := range tests {
		indexers := map[string]cache.IndexFunc{
			crtbsByPrincipalAndUserIndex: crtbByPrincipalAndUserIndex,
		}
		t.Run(tt.name, func(t *testing.T) {
			crtbDeleteCalled := 0
			crtbCreateCalled := 0
			mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			mockIndexer.AddIndexers(indexers)
			for _, obj := range tt.existingCrtbs {
				mockIndexer.Add(obj)
			}
			m := &adMigration{
				crtbs: &fakes.ClusterRoleTemplateBindingInterfaceMock{
					DeleteNamespacedFunc: func(_, _ string, _ *v1.DeleteOptions) error {
						crtbDeleteCalled++
						if tt.deleteFail {
							return errors.New("simulated delete fail")
						}
						return nil
					},
					CreateFunc: func(in1 *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
						crtbCreateCalled++
						if tt.createFail {
							return nil, errors.New("simulated create fail")
						}
						return in1, nil
					},
				},
				crtbIndexer: mockIndexer,
			}

			if err := m.migrateCRTB(tt.args.newPrincipalID, tt.args.dn); (err != nil) != tt.wantErr {
				t.Errorf("migrateCRTB() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.wantCreateCalledTimes, crtbCreateCalled)
			assert.Equal(t, tt.wantDeleteCalledTimes, crtbDeleteCalled)
		})
	}
}

func Test_adMigration_migratePRTB(t *testing.T) {
	type args struct {
		newPrincipalID string
		dn             string
	}
	var tests = []struct {
		name                  string
		prtbs                 v3.ProjectRoleTemplateBindingInterface
		prtbIndexer           cache.Indexer
		existingPrtbs         []*v3.ProjectRoleTemplateBinding
		args                  args
		wantErr               bool
		wantCreateCalledTimes int
		wantDeleteCalledTimes int
		createFail            bool
		deleteFail            bool
	}{
		{
			name:                  "update 2 ptrbs with new principalID",
			wantErr:               false,
			wantCreateCalledTimes: 2,
			wantDeleteCalledTimes: 2,
			createFail:            false,
			deleteFail:            false,
			args: args{
				dn:             "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				newPrincipalID: "activedirectory_user://abcdef123456abcdef123456",
			},
			existingPrtbs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-2",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-3",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "anyprincipal",
				},
			},
		},
		{
			name:                  "fail to create prtb with new principalID",
			wantErr:               true,
			wantCreateCalledTimes: 1,
			wantDeleteCalledTimes: 0,
			createFail:            true,
			deleteFail:            false,
			args: args{
				dn:             "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				newPrincipalID: "activedirectory_user://abcdef123456abcdef123456",
			},
			existingPrtbs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
			},
		},
		{
			name:                  "fail to delete prtb with old principalID",
			wantErr:               true,
			wantCreateCalledTimes: 1,
			wantDeleteCalledTimes: 1,
			createFail:            false,
			deleteFail:            true,
			args: args{
				dn:             "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				newPrincipalID: "activedirectory_user://abcdef123456abcdef123456",
			},
			existingPrtbs: []*v3.ProjectRoleTemplateBinding{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "prtb-1",
						Namespace: testNamespace,
					},
					RoleTemplateName:  "testRoleTemplateName",
					UserPrincipalName: "activedirectory_user://CN=testuser1,CN=Users,DC=qa,DC=rancher,DC=space",
				},
			},
		},
	}
	for _, tt := range tests {
		indexers := map[string]cache.IndexFunc{
			prtbsByPrincipalAndUserIndex: prtbByPrincipalAndUserIndex,
		}
		t.Run(tt.name, func(t *testing.T) {
			prtbDeleteCalled := 0
			prtbCreateCalled := 0
			mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			mockIndexer.AddIndexers(indexers)
			for _, obj := range tt.existingPrtbs {
				mockIndexer.Add(obj)
			}
			m := &adMigration{
				prtbs: &fakes.ProjectRoleTemplateBindingInterfaceMock{
					DeleteNamespacedFunc: func(_, _ string, _ *v1.DeleteOptions) error {
						prtbDeleteCalled++
						if tt.deleteFail {
							return errors.New("simulated delete fail")
						}
						return nil
					},
					CreateFunc: func(in1 *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
						prtbCreateCalled++
						if tt.createFail {
							return nil, errors.New("simulated create fail")
						}
						return in1, nil
					},
				},
				prtbIndexer: mockIndexer,
			}

			if err := m.migratePRTB(tt.args.newPrincipalID, tt.args.dn); (err != nil) != tt.wantErr {
				t.Errorf("migratePRTB() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.wantCreateCalledTimes, prtbCreateCalled)
			assert.Equal(t, tt.wantDeleteCalledTimes, prtbDeleteCalled)
		})
	}
}

func Test_adMigration_migrateTokens(t *testing.T) {
	type args struct {
		userName       string
		newPrincipalID string
	}
	var tests = []struct {
		name                  string
		tokenIndexer          cache.Indexer
		tokens                v3.TokenInterface
		tokenList             []*v3.Token
		args                  args
		wantUpdateCalledTimes int
		wantErr               bool
	}{
		{
			name: "update token with new principalId",
			args: args{
				userName:       "testuser1",
				newPrincipalID: "activedirectory_user://newprincipalID",
			},
			wantErr:               false,
			wantUpdateCalledTimes: 1,
			tokenList: []*v3.Token{
				{
					UserID: "testuser1",
					UserPrincipal: v3.Principal{
						Provider: providers.LocalProvider,
						ExtraInfo: map[string]string{
							common.UserAttributePrincipalID: "activedirectory://user-abcde",
							common.UserAttributeUserName:    "testuser1",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		indexers := map[string]cache.IndexFunc{
			tokenByUserRefKey: tokensByUserRefKey,
		}
		t.Run(tt.name, func(t *testing.T) {
			tokenUpdateCalled := 0
			mockIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			mockIndexer.AddIndexers(indexers)
			for _, obj := range tt.tokenList {
				mockIndexer.Add(obj)
			}
			m := &adMigration{
				tokenIndexer: mockIndexer,
				tokens: &fakes.TokenInterfaceMock{
					UpdateFunc: func(in1 *v3.Token) (*v3.Token, error) {
						tokenUpdateCalled++
						return in1, nil
					},
				},
			}
			if err := m.migrateTokens(tt.args.userName, tt.args.newPrincipalID); (err != nil) != tt.wantErr {
				t.Errorf("migrateTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func prtbByPrincipalAndUserIndex(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{prtb.UserPrincipalName}, nil
}

func crtbByPrincipalAndUserIndex(obj interface{}) ([]string, error) {
	crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{crtb.UserPrincipalName}, nil
}

func tokensByUserRefKey(obj interface{}) ([]string, error) {
	token, ok := obj.(*v3.Token)
	if !ok {
		return []string{}, nil
	}
	return []string{token.UserID}, nil
}

type mockProvider struct {
	principal v3.Principal
}

func (p *mockProvider) IsDisabledProvider() (bool, error) {
	panic("not implemented")
}

func (p *mockProvider) GetName() string {
	panic("not implemented")
}

func (p *mockProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockProvider) SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *mockProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	return p.principal, nil
}

func (p *mockProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *mockProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return []v3.Principal{}, errors.New("Not implemented")
}

func (p *mockProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	return true, nil
}

func (p *mockProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	panic("not implemented")
}
