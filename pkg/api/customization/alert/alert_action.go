package alert

import (
	"net/http"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Handler struct {
	ClusterAlertRule v3.ClusterAlertRuleInterface
	ProjectAlertRule v3.ProjectAlertRuleInterface
	Notifiers        v3.NotifierInterface
	DialerFactory    dialer.Factory
}

func RuleFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "unmute")
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "mute")
	resource.AddAction(apiContext, "deactivate")
}

func GroupFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "unmute")
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "mute")
	resource.AddAction(apiContext, "deactivate")
}

func (h *Handler) ClusterAlertRuleActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	parts := strings.Split(request.ID, ":")
	ns := parts[0]
	id := parts[1]

	alert, err := h.ClusterAlertRule.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting alert for %s :%v", request.ID, err)
		return err
	}

	switch actionName {
	case "activate":
		if alert.Status.AlertState == "inactive" {
			alert.Status.AlertState = "active"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not inactive")
		}

	case "deactivate":
		if alert.Status.AlertState == "active" {
			alert.Status.AlertState = "inactive"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not active")
		}

	case "mute":
		if alert.Status.AlertState == "alerting" {
			alert.Status.AlertState = "muted"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not alerting")
		}

	case "unmute":
		if alert.Status.AlertState == "muted" {
			alert.Status.AlertState = "alerting"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not muted")
		}
	}

	alert, err = h.ClusterAlertRule.Update(alert)
	if err != nil {
		logrus.Errorf("Error while updating alert:%v", err)
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(request, request.Version, request.Type, request.ID, &data); err != nil {
		return err
	}
	request.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) ProjectAlertRuleActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	parts := strings.Split(request.ID, ":")
	ns := parts[0]
	id := parts[1]

	alert, err := h.ProjectAlertRule.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting alert for %s :%v", request.ID, err)
		return err
	}

	switch actionName {
	case "activate":
		if alert.Status.AlertState == "inactive" {
			alert.Status.AlertState = "active"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not inactive")
		}

	case "deactivate":
		if alert.Status.AlertState == "active" {
			alert.Status.AlertState = "inactive"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not active")
		}

	case "mute":
		if alert.Status.AlertState == "alerting" {
			alert.Status.AlertState = "muted"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not alerting")
		}

	case "unmute":
		if alert.Status.AlertState == "muted" {
			alert.Status.AlertState = "alerting"
		} else {
			return httperror.NewAPIError(httperror.ActionNotAvailable, "state is not muted")
		}
	}

	alert, err = h.ProjectAlertRule.Update(alert)
	if err != nil {
		logrus.Errorf("Error while updating alert:%v", err)
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(request, request.Version, request.Type, request.ID, &data); err != nil {
		return err
	}
	request.WriteResponse(http.StatusOK, data)
	return nil
}
