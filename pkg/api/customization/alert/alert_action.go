package alert

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Handler struct {
	ClusterAlerts v3.ClusterAlertInterface
	ProjectAlerts v3.ProjectAlertInterface
	Notifiers     v3.NotifierInterface
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "unmute")
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "mute")
	resource.AddAction(apiContext, "deactivate")
}

func (h *Handler) ClusterActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	parts := strings.Split(request.ID, ":")
	ns := parts[0]
	id := parts[1]

	alert, err := h.ClusterAlerts.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting alert for %s :%v", request.ID, err)
		return err
	}

	switch actionName {
	case "activate":
		if alert.Status.State == "inactive" {
			alert.Status.State = "active"
		} else {
			return errors.New("the alert state is not inactive")
		}

	case "deactivate":
		if alert.Status.State == "active" {
			alert.Status.State = "inactive"
		} else {
			return errors.New("the alert state is not active")
		}

	case "mute":
		if alert.Status.State == "alerting" {
			alert.Status.State = "muted"
		} else {
			return errors.New("the alert state is not alerting")
		}

	case "unmute":
		if alert.Status.State == "muted" {
			alert.Status.State = "alerting"
		} else {
			return errors.New("the alert state is not muted")
		}
	}

	alert, err = h.ClusterAlerts.Update(alert)
	if err != nil {
		logrus.Errorf("Error while updating alert:%v", err)
		return err
	}

	//TODO: how to write data back
	request.WriteResponse(http.StatusOK, alert)
	return nil
}

func (h *Handler) ProjectActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	parts := strings.Split(request.ID, ":")
	ns := parts[0]
	id := parts[1]

	alert, err := h.ProjectAlerts.GetNamespaced(ns, id, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting alert for %s :%v", request.ID, err)
		return err
	}

	switch actionName {
	case "activate":
		if alert.Status.State == "inactive" {
			alert.Status.State = "active"
		} else {
			return errors.New("the alert state is not inactive")
		}

	case "deactivate":
		if alert.Status.State == "active" {
			alert.Status.State = "inactive"
		} else {
			return errors.New("the alert state is not active")
		}

	case "mute":
		if alert.Status.State == "alerting" {
			alert.Status.State = "muted"
		} else {
			return errors.New("the alert state is not alerting")
		}

	case "unmute":
		if alert.Status.State == "muted" {
			alert.Status.State = "alerting"
		} else {
			return errors.New("the alert state is not muted")
		}
	}

	alert, err = h.ProjectAlerts.Update(alert)
	if err != nil {
		logrus.Errorf("Error while updating alert:%v", err)
		return err
	}

	//TODO: how to write data back
	request.WriteResponse(http.StatusOK, alert)
	return nil
}
