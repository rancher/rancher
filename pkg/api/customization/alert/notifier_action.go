package alert

import (
	"encoding/json"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/notifiers"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testSMTPTitle = "Alert From Rancher: SMTP configuration validated"

func NotifierCollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	if canCreateNotifier(apiContext, nil, "") {
		collection.AddAction(apiContext, "send")
	}
}

func NotifierFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	if canCreateNotifier(apiContext, resource, "") {
		resource.AddAction(apiContext, "send")
	}
}

func (h *Handler) NotifierActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "send":
		return h.testNotifier(actionName, action, apiContext)
	}

	return httperror.NewAPIError(httperror.InvalidAction, "invalid action: "+actionName)

}

func (h *Handler) testNotifier(actionName string, action *types.Action, apiContext *types.APIContext) error {
	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body error")
	}
	input := &struct {
		Message string
		v3.NotifierSpec
	}{}
	clientNotifier := &struct {
		client.NotifierSpec
	}{}

	if err = json.Unmarshal(data, input); err != nil {
		return errors.Wrap(err, "unmarshalling input error")
	}
	if err = json.Unmarshal(data, clientNotifier); err != nil {
		return errors.Wrap(err, "unmarshalling input error client")
	}
	if !canCreateNotifier(apiContext, nil, clientNotifier.ClusterID) {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	notifier := &v3.Notifier{
		Spec: input.NotifierSpec,
	}
	msg := input.Message
	if apiContext.ID != "" {
		ns, id := ref.Parse(apiContext.ID)
		notifier, err = h.Notifiers.GetNamespaced(ns, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	notifierMessage := &notifiers.Message{
		Content: msg,
	}
	if notifier.Spec.SMTPConfig != nil {
		notifierMessage.Title = testSMTPTitle
	}

	dialer, err := h.DialerFactory.ClusterDialer(clientNotifier.ClusterID)
	if err != nil {
		return errors.Wrap(err, "error getting dialer")
	}
	return notifiers.SendMessage(notifier, "", notifierMessage, dialer)
}

func canCreateNotifier(apiContext *types.APIContext, resource *types.RawResource, clusterID string) bool {
	obj := rbac.ObjFromContext(apiContext, resource)
	if clusterID != "" {
		obj[rbac.NamespaceID] = clusterID
	}
	return apiContext.AccessControl.CanDo(v3.NotifierGroupVersionKind.Group, v3.NotifierResource.Name, "create", apiContext, obj, apiContext.Schema) == nil
}
