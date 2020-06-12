package accesscontrol

import (
	"sort"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

type AccessSet struct {
	ID  string
	set map[key]resourceAccessSet
}

type resourceAccessSet map[Access]bool

type key struct {
	verb string
	gr   schema.GroupResource
}

func (a *AccessSet) Namespaces() (result []string) {
	set := map[string]bool{}
	for k, as := range a.set {
		if k.verb != "get" && k.verb != "list" {
			continue
		}
		for access := range as {
			if access.Namespace == All {
				continue
			}
			set[access.Namespace] = true
		}
	}
	for k := range set {
		result = append(result, k)
	}
	sort.Strings(result)
	return
}

func (a *AccessSet) Merge(right *AccessSet) {
	for k, accessMap := range right.set {
		m, ok := a.set[k]
		if !ok {
			m = map[Access]bool{}
			if a.set == nil {
				a.set = map[key]resourceAccessSet{}
			}
			a.set[k] = m
		}

		for k, v := range accessMap {
			m[k] = v
		}
	}
}

func (a AccessSet) Grants(verb string, gr schema.GroupResource, namespace, name string) bool {
	for _, v := range []string{All, verb} {
		for _, g := range []string{All, gr.Group} {
			for _, r := range []string{All, gr.Resource} {
				for k := range a.set[key{
					verb: v,
					gr: schema.GroupResource{
						Group:    g,
						Resource: r,
					},
				}] {
					if k.Grants(namespace, name) {
						return true
					}
				}
			}
		}
	}

	return false
}

func (a AccessSet) AccessListFor(verb string, gr schema.GroupResource) (result AccessList) {
	dedup := map[Access]bool{}
	for _, v := range []string{All, verb} {
		for _, g := range []string{All, gr.Group} {
			for _, r := range []string{All, gr.Resource} {
				for k := range a.set[key{
					verb: v,
					gr: schema.GroupResource{
						Group:    g,
						Resource: r,
					},
				}] {
					dedup[k] = true
				}
			}
		}
	}

	for k := range dedup {
		result = append(result, k)
	}

	return
}

func (a *AccessSet) Add(verb string, gr schema.GroupResource, access Access) {
	if a.set == nil {
		a.set = map[key]resourceAccessSet{}
	}

	k := key{verb: verb, gr: gr}
	if m, ok := a.set[k]; ok {
		m[access] = true
	} else {
		m = map[Access]bool{}
		m[access] = true
		a.set[k] = m
	}
}

type AccessListByVerb map[string]AccessList

func (a AccessListByVerb) Grants(verb, namespace, name string) bool {
	return a[verb].Grants(namespace, name)
}

func (a AccessListByVerb) All(verb string) bool {
	return a.Grants(verb, All, All)
}

type Resources struct {
	All   bool
	Names sets.String
}

func (a AccessListByVerb) Granted(verb string) (result map[string]Resources) {
	result = map[string]Resources{}

	// if list, we need to check get also
	verbs := []string{verb}
	if verb == "list" {
		verbs = append(verbs, "get")
	}

	for _, verb := range verbs {
		for _, access := range a[verb] {
			resources := result[access.Namespace]
			if access.ResourceName == All {
				resources.All = true
			} else {
				if resources.Names == nil {
					resources.Names = sets.String{}
				}
				resources.Names.Insert(access.ResourceName)
			}
			result[access.Namespace] = resources
		}
	}
	return result
}

func (a AccessListByVerb) AnyVerb(verb ...string) bool {
	for _, v := range verb {
		if len(a[v]) > 0 {
			return true
		}
	}
	return false
}

type AccessList []Access

func (a AccessList) Grants(namespace, name string) bool {
	for _, a := range a {
		if a.Grants(namespace, name) {
			return true
		}
	}

	return false
}

type Access struct {
	Namespace    string
	ResourceName string
}

func (a Access) Grants(namespace, name string) bool {
	return a.nsOK(namespace) && a.nameOK(name)
}

func (a Access) nsOK(namespace string) bool {
	return a.Namespace == All || a.Namespace == namespace
}

func (a Access) nameOK(name string) bool {
	return a.ResourceName == All || a.ResourceName == name
}

func GetAccessListMap(s *types.APISchema) AccessListByVerb {
	if s == nil {
		return nil
	}
	v, _ := attributes.Access(s).(AccessListByVerb)
	return v
}
