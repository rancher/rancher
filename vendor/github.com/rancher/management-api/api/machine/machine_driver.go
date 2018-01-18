package machine

import (
	"net/http"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Handlers struct {
	MachineDriverClient v3.MachineDriverInterface
}

func (h *Handlers) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	m, err := h.MachineDriverClient.GetNamespace(apiContext.ID, "", metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch actionName {
	case "activate":
		m.Spec.Active = true
		v3.MachineDriverConditionActive.Unknown(m)
	case "deactivate":
		m.Spec.Active = false
		v3.MachineDriverConditionInactive.Unknown(m)
	}

	_, err = h.MachineDriverClient.Update(m)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

// Formatter for MachineDriver
func (h *Handlers) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "deactivate")
}

// Formatter for Machine
func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	roles := convert.ToStringSlice(resource.Values[client.MachineFieldRole])
	if len(roles) == 0 {
		resource.Values[client.MachineFieldRole] = []string{"worker"}
	}
}
