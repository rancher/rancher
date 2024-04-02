package clusterauthtoken

import (
	"fmt"

	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type clusterUserAttributeHandler struct {
	userAttribute        managementv3.UserAttributeInterface
	userAttributeLister  managementv3.UserAttributeLister
	clusterUserAttribute clusterv3.ClusterUserAttributeInterface
}

// Sync clusterUserAttributes and userAttributes
func (h *clusterUserAttributeHandler) Sync(key string, clusterUserAttribute *clusterv3.ClusterUserAttribute) (runtime.Object, error) {
	if clusterUserAttribute == nil || clusterUserAttribute.DeletionTimestamp != nil {
		return nil, nil
	}

	userAttribute, err := h.userAttributeLister.Get("", clusterUserAttribute.Name)
	if err != nil {
		// If the corresponding UserAttribute no longer exists, we need to clean up the ClusterUserAttribute.
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		// There is a chance that it hasn't ended up in the cache yet so we try to get it afresh.
		userAttribute, err = h.userAttribute.Get(clusterUserAttribute.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}

			err = h.clusterUserAttribute.Delete(clusterUserAttribute.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
				return nil, fmt.Errorf("error deleting orphaned clusteruserattribute %s: %w", clusterUserAttribute.Name, err)
			}

			logrus.Infof("Deleted orphaned clusteruserattribute %s", clusterUserAttribute.Name)
			return nil, nil
		}
		// The userAttribute exists, proceed.
	}

	if !clusterUserAttribute.NeedsRefresh {
		return nil, nil
	}
	if userAttribute.NeedsRefresh {
		return nil, nil
	}
	if userAttribute.LastRefresh != clusterUserAttribute.LastRefresh {
		return nil, nil
	}

	userAttribute.NeedsRefresh = true
	_, err = h.userAttribute.Update(userAttribute)
	return nil, err
}
