package integration

import (
	"context"

	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	extunstructured "github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/api/scheme"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	authzv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// checkAccessAllowed performs a single SelfSubjectAccessReview and returns whether access is allowed.
func checkAccessAllowed(client *rancher.Client, clusterID string, attr *authzv1.ResourceAttributes) (bool, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return false, err
	}

	ssar := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: attr,
		},
	}

	ssarGVR := authzv1.SchemeGroupVersion.WithResource("selfsubjectaccessreviews")
	resp, err := dynamicClient.Resource(ssarGVR).Create(context.TODO(), extunstructured.MustToUnstructured(ssar), metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	result := &authzv1.SelfSubjectAccessReview{}
	if err := scheme.Scheme.Convert(resp, result, resp.GroupVersionKind()); err != nil {
		return false, err
	}

	return result.Status.Allowed, nil
}

func (p *RTBTestSuite) TestImpersonationByClusterRole() {
	client := p.newSubSession()

	// Create user1 with standard "user" role.
	user1 := p.createUser(client, "imp-user1", "user")

	// Create user2 with standard "user" role.
	user2 := p.createUser(client, "imp-user2", "user")

	localCluster, err := client.Management.Cluster.ByID(p.downstreamClusterID)
	p.Require().NoError(err)

	// Give user1 cluster-member and user2 cluster-owner.
	err = users.AddClusterRoleToUser(client, localCluster, user1, "cluster-member", nil)
	p.Require().NoError(err)

	err = users.AddClusterRoleToUser(client, localCluster, user2, "cluster-owner", nil)
	p.Require().NoError(err)

	user1Client, err := client.AsUser(user1)
	p.Require().NoError(err)

	user2Client, err := client.AsUser(user2)
	p.Require().NoError(err)

	impersonateAttr := &authzv1.ResourceAttributes{
		Verb:     "impersonate",
		Resource: "users",
		Group:    "",
	}

	// Admin can always impersonate.
	err = extauthz.WaitForAllowed(client, p.downstreamClusterID, []*authzv1.ResourceAttributes{impersonateAttr})
	p.Require().NoError(err)

	// User1 is a cluster-member which does not grant impersonate.
	allowed, err := checkAccessAllowed(user1Client, p.downstreamClusterID, impersonateAttr)
	p.Require().NoError(err)
	p.Require().False(allowed, "cluster-member should not be able to impersonate")

	// User2 is a cluster-owner which allows impersonation.
	err = extauthz.WaitForAllowed(user2Client, p.downstreamClusterID, []*authzv1.ResourceAttributes{impersonateAttr})
	p.Require().NoError(err)

	// Create a ClusterRole allowing limited impersonation of user2 only.
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	impRoleName := namegen.AppendRandomString("limited-impersonator-")
	impRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: impRoleName},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"users"},
				Verbs:         []string{"impersonate"},
				ResourceNames: []string{user2.ID},
			},
		},
	}

	crResource := dynamicClient.Resource(extrbac.ClusterRoleGroupVersionResource)

	var cr unstructured.Unstructured
	err = scheme.Scheme.Convert(impRole, &cr, nil)
	p.Require().NoError(err)

	_, err = crResource.Create(context.TODO(), &cr, metav1.CreateOptions{})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		err := crResource.Delete(context.TODO(), impRoleName, metav1.DeleteOptions{})
		p.Require().NoError(err)
	})

	// Create a ClusterRoleBinding binding user1 to the impersonation role.
	impBindingName := namegen.AppendRandomString("limited-impersonator-binding-")
	impBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: impBindingName},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: user1.ID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     impRoleName,
		},
	}

	crbResource := dynamicClient.Resource(extrbac.ClusterRoleBindingGroupVersionResource)
	var crb unstructured.Unstructured
	err = scheme.Scheme.Convert(impBinding, &crb, nil)
	p.Require().NoError(err)

	_, err = crbResource.Create(context.TODO(), &crb, metav1.CreateOptions{})
	p.Require().NoError(err)
	p.T().Cleanup(func() {
		err := crbResource.Delete(context.TODO(), impBindingName, metav1.DeleteOptions{})
		p.Require().NoError(err)
	})

	// User1 should now be able to impersonate user2 specifically.
	err = extauthz.WaitForAllowed(user1Client, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "impersonate",
			Resource: "users",
			Group:    "",
			Name:     user2.ID,
		},
	})
	p.Require().NoError(err)
}
