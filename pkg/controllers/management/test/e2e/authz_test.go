package e2e

import (
	"context"
	"testing"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	authzv1 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"gopkg.in/check.v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type AuthzSuite struct {
	extClient     *extclient.Clientset
	clusterClient *clientset.Clientset
	ctx           *config.UserContext
}

var _ = check.Suite(&AuthzSuite{})

func (s *AuthzSuite) TestClusterRoleTemplateBindingCreate(c *check.C) {
	// create project

	// create RoleTemplate (this one will be referenced by the next one)
	podRORoleTemplateName := "testsubcrt1"
	s.clusterClient.RbacV1().ClusterRoles().Delete(context.TODO(), podRORoleTemplateName, metav1.DeleteOptions{})
	subRT, err := s.createRoleTemplate(podRORoleTemplateName,
		[]rbacv1.PolicyRule{
			{
				Verbs:           []string{"get", "list", "watch"},
				APIGroups:       []string{""},
				Resources:       []string{"pods"},
				ResourceNames:   []string{},
				NonResourceURLs: []string{},
			},
		}, []string{}, false, c)
	c.Assert(err, check.IsNil)

	// create RoleTemplate that will reference the first one
	rtName := "testcrt1"
	s.clusterClient.RbacV1().ClusterRoles().Delete(context.TODO(), rtName, metav1.DeleteOptions{})
	rt, err := s.createRoleTemplate(rtName,
		[]rbacv1.PolicyRule{
			{
				Verbs:           []string{"get", "list", "watch"},
				APIGroups:       []string{"apps", "extensions"},
				Resources:       []string{"deployments"},
				ResourceNames:   []string{},
				NonResourceURLs: []string{},
			},
		}, []string{podRORoleTemplateName}, false, c)
	c.Assert(err, check.IsNil)

	// create namespace and watchers for resources in that namespace
	bindingWatcher := s.clusterBindingWatcher(c)
	defer bindingWatcher.Stop()

	// create ProjectRoleTemplateBinding
	subject := rbacv1.Subject{
		Kind: "User",
		Name: "user1",
	}
	binding := s.createCRTBinding("testcbinding1", subject, rtName, c)

	// assert binding is created properly
	newBindings := map[string]bool{}
	roleNames := []string{rt.Name, subRT.Name}
	watchChecker(bindingWatcher, c, func(watchEvent watch.Event) bool {
		if watch.Modified == watchEvent.Type || watch.Added == watchEvent.Type {
			if binding, ok := watchEvent.Object.(*rbacv1.ClusterRoleBinding); ok {
				newBindings[binding.Name] = true
				c.Assert(binding.Subjects[0].Kind, check.Equals, subject.Kind)
				c.Assert(binding.Subjects[0].Name, check.Equals, subject.Name)
				c.Assert(slice.ContainsString(roleNames, binding.RoleRef.Name), check.Equals, true)
				c.Assert(binding.RoleRef.Kind, check.Equals, "ClusterRole")
			}
		}
		return len(newBindings) == 2
	})

	// assert corresponding role is created with all the rules
	rolesExpected := map[string]*authzv1.RoleTemplate{
		subRT.Name: subRT,
		rt.Name:    rt,
	}

	rolesActual := map[string]rbacv1.ClusterRole{}
	rs, err := s.clusterClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	for _, r := range rs.Items {
		if _, ok := rolesExpected[r.Name]; ok {
			rolesActual[r.Name] = r
		}
	}
	c.Assert(len(rolesActual), check.Equals, 2)
	for name, rt := range rolesExpected {
		c.Assert(rolesActual[name].Rules, check.DeepEquals, rt.Rules)
	}

	// Delete the PRTB
	bindingWatcher.Stop()
	bindingWatcher = s.clusterBindingWatcher(c)

	err = s.ctx.Management.Management.ClusterRoleTemplateBindings("").Delete(binding.Name, &metav1.DeleteOptions{})
	c.Assert(err, check.IsNil)

	deletes := map[string]bool{}
	watchChecker(bindingWatcher, c, func(watchEvent watch.Event) bool {
		if watch.Deleted == watchEvent.Type {
			if binding, ok := watchEvent.Object.(*rbacv1.ClusterRoleBinding); ok {
				deletes[binding.Name] = true
			}
		}
		return len(deletes) == 2
	})
}

