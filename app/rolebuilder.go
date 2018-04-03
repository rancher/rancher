package app

import (
	"fmt"

	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var defaultGRLabel = map[string]string{"authz.management.cattle.io/bootstrapping": "default-globalrole"}
var defaultRTLabel = map[string]string{"authz.management.cattle.io/bootstrapping": "default-roletemplate"}

type roleBuilder struct {
	previous          *roleBuilder
	next              *roleBuilder
	name              string
	displayName       string
	context           string
	builtin           bool
	external          bool
	hidden            bool
	roleTemplateNames []string
	rules             []*ruleBuilder
}

func (rb *roleBuilder) String() string {
	return fmt.Sprintf("%s (%s): %s", rb.displayName, rb.name, rb.rules)
}

func newRoleBuilder() *roleBuilder {
	return &roleBuilder{}
}

func (rb *roleBuilder) addRoleTemplate(displayName, name, context string, builtin, external, hidden bool) *roleBuilder {
	r := rb.addRole(displayName, name)
	r.context = context
	r.builtin = builtin
	r.external = external
	r.hidden = hidden
	return r
}

func (rb *roleBuilder) addRole(displayName, name string) *roleBuilder {
	if rb.name == "" {
		rb.name = name
		rb.displayName = displayName
		return rb
	}

	if rb.next != nil {
		return rb.next.addRole(displayName, name)
	}
	rb.next = &roleBuilder{
		name:        name,
		displayName: displayName,
		previous:    rb,
	}
	return rb.next
}

func (rb *roleBuilder) addRule() *ruleBuilder {
	r := &ruleBuilder{
		rb: rb,
	}
	rb.rules = append(rb.rules, r)
	return r
}

func (rb *roleBuilder) setRoleTemplateNames(names ...string) *roleBuilder {
	rb.roleTemplateNames = names
	return rb
}

func (rb *roleBuilder) first() *roleBuilder {
	if rb.previous == nil {
		return rb
	}
	return rb.previous.first()
}

func (rb *roleBuilder) policyRules() []rbacv1.PolicyRule {
	var prs []rbacv1.PolicyRule
	for _, r := range rb.rules {
		prs = append(prs, r.toPolicyRule())
	}
	return prs
}

type ruleBuilder struct {
	rb             *roleBuilder
	_verbs         []string
	_resources     []string
	_resourceNames []string
	_apiGroups     []string
	_urls          []string
}

func (r *ruleBuilder) String() string {
	return fmt.Sprintf("apigroups: %v, resource: %v, resourceNames: %v, nonResourceURLs %v, verbs: %v", r._apiGroups, r._resources, r._resourceNames, r._urls, r._verbs)
}

func (r *ruleBuilder) verbs(v ...string) *ruleBuilder {
	r._verbs = append(r._verbs, v...)
	return r
}

func (r *ruleBuilder) resources(rc ...string) *ruleBuilder {
	r._resources = append(r._resources, rc...)
	return r
}

func (r *ruleBuilder) resourceNames(rn ...string) *ruleBuilder {
	r._resourceNames = append(r._resourceNames, rn...)
	return r
}

func (r *ruleBuilder) apiGroups(a ...string) *ruleBuilder {
	r._apiGroups = append(r._apiGroups, a...)
	return r
}

func (r *ruleBuilder) nonResourceURLs(u ...string) *ruleBuilder {
	r._urls = append(r._urls, u...)
	return r
}

func (r *ruleBuilder) addRoleTemplate(diplsayName, name, context string, builtin, external, hidden bool) *roleBuilder {
	return r.rb.addRoleTemplate(diplsayName, name, context, builtin, external, hidden)
}

func (r *ruleBuilder) addRole(diplsayName, name string) *roleBuilder {
	return r.rb.addRole(diplsayName, name)
}

func (r *ruleBuilder) addRule() *ruleBuilder {
	return r.rb.addRule()
}

func (r *ruleBuilder) setRoleTemplateNames(names ...string) *roleBuilder {
	return r.rb.setRoleTemplateNames(names...)
}

func (r *ruleBuilder) reconcileGlobalRoles(mgmt *config.ManagementContext) error {
	return r.rb.reconcileGlobalRoles(mgmt)
}

func (r *ruleBuilder) reconcileRoleTemplates(mgmt *config.ManagementContext) error {
	return r.rb.reconcileRoleTemplates(mgmt)
}

func (r *ruleBuilder) toPolicyRule() rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups:       r._apiGroups,
		Resources:       r._resources,
		ResourceNames:   r._resourceNames,
		NonResourceURLs: r._urls,
		Verbs:           r._verbs,
	}
}

type buildFnc func(thisRB *roleBuilder) (string, runtime.Object)
type compareAndModifyFnc func(have runtime.Object, want runtime.Object) (bool, runtime.Object, error)
type gatherExistingFnc func() (map[string]runtime.Object, error)

