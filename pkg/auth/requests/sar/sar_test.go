package sar

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/stretchr/testify/assert"
	authV1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"testing"
)

func TestUserCanImpersonateUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	err := errors.New("unexpected error")
	tests := map[string]struct {
		sarClientGetterMock func(req *http.Request, user string, impUser string) SubjectAccessReviewClientGetter
		user                string
		impUser             string
		expectedAuthed      bool
		expecterErr         error
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
				sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarMock
			},
			user:           "user",
			impUser:        "impUser",
			expectedAuthed: true,
			expecterErr:    nil,
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
				sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: false,
					},
				}, nil)

				return sarMock
			},
			user:           "user",
			impUser:        "impUser",
			expectedAuthed: false,
			expecterErr:    nil,
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
				sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarMock
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
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.user, test.impUser))

			authed, err := sar.UserCanImpersonateUser(req, test.user, test.impUser)
			assert.Equal(t, test.expecterErr, err)
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
		expecterErr         error
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
				sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: true,
					},
				}, nil)

				return sarMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: true,
			expecterErr:    nil,
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
				sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
					Status: authV1.SubjectAccessReviewStatus{
						Allowed: false,
					},
				}, nil)

				return sarMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: false,
			expecterErr:    nil,
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
				sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(nil, err)

				return sarMock
			},
			user:           "user",
			group:          "admin",
			expectedAuthed: false,
			expecterErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.user, test.group))

			authed, err := sar.UserCanImpersonateGroup(req, test.user, test.group)
			assert.Equal(t, test.expecterErr, err)
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
		expecterErr         error
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
						sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
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
			expecterErr:    nil,
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
						sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(&authV1.SubjectAccessReview{
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
			expecterErr:    nil,
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
						sarClientMock.EXPECT().Create(context.TODO(), &sar, metav1.CreateOptions{}).Return(nil, err)
					}
				}

				return sarMock
			},
			user:           "user",
			extras:         map[string][]string{"extra": {"extra1"}},
			expectedAuthed: false,
			expecterErr:    err,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{}
			sar := NewSubjectAccessReview(test.sarClientGetterMock(req, test.extras, test.user))

			authed, err := sar.UserCanImpersonateExtras(req, test.user, test.extras)
			assert.Equal(t, test.expecterErr, err)
			assert.Equal(t, test.expectedAuthed, authed)
		})
	}
}