func (s *AuthzSuite) TestRoleTemplateBindingCreate(c *check.C) {
	// create project
	projectName := "testproject1"

	// create RoleTemplate (this one will be referenced by the next one)
	podRORoleTemplateName := "testsubrt1"
	s.clusterClient.RbacV1().ClusterRoles().Delete(context.TODO(), podRORoleTemplateName, metav1.DeleteOptions{})
	subRT, err := s.createRoleTemplate(podRORoleTemplateName,
		[]rbacv1.PolicyRule{
			{
				Verbs:           []string{"get", "list", "watch"},
				APIGroups:       []string{""},
				Resources:       []string{"pods"},
				ResourceNames:   []string{},
				NonResourceURLs: []string{},
			},
		}, []string{}, false, c)
	c.Assert(err, check.IsNil)

	// create RoleTemplate that will reference the first one
	rtName := "testrt1"
	s.clusterClient.RbacV1().ClusterRoles().Delete(context.TODO(), rtName, metav1.DeleteOptions{})
	rt, err := s.createRoleTemplate(rtName,
		[]rbacv1.PolicyRule{
			{
				Verbs:           []string{"get", "list", "watch"},
				APIGroups:       []string{"apps", "extensions"},
				Resources:       []string{"deployments"},
				ResourceNames:   []string{},
				NonResourceURLs: []string{},
			},
		}, []string{podRORoleTemplateName}, false, c)
	c.Assert(err, check.IsNil)

	// create namespace and watchers for resources in that namespace
	ns := setupNS("testauthzns1", projectName, s.clusterClient.CoreV1().Namespaces(), c)
	defer deleteNSOnPass(ns.Name, s.clusterClient.CoreV1().Namespaces(), c)
	bindingWatcher := s.bindingWatcher(ns.Name, c)
	defer bindingWatcher.Stop()

	// create ProjectRoleTemplateBinding
	subject := rbacv1.Subject{
		Kind: "User",
		Name: "user1",
	}
	binding := s.createPRTBinding("testbinding1", subject, projectName, rtName, c)

	// assert binding is created properly
	newBindings := map[string]bool{}
	roleNames := []string{rt.Name, subRT.Name}
	watchChecker(bindingWatcher, c, func(watchEvent watch.Event) bool {
		if watch.Modified == watchEvent.Type || watch.Added == watchEvent.Type {
			if binding, ok := watchEvent.Object.(*rbacv1.RoleBinding); ok {
				newBindings[binding.Name] = true
				c.Assert(binding.Subjects[0].Kind, check.Equals, subject.Kind)
				c.Assert(binding.Subjects[0].Name, check.Equals, subject.Name)
				c.Assert(slice.ContainsString(roleNames, binding.RoleRef.Name), check.Equals, true)
				c.Assert(binding.RoleRef.Kind, check.Equals, "ClusterRole")
			}
		}
		return len(newBindings) == 2
	})

	// assert corresponding role is created with all the rules
	rolesExpected := map[string]*authzv1.RoleTemplate{
		subRT.Name: subRT,
		rt.Name:    rt,
	}

	rolesActual := map[string]rbacv1.ClusterRole{}
	rs, err := s.clusterClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	for _, r := range rs.Items {
		if _, ok := rolesExpected[r.Name]; ok {
			rolesActual[r.Name] = r
		}
	}
	c.Assert(len(rolesActual), check.Equals, 2)
	for name, rt := range rolesExpected {
		c.Assert(rolesActual[name].Rules, check.DeepEquals, rt.Rules)
	}

	// Delete the PRTB
	bindingWatcher.Stop()
	bindingWatcher = s.bindingWatcher(ns.Name, c)

	err = s.ctx.Management.Management.ProjectRoleTemplateBindings("default").Delete(binding.Name, &metav1.DeleteOptions{})
	c.Assert(err, check.IsNil)

	deletes := map[string]bool{}
	watchChecker(bindingWatcher, c, func(watchEvent watch.Event) bool {
		if watch.Deleted == watchEvent.Type {
			if binding, ok := watchEvent.Object.(*rbacv1.RoleBinding); ok {
				deletes[binding.Name] = true
			}
		}
		return len(deletes) == 2
	})
}

