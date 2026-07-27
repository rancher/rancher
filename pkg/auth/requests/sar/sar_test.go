package sar

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	authV1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

func TestUserCanImpersonateUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, user string, groups []string, impUser string) authorizationv1.SubjectAccessReviewInterface
		user           string
		groups         []string
		impUser        string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impUser string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "users",
							Name:     impUser,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			impUser:        "impUser",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"can impersonate with group permission": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impUser string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "users",
							Name:     impUser,
						},
					},
				}
				sarClientMock.EXPECT().Create(gomock.Any(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         []string{"azuread_group://a1cc05b8-d30b-454c-be77-0830ce1eae94", "system:authenticated"},
			impUser:        "impUser",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "users",
							Name:     "impUser",
						},
					},
				}
				sarClientMock.EXPECT().Create(gomock.Any(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: false,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			impUser:        "impUser",
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impUser string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "users",
							Name:     impUser,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			impUser:        "impUser",
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.user, test.groups, test.impUser))

			authed, err := sar.UserCanImpersonateUser(req, test.user, test.groups, test.impUser)
			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, user string, groups []string, impGroup string) authorizationv1.SubjectAccessReviewInterface
		user           string
		groups         []string
		group          string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "groups",
							Name:     impGroup,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			group:          "admin",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"can impersonate with group permission": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "groups",
							Name:     impGroup,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarClientMock
			},
			user:           "u-tifl6nuj5i",
			groups:         []string{"azuread_group://a1cc05b8-d30b-454c-be77-0830ce1eae94", "system:authenticated"},
			group:          "admin",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "groups",
							Name:     impGroup,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: false,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			group:          "admin",
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "groups",
							Name:     impGroup,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			group:          "admin",
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.user, test.groups, test.group))

			authed, err := sar.UserCanImpersonateGroup(req, test.user, test.groups, test.group)
			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateExtras(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, impExtras map[string][]string, user string, groups []string) authorizationv1.SubjectAccessReviewInterface
		user           string
		groups         []string
		extras         map[string][]string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string, groups []string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User:   user,
								Groups: groups,
								ResourceAttributes: &authV1.ResourceAttributes{
									Verb:     "impersonate",
									Resource: "userextras/" + name,
									Name:     value,
								},
							},
						}
						sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
							Status: authV1.SubjectAccessReviewStatus{
								Allowed: true,
							},
						}, nil)

					}
				}

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			extras:         map[string][]string{"extra": {"extra1", "extra2"}},
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"can impersonate with group permission": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string, groups []string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User:   user,
								Groups: groups,
								ResourceAttributes: &authV1.ResourceAttributes{
									Verb:     "impersonate",
									Resource: "userextras/" + name,
									Name:     value,
								},
							},
						}
						sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
							Status: authV1.SubjectAccessReviewStatus{
								Allowed: true,
							},
						}, nil)

					}
				}

				return sarClientMock
			},
			user:           "u-tifl6nuj5i",
			groups:         []string{"azuread_group://a1cc05b8-d30b-454c-be77-0830ce1eae94", "system:authenticated"},
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string, groups []string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User:   user,
								Groups: groups,
								ResourceAttributes: &authV1.ResourceAttributes{
									Verb:     "impersonate",
									Resource: "userextras/" + name,
									Name:     value,
								},
							},
						}
						sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
							Status: authV1.SubjectAccessReviewStatus{
								Allowed: false,
							},
						}, nil)

					}
				}

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string, groups []string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User:   user,
								Groups: groups,
								ResourceAttributes: &authV1.ResourceAttributes{
									Verb:     "impersonate",
									Resource: "userextras/" + name,
									Name:     value,
								},
							},
						}
						sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)
					}
				}

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.extras, test.user, test.groups))

			authed, err := sar.UserCanImpersonateExtras(req, test.user, test.groups, test.extras)
			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateServiceAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, user string, groups []string, impSA string, ns string, name string) authorizationv1.SubjectAccessReviewInterface
		user           string
		groups         []string
		impSA          string
		saNs           string
		saName         string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate with direct user permission": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impSA string, ns string, name string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:      "impersonate",
							Resource:  "serviceaccounts",
							Namespace: ns,
							Name:      name,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"can impersonate with group permission (Azure AD group)": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impSA string, ns string, name string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:      "impersonate",
							Resource:  "serviceaccounts",
							Namespace: ns,
							Name:      name,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarClientMock
			},
			user:           "u-tifl6nuj5i",
			groups:         []string{"azuread_group://a1cc05b8-d30b-454c-be77-0830ce1eae94", "system:authenticated"},
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impSA string, ns string, name string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:      "impersonate",
							Resource:  "serviceaccounts",
							Namespace: ns,
							Name:      name,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: false,
					},
				}, nil)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impSA string, ns string, name string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User:   user,
						Groups: groups,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:      "impersonate",
							Resource:  "serviceaccounts",
							Namespace: ns,
							Name:      name,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarClientMock
			},
			user:           "user",
			groups:         nil,
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: false,
			expectedErr:    err,
		},
		"impersonate parsing error": {
			sarClientFunc: func(req *http.Request, user string, groups []string, impSA string, ns string, name string) authorizationv1.SubjectAccessReviewInterface {
				return mocks.NewMockSubjectAccessReviewInterface(ctrl)
			},
			user:           "user",
			groups:         nil,
			impSA:          "system:serviceaccount:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: false,
			expectedErr:    fmt.Errorf("invalid service account username format: expected system:serviceaccount:<namespace>:<name>, but got 'system:serviceaccount:example-test'"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.user, test.groups, test.impSA, test.saNs, test.saName))

			authed, err := sar.UserCanImpersonateServiceAccount(req, test.user, test.groups, test.impSA)
			if test.expectedErr != nil {
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateExtrasWithMaliciousKey(t *testing.T) {
	tests := map[string]struct {
		extras map[string][]string
	}{
		"path traversal via dotdot-slash in key": {
			extras: map[string][]string{"../users": {"admin"}},
		},
		"forward slash splits resource path": {
			extras: map[string][]string{"some/key": {"value"}},
		},
		"empty key": {
			extras: map[string][]string{"": {"value"}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// The stub always returns Allowed: true so that if the key is not validated
			// the call succeeds — causing authed=true and err=nil, which fails the
			// assertions below and proves the vulnerability is present.
			getter := &stubSubjectAccessReviewClient{
				createFunc: func(_ context.Context, _ *authV1.SubjectAccessReview, _ metav1.CreateOptions) (*authV1.SubjectAccessReview, error) {
					return &authV1.SubjectAccessReview{
						Status: authV1.SubjectAccessReviewStatus{Allowed: true},
					}, nil
				},
			}

			req := &http.Request{}
			s := NewSubjectAccessReview(getter)

			authed, err := s.UserCanImpersonateExtras(req, "user", nil, test.extras)
			assert.False(t, authed, "impersonation should be denied for malicious extra key in case %q", name)
			assert.Error(t, err, "expected an error for malicious extra key in case %q", name)
		})
	}
}
