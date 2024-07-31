package management

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	administrative    bool
	roleTemplateNames []string
	rules             []*ruleBuilder
	externalRules     []*ruleBuilder
}

func (rb *roleBuilder) String() string {
	return fmt.Sprintf("%s (%s): %s", rb.displayName, rb.name, rb.rules)
}

func newRoleBuilder() *roleBuilder {
	return &roleBuilder{
		builtin: true,
	}
}

func (rb *roleBuilder) addRoleTemplate(displayName, name, context string, external, hidden, administrative bool) *roleBuilder {
	r := rb.addRole(displayName, name)
	r.context = context
	r.external = external
	r.hidden = hidden
	r.administrative = administrative
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
	rb.next = newRoleBuilder()
	rb.next.name = name
	rb.next.displayName = displayName
	rb.next.previous = rb

	return rb.next
}

func (rb *roleBuilder) addRule() *ruleBuilder {
	r := &ruleBuilder{
		rb: rb,
	}
	rb.rules = append(rb.rules, r)
	return r
}

func (rb *roleBuilder) addExternalRule() *ruleBuilder {
	r := &ruleBuilder{
		rb: rb,
	}
	rb.externalRules = append(rb.externalRules, r)
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

func (rb *roleBuilder) policyExternalRules() []rbacv1.PolicyRule {
	var prs []rbacv1.PolicyRule
	for _, r := range rb.externalRules {
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

func (r *ruleBuilder) addRoleTemplate(displayName, name, context string, external, hidden, administrative bool) *roleBuilder {
	return r.rb.addRoleTemplate(displayName, name, context, external, hidden, administrative)
}

func (r *ruleBuilder) addRole(displayName, name string) *roleBuilder {
	return r.rb.addRole(displayName, name)
}

func (r *ruleBuilder) addRule() *ruleBuilder {
	return r.rb.addRule()
}

func (r *ruleBuilder) addExternalRule() *ruleBuilder {
	return r.rb.addExternalRule()
}

func (r *ruleBuilder) setRoleTemplateNames(names ...string) *roleBuilder {
	return r.rb.setRoleTemplateNames(names...)
}

func (r *ruleBuilder) reconcileGlobalRoles(grClient wranglerv3.GlobalRoleClient) error {
	return r.rb.reconcileGlobalRoles(grClient)
}

func (r *ruleBuilder) reconcileRoleTemplates(rtClient wranglerv3.RoleTemplateClient) error {
	return r.rb.reconcileRoleTemplates(rtClient)
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

// buildFnc takes in a roleBuilder and returns desired runtime.Objects.
type buildFnc[T runtime.Object] func(thisRB *roleBuilder) (name string, object T, err error)

// compareAndModifyFnc check two objects and returns if they are different as well as the desired object to create.
// haveObj will be mutated and returned contain the desired values from the wantObj.
type compareAndModifyFnc[T runtime.Object] func(haveObj T, wantObj T) (isDifferent bool, desiredObject T, err error)

// gatherExistingFnc returns a map of objects that already exist using the object name as the key.
type gatherExistingFnc[T runtime.Object] func() (ObjectNameMap map[string]T, err error)

func reconcile[T generic.RuntimeMetaObject, TList runtime.Object](
	rb *roleBuilder, build buildFnc[T], gatherExisting gatherExistingFnc[T],
	compareAndModify compareAndModifyFnc[T], client generic.NonNamespacedClientInterface[T, TList]) error {
	current := rb.first()
	builtRoles := map[string]T{}
	for current != nil {
		name, role, err := build(current)
		if err != nil {
			return errors.Wrapf(err, "couldn't create %v", name)
		}
		if name != "" {
			builtRoles[name] = role
		}
		current = current.next
	}

	existing, err := gatherExisting()
	if err != nil {
		return err
	}

	for name := range existing {
		if _, ok := builtRoles[name]; !ok {
			logrus.Infof("Removing %v", name)
			if err := client.Delete(name, nil); err != nil {
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
				if _, err := client.Update(modified); err != nil {
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

func (rb *roleBuilder) reconcileGlobalRoles(grClient wranglerv3.GlobalRoleClient) error {
	logrus.Info("Reconciling GlobalRoles")
	build := func(current *roleBuilder) (string, *v3.GlobalRole, error) {
		gr := &v3.GlobalRole{
			ObjectMeta: v1.ObjectMeta{
				Name:   current.name,
				Labels: defaultGRLabel,
			},
			DisplayName: current.displayName,
			Rules:       current.policyRules(),
			Builtin:     current.builtin,
		}
		return gr.Name, gr, nil
	}

	gather := func() (map[string]*v3.GlobalRole, error) {
		set := labels.Set(defaultGRLabel)
		existingList, err := grClient.List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't list globalRoles with selector %s", set)
		}

		existing := map[string]*v3.GlobalRole{}
		for _, e := range existingList.Items {
			existing[e.Name] = e.DeepCopy()
		}

		return existing, nil
	}

	compareAndMod := func(haveGR *v3.GlobalRole, wantGR *v3.GlobalRole) (bool, *v3.GlobalRole, error) {
		builtin := haveGR.Builtin == wantGR.Builtin
		equal := builtin && haveGR.DisplayName == wantGR.DisplayName && reflect.DeepEqual(haveGR.Rules, wantGR.Rules)

		haveGR.DisplayName = wantGR.DisplayName
		haveGR.Rules = wantGR.Rules
		haveGR.Builtin = wantGR.Builtin

		return equal, haveGR, nil
	}

	// create a new client that impersonates the webhook to bypass field validation that would normally block updating builtin roles
	bypassClient, err := grClient.WithImpersonation(controllers.WebhookImpersonation())
	if err != nil {
		return fmt.Errorf("failed to make impersonation client: %w", err)
	}
	return reconcile(rb, build, gather, compareAndMod, bypassClient)
}

func (rb *roleBuilder) reconcileRoleTemplates(rtClient wranglerv3.RoleTemplateClient) error {
	logrus.Info("Reconciling RoleTemplates")
	build := func(current *roleBuilder) (string, *v3.RoleTemplate, error) {
		if current.externalRules != nil && !current.external {
			return "", nil, fmt.Errorf("can't create RoleTemplate with externalRules and external=false")
		}
		role := &v3.RoleTemplate{
			ObjectMeta: v1.ObjectMeta{
				Name:   current.name,
				Labels: defaultRTLabel,
			},
			DisplayName:       current.displayName,
			Builtin:           current.builtin,
			External:          current.external,
			ExternalRules:     current.policyExternalRules(),
			Hidden:            current.hidden,
			Context:           current.context,
			Rules:             current.policyRules(),
			RoleTemplateNames: current.roleTemplateNames,
			Administrative:    current.administrative,
		}
		return role.Name, role, nil
	}

	gather := func() (map[string]*v3.RoleTemplate, error) {
		set := labels.Set(defaultRTLabel)
		existingList, err := rtClient.List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't list roleTemplate with selector %s", set)
		}

		existing := map[string]*v3.RoleTemplate{}
		for _, e := range existingList.Items {
			existing[e.Name] = e.DeepCopy()
		}

		return existing, nil
	}

	compareAndMod := func(haveRT *v3.RoleTemplate, wantRT *v3.RoleTemplate) (bool, *v3.RoleTemplate, error) {
		equal := haveRT.DisplayName == wantRT.DisplayName && reflect.DeepEqual(haveRT.Rules, wantRT.Rules) && reflect.DeepEqual(haveRT.ExternalRules, wantRT.ExternalRules) &&
			reflect.DeepEqual(haveRT.RoleTemplateNames, wantRT.RoleTemplateNames) && haveRT.Builtin == wantRT.Builtin &&
			haveRT.External == wantRT.External && haveRT.Hidden == wantRT.Hidden && haveRT.Context == wantRT.Context &&
			haveRT.Administrative == wantRT.Administrative

		haveRT.DisplayName = wantRT.DisplayName
		haveRT.Rules = wantRT.Rules
		haveRT.ExternalRules = wantRT.ExternalRules
		haveRT.RoleTemplateNames = wantRT.RoleTemplateNames
		haveRT.Builtin = wantRT.Builtin
		haveRT.External = wantRT.External
		haveRT.Hidden = wantRT.Hidden
		haveRT.Context = wantRT.Context
		haveRT.Administrative = wantRT.Administrative

		return equal, haveRT, nil
	}

	// create a new client that impersonates the webhook to bypass field validation that would normally block updating builtin roles
	bypassClient, err := rtClient.WithImpersonation(controllers.WebhookImpersonation())
	if err != nil {
		return fmt.Errorf("failed to make impersonation client: %w", err)
	}
	return reconcile(rb, build, gather, compareAndMod, bypassClient)
}
