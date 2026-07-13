package sar

import (
	"context"
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
		sarClientFunc  func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface
		user           string
		impUser        string
		expectedAuthed bool
		expecterErr    error
	}{
		"can impersonate": {
			sarClientFunc: func(req *http.Request, user string, impUser string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			impUser:        "impUser",
			expectedAuthed: true,
		},
		"can impersonate with group permission": {
			sarClientFunc: func(req *http.Request, user, impUser string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			impUser:        "impUser",
			expectedAuthed: true,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			impUser:        "impUser",
			expectedAuthed: false,
			expecterErr:    nil,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, user string, impUser string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			impUser:        "impUser",
			expectedAuthed: false,
			expecterErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.user, test.impUser))

			authed, err := sar.UserCanImpersonateUser(req, test.user, test.impUser)
			assert.Equal(t, test.expecterErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	unexpectedErr := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface
		user           string
		group          string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate": {
			sarClientFunc: func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			group:          "admin",
			expectedAuthed: true,
		},
		"can impersonate with group permission": {
			sarClientFunc: func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			group:          "admin",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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
			group:          "admin",
			expectedAuthed: false,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, user, impGroup string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "groups",
							Name:     impGroup,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, unexpectedErr)

				return sarClientMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: false,
			expectedErr:    unexpectedErr,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.user, test.group))

			authed, err := sar.UserCanImpersonateGroup(req, test.user, test.group)
			assert.Equal(t, err, test.expectedErr)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateExtras(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, impExtras map[string][]string, user string) authorizationv1.SubjectAccessReviewInterface
		user           string
		extras         map[string][]string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User: user,
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
			extras:         map[string][]string{"extra": {"extra1", "extra2"}},
			expectedAuthed: true,
		},
		"can impersonate with group permission": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User: user,
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
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User: user,
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
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: false,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, impExtras map[string][]string, user string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				for name, values := range impExtras {
					for _, value := range values {
						sar := authV1.SubjectAccessReview{
							Spec: authV1.SubjectAccessReviewSpec{
								User: user,
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
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.extras, test.user))

			authed, err := sar.UserCanImpersonateExtras(req, test.user, test.extras)
			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateServiceAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientFunc  func(req *http.Request, user, impSA string) authorizationv1.SubjectAccessReviewInterface
		user           string
		impSA          string
		saName         string
		expectedAuthed bool
		expectedErr    error
	}{
		"can impersonate with direct user permission": {
			sarClientFunc: func(req *http.Request, user string, impSA string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "serviceaccounts",
							Name:     impSA,
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
			impSA:          "impSA",
			expectedAuthed: true,
		},
		"can impersonate with group permission (Azure AD group)": {
			sarClientFunc: func(req *http.Request, user string, impSA string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "serviceaccounts",
							Name:     impSA,
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
			impSA:          "system:serviceaccount:example-ns:example-test",
			saName:         "system:serviceaccount:example-ns:example-test",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientFunc: func(req *http.Request, user, impSA string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "serviceaccounts",
							Name:     impSA,
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
			impSA:          "impSA",
			expectedAuthed: false,
		},
		"impersonate error": {
			sarClientFunc: func(req *http.Request, user, impSA string) authorizationv1.SubjectAccessReviewInterface {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:     "impersonate",
							Resource: "serviceaccounts",
							Name:     impSA,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarClientMock
			},
			user:           "user",
			impSA:          "impUser",
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientFunc(req, test.user, test.impSA))

			authed, err := sar.UserCanImpersonateServiceAccount(req, test.user, test.impSA)
			assert.Equal(t, test.expectedErr, err)
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

			authed, err := s.UserCanImpersonateExtras(req, "user", test.extras)
			assert.False(t, authed, "impersonation should be denied for malicious extra key in case %q", name)
			assert.Error(t, err, "expected an error for malicious extra key in case %q", name)
		})
	}
}