func (rb *roleBuilder) reconcile(build buildFnc, gatherExisting gatherExistingFnc, compareAndModify compareAndModifyFnc, client *objectclient.ObjectClient) error {
	current := rb.first()
	builtRoles := map[string]runtime.Object{}
	for current != nil {
		name, role := build(current)
		builtRoles[name] = role
		current = current.next
	}

	existing, err := gatherExisting()
	if err != nil {
		return err
	}

	for name := range existing {
		if _, ok := builtRoles[name]; !ok {
			logrus.Infof("Remvoing %v", name)
			if err := client.Delete(name, &v1.DeleteOptions{}); err != nil {
				return errors.Wrapf(err, "couldn't delete %v", name)
			}
			delete(existing, name)
		}
	}

	for name, gr := range builtRoles {
		if existingCR, ok := existing[name]; ok {
			equal, modified, err := compareAndModify(existingCR, gr)
			if err != nil {
				return err
			}
			if !equal {
				if _, err := client.Update(name, modified); err != nil {
					return errors.Wrapf(err, "couldn't update %v", name)
				}
			}
			continue
		}

		logrus.Infof("Creating %v", name)
		if _, err := client.Create(gr); err != nil {
			return errors.Wrapf(err, "couldn't create %v", name)
		}
	}

	return nil
}

func (rb *roleBuilder) reconcileGlobalRoles(mgmt *config.ManagementContext) error {
	logrus.Info("Reconciling GlobalRoles")

	build := func(current *roleBuilder) (string, runtime.Object) {
		gr := &v3.GlobalRole{
			ObjectMeta: v1.ObjectMeta{
				Name:   current.name,
				Labels: defaultGRLabel,
			},
			DisplayName: current.displayName,
			Rules:       current.policyRules(),
		}
		return gr.Name, gr
	}

	grCli := mgmt.Management.GlobalRoles("")
	gather := func() (map[string]runtime.Object, error) {
		set := labels.Set(defaultGRLabel)
		existingList, err := grCli.List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't list globalRoles with selector %s", set)
		}

		existing := map[string]runtime.Object{}
		for _, e := range existingList.Items {
			existing[e.Name] = e.DeepCopy()
		}

		return existing, nil
	}

	compareAndMod := func(have runtime.Object, want runtime.Object) (bool, runtime.Object, error) {
		haveGR, ok := have.(*v3.GlobalRole)
		wantGR, ok2 := want.(*v3.GlobalRole)
		if !ok || !ok2 {
			return false, nil, errors.Errorf("unexpected type comparing %v and %v", have, want)
		}

		equal := haveGR.DisplayName == wantGR.DisplayName && reflect.DeepEqual(haveGR.Rules, wantGR.Rules)

		haveGR.DisplayName = wantGR.DisplayName
		haveGR.Rules = wantGR.Rules

		return equal, haveGR, nil
	}

	return rb.reconcile(build, gather, compareAndMod, grCli.ObjectClient())
}

func (rb *roleBuilder) reconcileRoleTemplates(mgmt *config.ManagementContext) error {
	logrus.Info("Reconciling RoleTemplates")

	build := func(current *roleBuilder) (string, runtime.Object) {
		role := &v3.RoleTemplate{
			ObjectMeta: v1.ObjectMeta{
				Name:   current.name,
				Labels: defaultRTLabel,
			},
			DisplayName:       current.displayName,
			Builtin:           current.builtin,
			External:          current.external,
			Hidden:            current.hidden,
			Context:           current.context,
			Rules:             current.policyRules(),
			RoleTemplateNames: current.roleTemplateNames,
		}
		return role.Name, role
	}

	client := mgmt.Management.RoleTemplates("")
	gather := func() (map[string]runtime.Object, error) {
		set := labels.Set(defaultRTLabel)
		existingList, err := client.List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't list roleTemplate with selector %s", set)
		}

		existing := map[string]runtime.Object{}
		for _, e := range existingList.Items {
			existing[e.Name] = e.DeepCopy()
		}

		return existing, nil
	}

	compareAndMod := func(have runtime.Object, want runtime.Object) (bool, runtime.Object, error) {
		haveRT, ok := have.(*v3.RoleTemplate)
		wantRT, ok2 := want.(*v3.RoleTemplate)
		if !ok || !ok2 {
			return false, nil, errors.Errorf("unexpected type comparing %v and %v", have, want)
		}

		equal := haveRT.DisplayName == wantRT.DisplayName && reflect.DeepEqual(haveRT.Rules, wantRT.Rules) &&
			reflect.DeepEqual(haveRT.RoleTemplateNames, wantRT.RoleTemplateNames) && haveRT.Builtin != wantRT.Builtin &&
			haveRT.External == wantRT.External && haveRT.Hidden == wantRT.Hidden && haveRT.Context == wantRT.Context

		haveRT.DisplayName = wantRT.DisplayName
		haveRT.Rules = wantRT.Rules
		haveRT.RoleTemplateNames = wantRT.RoleTemplateNames
		haveRT.Builtin = wantRT.Builtin
		haveRT.External = wantRT.External
		haveRT.Hidden = wantRT.Hidden
		haveRT.Context = wantRT.Context

		return equal, haveRT, nil
	}

	return rb.reconcile(build, gather, compareAndMod, client.ObjectClient())
}
