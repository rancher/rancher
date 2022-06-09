package rbac

import (
	"github.com/rancher/rancher/pkg/impersonation"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

func (m *manager) ensureServiceAccountImpersonator(username string) error {
	logrus.Debugf("ensuring service account impersonator for %s", username)
	i, err := impersonation.New(&user.DefaultInfo{UID: username}, m.workload)
	if apierrors.IsNotFound(err) {
		logrus.Warnf("could not find user %s, will not create impersonation account on cluster", username)
		return nil
	}
	if err != nil {
		return err
	}
	_, err = i.SetUpImpersonation()
	return err
}

func (m *manager) deleteServiceAccountImpersonator(username string) error {
	crtbs, err := m.crtbIndexer.ByIndex(rtbByClusterAndUserIndex, m.workload.ClusterName+"-"+username)
	if err != nil {
		return err
	}
	prtbs, err := m.prtbIndexer.ByIndex(rtbByClusterAndUserIndex, m.workload.ClusterName+"-"+username)
	if err != nil {
		return err
	}
	if len(crtbs)+len(prtbs) > 0 {
		return nil
	}
	roleName := impersonation.ImpersonationPrefix + username
	logrus.Debugf("deleting service account impersonator for %s", username)
	err = m.workload.RBAC.ClusterRoles("").Delete(roleName, &metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
