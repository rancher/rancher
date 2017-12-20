package app

import (
	"fmt"

	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var roleBootstrapLabel = map[string]string{"management.cattle.io/bootstrapping": "rbac-defaults"}

type roleBuilder struct {
	previous *roleBuilder
	next     *roleBuilder
	name     string
	rules    []*ruleBuilder
}

func (rb *roleBuilder) String() string {
	return fmt.Sprintf("%s: %s", rb.name, rb.rules)
}

type ruleBuilder struct {
	name           string
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

func (r *ruleBuilder) addRole(name string) *roleBuilder {
	return r.rb.addRole(name)
}

func (r *ruleBuilder) addRule() *ruleBuilder {
	return r.rb.addRule()
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
func newRoleBuilder() *roleBuilder {
	return &roleBuilder{}
}

func (rb *roleBuilder) addRole(name string) *roleBuilder {
	if rb.name == "" {
		rb.name = name
		return rb
	}

	if rb.next != nil {
		return rb.next.addRole(name)
	}
	rb.next = &roleBuilder{
		name:     name,
		previous: rb,
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

func (rb *roleBuilder) reconcileGlobalRoles(mgmt *config.ManagementContext) error {
	logrus.Info("Reconciling GlobalRoles")
	globalRoles := map[string]*v3.GlobalRole{}
	current := rb.first()
	for current != nil {
		logrus.Debugf("Converting role %s to GlobalRole", current)
		cr := &v3.GlobalRole{
			ObjectMeta: v1.ObjectMeta{
				Name:   current.name,
				Labels: roleBootstrapLabel,
			},
			Rules: current.policyRules(),
		}
		globalRoles[cr.Name] = cr
		current = current.next
	}

	grCli := mgmt.Management.GlobalRoles("")
	set := labels.Set(roleBootstrapLabel)
	existingList, err := grCli.List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return errors.Wrapf(err, "couldn't list bootstrap clusterRoles with selector %s", set)
	}
	existing := map[string]v3.GlobalRole{}
	for _, cr := range existingList.Items {
		existing[cr.Name] = cr
	}

	for name := range existing {
		if _, ok := globalRoles[name]; !ok {
			logrus.Infof("Remvoing GlobalRole %v", name)
			if err := grCli.Delete(name, &v1.DeleteOptions{}); err != nil {
				return errors.Wrapf(err, "couldn't delete GlobalRole %v", name)
			}
			delete(existing, name)
		}
	}

	for name, gr := range globalRoles {
		if existingCR, ok := existing[name]; ok {
			if !reflect.DeepEqual(gr.Rules, existingCR.Rules) {
				logrus.Infof("Updating GlobalRole %v. Rules have changed. Have: %+v. Want: %+v", name, existingCR.Rules, gr.Rules)
				existingCR.Rules = gr.Rules
				if _, err := grCli.Update(&existingCR); err != nil {
					return errors.Wrapf(err, "couldn't update GlobalRole %v", name)
				}
			}
			continue
		}
		logrus.Infof("Creating new GlobalRole %v", name)
		if _, err := grCli.Create(gr); err != nil {
			return errors.Wrapf(err, "couldn't create GlobalRole %v", name)
		}
	}

	return nil
}

func (rb *roleBuilder) policyRules() []rbacv1.PolicyRule {
	prs := []rbacv1.PolicyRule{}
	for _, r := range rb.rules {
		prs = append(prs, r.toPolicyRule())
	}
	return prs
}

func (rb *roleBuilder) reconcileRoleTemplates(mgmt *config.ManagementContext) error {
	return nil
}

func (rb *roleBuilder) first() *roleBuilder {
	if rb.previous == nil {
		return rb
	}
	return rb.previous.first()
}
