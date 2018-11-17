package globalresource

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"
	"strings"
)

const (
	cattleGlobalNamespace = "cattle-global-data"
)

//GlobalNamespaceStore makes sure that the global resources are assigned to a global namespace, it creates one if not already present.
type GlobalNamespaceStore struct {
	types.Store
	NamespaceInterface v1.NamespaceInterface
}

func (s *GlobalNamespaceStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	logrus.Infof("Creating a Global CR of type %v, assigning to %v namespace", apiContext.Type, cattleGlobalNamespace)

	ns, ok := values.GetValue(data, client.PreferenceFieldNamespaceId)
	if ok && !strings.EqualFold(convert.ToString(ns), cattleGlobalNamespace) {
		return nil, fmt.Errorf("Error creating Global resource, cannot assign to %v since already assigned to %v namespace", cattleGlobalNamespace, ns)
	} else if !ok {
		err := s.ensureGlobalNamespace(apiContext)
		if err != nil {
			return nil, err
		}
		data[client.PreferenceFieldNamespaceId] = cattleGlobalNamespace
	}

	return s.Store.Create(apiContext, schema, data)
}

func (s *GlobalNamespaceStore) ensureGlobalNamespace(apiContext *types.APIContext) error {
	_, err := s.NamespaceInterface.Get(cattleGlobalNamespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error listing global namespace %v: %v", cattleGlobalNamespace, err)
	}
	return nil
}