func (s *AuthzSuite) TestBuiltinRoleTemplateBindingCreate(c *check.C) {
	// create project
	projectName := "testproject2"

	// create RoleTemplate that user will be bound to
	rtName := "testrt2"
	_, err := s.createRoleTemplate(rtName,
		[]rbacv1.PolicyRule{}, []string{}, true, c)
	c.Assert(err, check.IsNil)

	// create namespace and watchers for resources in that namespace
	ns := setupNS("testauthzbuiltinns1", projectName, s.clusterClient.CoreV1().Namespaces(), c)
	defer deleteNSOnPass(ns.Name, s.clusterClient.CoreV1().Namespaces(), c)
	bindingWatcher := s.bindingWatcher(ns.Name, c)
	defer bindingWatcher.Stop()

	roles, err := s.clusterClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	rolesPreCount := len(roles.Items)

	// create ProjectRoleTemplateBinding
	subject := rbacv1.Subject{
		Kind: "User",
		Name: "user1",
	}
	binding := s.createPRTBinding("testbuiltin1", subject, projectName, rtName, c)

	// assert binding is created properly
	watchChecker(bindingWatcher, c, func(watchEvent watch.Event) bool {
		if watch.Modified == watchEvent.Type || watch.Added == watchEvent.Type {
			if binding, ok := watchEvent.Object.(*rbacv1.RoleBinding); ok {
				c.Assert(binding.Subjects[0].Kind, check.Equals, subject.Kind)
				c.Assert(binding.Subjects[0].Name, check.Equals, subject.Name)
				c.Assert(binding.RoleRef.Name, check.Equals, rtName)
				c.Assert(binding.RoleRef.Kind, check.Equals, "ClusterRole")
				return true
			}
		}
		return false
	})

	// ensure no new roles were created in the namespace
	roles, err = s.clusterClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	rolesPostCount := len(roles.Items)
	c.Assert(rolesPostCount, check.Equals, rolesPreCount)

	// Delete the PRTB
	bindingWatcher.Stop()
	bindingWatcher = s.bindingWatcher(ns.Name, c)

	err = s.ctx.Management.Management.ProjectRoleTemplateBindings("default").Delete(binding.Name, &metav1.DeleteOptions{})
	c.Assert(err, check.IsNil)

	watchChecker(bindingWatcher, c, func(watchEvent watch.Event) bool {
		return watch.Deleted == watchEvent.Type
	})
}

func (s *AuthzSuite) createCRTBinding(bindingName string, subject rbacv1.Subject, rtName string, c *check.C) *authzv1.ClusterRoleTemplateBinding {
	binding, err := s.ctx.Management.Management.ClusterRoleTemplateBindings("").Create(&authzv1.ClusterRoleTemplateBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleTemplateBinding",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		UserName:         subject.Name,
		RoleTemplateName: rtName,
	})

	c.Assert(err, check.IsNil)
	c.Assert(binding.Name, check.Equals, bindingName)
	return binding
}

func (s *AuthzSuite) createPRTBinding(bindingName string, subject rbacv1.Subject, projectName string, rtName string, c *check.C) *authzv1.ProjectRoleTemplateBinding {
	binding, err := s.ctx.Management.Management.ProjectRoleTemplateBindings("default").Create(&authzv1.ProjectRoleTemplateBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ProjectRoleTemplateBinding",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		UserName:         subject.Name,
		ProjectName:      projectName,
		RoleTemplateName: rtName,
	})

	c.Assert(err, check.IsNil)
	c.Assert(binding.Name, check.Equals, bindingName)
	return binding
}

func (s *AuthzSuite) createRoleTemplate(name string, rules []rbacv1.PolicyRule, prts []string, builtin bool, c *check.C) (*authzv1.RoleTemplate, error) {
	rt, err := s.ctx.Management.Management.RoleTemplates("").Create(&authzv1.RoleTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleTemplate",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules:             rules,
		RoleTemplateNames: prts,
		Builtin:           builtin,
	})
	c.Assert(err, check.IsNil)
	c.Assert(rt.Name, check.Equals, name)
	return rt, err
}

