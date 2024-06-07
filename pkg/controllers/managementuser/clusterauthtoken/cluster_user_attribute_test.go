package clusterauthtoken

import (
	"errors"
	"fmt"
	"testing"

	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	clusterFakes "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3/fakes"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementFakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterUserAttributeHandlerCleanup(t *testing.T) {
	clusterUserAttribute := &clusterv3.ClusterUserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: "u-bdploclm4x",
		},
	}

	var userAttributeGetCalledTimes int
	userAttributes := &managementFakes.UserAttributeInterfaceMock{
		GetFunc: func(name string, opts metav1.GetOptions) (*managementv3.UserAttribute, error) {
			userAttributeGetCalledTimes++
			return nil, apierrors.NewNotFound(managementv3.Resource("userattributes"), name)
		},
	}

	var userAttributeListerGetCalledTimes int
	userAttributeLister := &managementFakes.UserAttributeListerMock{
		GetFunc: func(namespace string, name string) (*managementv3.UserAttribute, error) {
			userAttributeListerGetCalledTimes++
			return nil, apierrors.NewNotFound(managementv3.Resource("userattributes"), name)
		},
	}

	var deleted bool
	clusterUserAttributes := &clusterFakes.ClusterUserAttributeInterfaceMock{
		DeleteFunc: func(name string, opts *metav1.DeleteOptions) error {
			if name == clusterUserAttribute.Name {
				deleted = true
			}
			return nil
		},
	}

	handler := &clusterUserAttributeHandler{
		userAttribute:        userAttributes,
		userAttributeLister:  userAttributeLister,
		clusterUserAttribute: clusterUserAttributes,
	}

	_, err := handler.Sync("", clusterUserAttribute)
	if err != nil {
		t.Fatal(err)
	}

	if !deleted {
		t.Error("Expected ClusterUserAttribute to be deleted")
	}
	if want, got := 1, userAttributeGetCalledTimes; want != got {
		t.Errorf("Expected userAttributeGetCalledTimes %d got %d", want, got)
	}
	if want, got := 1, userAttributeListerGetCalledTimes; want != got {
		t.Errorf("Expected userAttributeListerGetCalledTimes %d got %d", want, got)
	}
}

func TestClusterUserAttributeHandlerCleanupErrors(t *testing.T) {
	attribName := "u-bdploclm4x"
	someErr := fmt.Errorf("some error")
	notFoundErr := apierrors.NewNotFound(managementv3.Resource("userattributes"), attribName)

	tests := []struct {
		desc                          string
		userAttributeGetErr           error
		userAttributeListerGetErr     error
		clusterUserAttributeDeleteErr error
		shouldErr                     bool
	}{
		{
			desc:                      "userAttributeListerGet error",
			userAttributeListerGetErr: someErr,
			shouldErr:                 true,
		},
		{
			desc:                      "userAttributeGet error",
			userAttributeListerGetErr: notFoundErr,
			userAttributeGetErr:       someErr,
			shouldErr:                 true,
		},
		{
			desc:                          "userAttributeDelete error",
			userAttributeGetErr:           notFoundErr,
			userAttributeListerGetErr:     notFoundErr,
			clusterUserAttributeDeleteErr: someErr,
			shouldErr:                     true,
		},
		{
			desc:                          "userAttributeDelete errors with NotFound",
			userAttributeGetErr:           notFoundErr,
			userAttributeListerGetErr:     notFoundErr,
			clusterUserAttributeDeleteErr: notFoundErr,
		},
		{
			desc:                          "userAttributeDelete errors with Gone",
			userAttributeGetErr:           notFoundErr,
			userAttributeListerGetErr:     notFoundErr,
			clusterUserAttributeDeleteErr: apierrors.NewGone("gone"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			userAttributes := &managementFakes.UserAttributeInterfaceMock{
				GetFunc: func(name string, opts metav1.GetOptions) (*managementv3.UserAttribute, error) {
					if name != attribName {
						t.Errorf("Unexpected name in userAttributes.Get call: %s", name)
					}
					return nil, tt.userAttributeGetErr
				},
			}

			userAttributeLister := &managementFakes.UserAttributeListerMock{
				GetFunc: func(namespace string, name string) (*managementv3.UserAttribute, error) {
					if name != attribName {
						t.Errorf("Unexpected name in userAttributeLister.Get call: %s", name)
					}
					return nil, tt.userAttributeListerGetErr
				},
			}

			clusterUserAttributes := &clusterFakes.ClusterUserAttributeInterfaceMock{
				DeleteFunc: func(name string, opts *metav1.DeleteOptions) error {
					if name != attribName {
						t.Errorf("Unexpected name in clusterUserAttributes.Delete call: %s", name)
					}
					return tt.clusterUserAttributeDeleteErr
				},
			}

			handler := &clusterUserAttributeHandler{
				userAttribute:        userAttributes,
				userAttributeLister:  userAttributeLister,
				clusterUserAttribute: clusterUserAttributes,
			}

			_, err := handler.Sync("", &clusterv3.ClusterUserAttribute{
				ObjectMeta: metav1.ObjectMeta{
					Name: attribName,
				},
			})
			if tt.shouldErr {
				if err == nil {
					t.Fatal("Expected an error")
				}

				if !errors.Is(err, someErr) {
					t.Errorf("Unexpected error: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
