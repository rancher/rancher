package cluster

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"time"

	yaml2 "github.com/ghodss/yaml"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	v1 "github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	authV1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientauthv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type noopCloser struct {
	io.Reader
}

func (noopCloser) Close() error {
	return nil
}

func findNamespaceCreates(inputYAML string) ([]string, error) {
	var namespaces []string

	reader := yaml.NewDocumentDecoder(noopCloser{Reader: bytes.NewBufferString(inputYAML)})
	for {
		next, readErr := ioutil.ReadAll(reader)
		if readErr != nil && readErr != io.ErrShortBuffer {
			return nil, readErr
		}

		obj := &unstructured.Unstructured{}
		next, err := yaml2.YAMLToJSON(next)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(next, &obj.Object)
		if err != nil {
			return nil, err
		}

		if obj.IsList() {
			obj.EachListItem(func(obj runtime.Object) error {
				metadata, err := meta.Accessor(obj)
				if err != nil {
					return err
				}
				if obj.GetObjectKind().GroupVersionKind().Kind == "Namespace" && obj.GetObjectKind().GroupVersionKind().Version == "v1" {
					namespaces = append(namespaces, metadata.GetName())
				}

				if metadata.GetNamespace() != "" {
					namespaces = append(namespaces, metadata.GetNamespace())
				}
				return nil
			})
		} else if obj.GetKind() == "Namespace" && obj.GetAPIVersion() == "v1" {
			namespaces = append(namespaces, obj.GetName())
			if obj.GetNamespace() != "" {
				namespaces = append(namespaces, obj.GetNamespace())
			}
		}

		if readErr == nil {
			break
		}
	}

	uniq := map[string]bool{}
	var newNamespaces []string
	for _, ns := range namespaces {
		if !uniq[ns] {
			uniq[ns] = true
			newNamespaces = append(newNamespaces, ns)
		}
	}

	return newNamespaces, nil
}

func waitForNS(nsClient v1.NamespaceInterface, namespaces []string) {
	for i := 0; i < 3; i++ {
		allGood := true
		for _, ns := range namespaces {
			ns, err := nsClient.Get(ns, v12.GetOptions{})
			if err != nil {
				allGood = false
				break
			}
			status := ns.Annotations["cattle.io/status"]
			if status == "" {
				allGood = false
				break
			}
			nsMap := map[string]interface{}{}
			err = json.Unmarshal([]byte(status), &nsMap)
			if err != nil {
				allGood = false
				break
			}

			foundCond := false
			conds := convert.ToMapSlice(values.GetValueN(nsMap, "Conditions"))
			for _, cond := range conds {
				if cond["Type"] == "InitialRolesPopulated" && cond["Status"] == "True" {
					foundCond = true
				}
			}

			if !foundCond {
				allGood = false
			}
		}

		if allGood {
			break
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}

func CanCreateRKETemplate(callerID string, subjectAccessReviewClient clientauthv1.SubjectAccessReviewInterface) (bool, error) {
	review := authV1.SubjectAccessReview{
		Spec: authV1.SubjectAccessReviewSpec{
			User: callerID,
			ResourceAttributes: &authV1.ResourceAttributes{
				Verb:     "create",
				Resource: "clustertemplates",
				Group:    "management.cattle.io",
			},
		},
	}

	result, err := subjectAccessReviewClient.Create(&review)
	if err != nil {
		return false, err
	}
	logrus.Debugf("CanCreateRKETemplate: %v", result)
	return result.Status.Allowed, nil
}