func (s *AuthzSuite) pspWatcher(c *check.C) watch.Interface {
	pspClient := s.clusterClient.ExtensionsV1beta1().PodSecurityPolicies()
	pList, err := pspClient.List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	pListMeta, err := meta.ListAccessor(pList)
	c.Assert(err, check.IsNil)
	pspWatch, err := pspClient.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: pListMeta.GetResourceVersion()})
	c.Assert(err, check.IsNil)
	return pspWatch
}

func (s *AuthzSuite) clusterBindingWatcher(c *check.C) watch.Interface {
	bindingClient := s.clusterClient.RbacV1().ClusterRoleBindings()
	bList, err := bindingClient.List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	bListMeta, err := meta.ListAccessor(bList)
	c.Assert(err, check.IsNil)
	bindingWatch, err := bindingClient.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: bListMeta.GetResourceVersion()})
	c.Assert(err, check.IsNil)
	return bindingWatch
}

func (s *AuthzSuite) bindingWatcher(namespace string, c *check.C) watch.Interface {
	bindingClient := s.clusterClient.RbacV1().RoleBindings(namespace)
	bList, err := bindingClient.List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	bListMeta, err := meta.ListAccessor(bList)
	c.Assert(err, check.IsNil)
	bindingWatch, err := bindingClient.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: bListMeta.GetResourceVersion()})
	c.Assert(err, check.IsNil)
	return bindingWatch
}

func (s *AuthzSuite) roleWatcher(c *check.C) watch.Interface {
	roleClient := s.clusterClient.RbacV1().ClusterRoles()
	initialList, err := roleClient.List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)
	initialListListMeta, err := meta.ListAccessor(initialList)
	c.Assert(err, check.IsNil)
	roleWatch, err := roleClient.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: initialListListMeta.GetResourceVersion()})
	c.Assert(err, check.IsNil)
	return roleWatch
}

func (s *AuthzSuite) SetUpSuite(c *check.C) {
	c.Skip("Environments not configured for client setup: TEST_CLUSTER_CONFIG is missing")
	clusterClient, extClient, workload := clientForSetup(c)
	s.extClient = extClient
	s.clusterClient = clusterClient
	s.ctx = workload
	s.setupCRDs(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rbac.Register(ctx, workload)

	err := workload.Start(ctx)
	c.Assert(err, check.IsNil)
	err = workload.Management.Start(ctx)
	c.Assert(err, check.IsNil)

}

func (s *AuthzSuite) setupCRDs(c *check.C) {
	crdClient := s.extClient.ApiextensionsV1beta1().CustomResourceDefinitions()

	initialList, err := crdClient.List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, check.IsNil)

	initialListListMeta, err := meta.ListAccessor(initialList)
	c.Assert(err, check.IsNil)

	crdWatch, err := crdClient.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: initialListListMeta.GetResourceVersion()})
	c.Assert(err, check.IsNil)
	defer crdWatch.Stop()

	setupCRD("roletemplate", "roletemplates", "management.cattle.io", "RoleTemplate", "v3",
		apiextensionsv1beta1.ClusterScoped, crdClient, crdWatch, c)

	setupCRD("projectroletemplatebinding", "projectroletemplatebindings", "management.cattle.io", "ProjectRoleTemplateBinding", "v3",
		apiextensionsv1beta1.ClusterScoped, crdClient, crdWatch, c)

	setupCRD("clusterroletemplatebinding", "clusterroletemplatebindings", "management.cattle.io", "ClusterRoleTemplateBinding", "v3",
		apiextensionsv1beta1.ClusterScoped, crdClient, crdWatch, c)

	setupCRD("podsecuritypolicytemplate", "podsecuritypolicytemplates", "management.cattle.io", "PodSecurityPolicyTemplates", "v3",
		apiextensionsv1beta1.ClusterScoped, crdClient, crdWatch, c)
}
