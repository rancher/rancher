package sar

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	authV1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUserCanImpersonateUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientGetterMock func(req *http.Request, user string, impUser string) SubjectAccessReviewClientGetter
		user                string
		impUser             string
		expectedAuthed      bool
		expectedErr         error
	}{
		"can impersonate": {
			sarClientGetterMock: func(req *http.Request, user string, impUser string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
			},
			user:           "user",
			impUser:        "impUser",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientGetterMock: func(req *http.Request, user string, impUser string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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
						Allowed: false,
					},
				}, nil)

				return sarMock
			},
			user:           "user",
			impUser:        "impUser",
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientGetterMock: func(req *http.Request, user string, impUser string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
			},
			user:           "user",
			impUser:        "impUser",
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.user, test.impUser))

			authed, err := sar.UserCanImpersonateUser(req, test.user, test.impUser)
			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientGetterMock func(req *http.Request, user string, impGroup string) SubjectAccessReviewClientGetter
		user                string
		group               string
		expectedAuthed      bool
		expectedErr         error
	}{
		"can impersonate": {
			sarClientGetterMock: func(req *http.Request, user string, impGroup string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientGetterMock: func(req *http.Request, user string, impGroup string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientGetterMock: func(req *http.Request, user string, impGroup string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: false,
			expectedErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.user, test.group))

			authed, err := sar.UserCanImpersonateGroup(req, test.user, test.group)
			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}

func TestUserCanImpersonateExtras(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientGetterMock func(req *http.Request, impExtras map[string][]string, user string) SubjectAccessReviewClientGetter
		user                string
		extras              map[string][]string
		expectedAuthed      bool
		expectedErr         error
	}{
		"can impersonate": {
			sarClientGetterMock: func(req *http.Request, impExtras map[string][]string, user string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
			},
			user:           "user",
			extras:         map[string][]string{"extra": {"extra1", "extra2"}},
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientGetterMock: func(req *http.Request, impExtras map[string][]string, user string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
			},
			user:           "user",
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientGetterMock: func(req *http.Request, impExtras map[string][]string, user string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

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

				return sarMock
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
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.extras, test.user))

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
		sarClientGetterMock func(req *http.Request, user string, impSA string, ns string, name string) SubjectAccessReviewClientGetter
		user                string
		impSA               string
		saNs                string
		saName              string
		expectedAuthed      bool
		expectedErr         error
	}{
		"can impersonate": {
			sarClientGetterMock: func(req *http.Request, user string, impSA string, ns string, name string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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

				return sarMock
			},
			user:           "user",
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: true,
			expectedErr:    nil,
		},
		"impersonate not allowed": {
			sarClientGetterMock: func(req *http.Request, user string, impSA string, ns string, name string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
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

				return sarMock
			},
			user:           "user",
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: false,
			expectedErr:    nil,
		},
		"impersonate error": {
			sarClientGetterMock: func(req *http.Request, user string, impSA string, ns string, name string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)

				sar := authV1.SubjectAccessReview{
					Spec: authV1.SubjectAccessReviewSpec{
						User: user,
						ResourceAttributes: &authV1.ResourceAttributes{
							Verb:      "impersonate",
							Resource:  "serviceaccounts",
							Namespace: ns,
							Name:      name,
						},
					},
				}
				sarClientMock.EXPECT().Create(req.Context(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarMock
			},
			user:           "user",
			impSA:          "system:serviceaccount:example-ns:example-test",
			saNs:           "example-ns",
			saName:         "example-test",
			expectedAuthed: false,
			expectedErr:    err,
		},
		"impersonate parsing error": {
			sarClientGetterMock: func(req *http.Request, user string, impSA string, ns string, name string) SubjectAccessReviewClientGetter {
				sarClientMock := mocks.NewMockSubjectAccessReviewInterface(ctrl)
				sarMock := mocks.NewMockSubjectAccessReviewClientGetter(ctrl)
				sarMock.EXPECT().SubjectAccessReviewForCluster(req).Return(sarClientMock, nil)
				return sarMock
			},
			user:           "user",
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
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.user, test.impSA, test.saNs, test.saName))

			authed, err := sar.UserCanImpersonateServiceAccount(req, test.user, test.impSA)
			if test.expectedErr != nil {
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}
