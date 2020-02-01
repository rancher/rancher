package accesscontrol

import (
	"github.com/rancher/steve/pkg/attributes"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AccessSet struct {
	set map[key]resourceAccessSet
}

type resourceAccessSet map[Access]bool

type key struct {
	verb string
	gr   schema.GroupResource
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

func (a AccessSet) AccessListFor(verb string, gr schema.GroupResource) (result AccessList) {
	for _, v := range []string{all, verb} {
		for _, g := range []string{all, gr.Group} {
			for _, r := range []string{all, gr.Resource} {
				for k := range a.set[key{
					verb: v,
					gr: schema.GroupResource{
						Group:    g,
						Resource: r,
					},
				}] {
					result = append(result, k)
				}
			}
		}
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
	return a.Namespace == all || a.Namespace == namespace
}

func (a Access) nameOK(name string) bool {
	return a.ResourceName == all || a.ResourceName == name
}

func GetAccessListMap(s *types.APISchema) AccessListByVerb {
	if s == nil {
		return nil
	}
	v, _ := attributes.Access(s).(AccessListByVerb)
	return v
}
