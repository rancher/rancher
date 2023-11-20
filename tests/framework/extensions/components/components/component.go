package components

import (
	"context"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Component interface {
	Apply(poll bool, poll_interval time.Duration, poll_timeout time.Duration) (err error)
	Revert(poll bool, poll_interval time.Duration, poll_timeout time.Duration) (err error)
}

// ObjSpecs: The API object structs that inlude everything necessary to create that object through the Steve client
// objs: The API objects created on the cluster.
// ObjType: The api endpoint which is needed to create/delete the api object.
// Client: The rancher client used to establish the connection to the rancher cluster.
type GenericCreate struct {
	ObjSpecs []interface{}
	objs     []v1.SteveAPIObject
	ObjType  string
	Client   *rancher.Client
}

func (gc *GenericCreate) Apply(poll bool, poll_interval time.Duration, poll_timeout time.Duration) (err error) {
	for _, objSpec := range gc.ObjSpecs {
		resp, err := gc.Client.Steve.SteveType(gc.ObjType).Create(objSpec)
		if err != nil {
			return err
		}
		log.WithFields(log.Fields{
			"ObjType": gc.ObjType,
			"Spec":    objSpec,
		}).Infof("Creating")
		gc.objs = append(gc.objs, *resp)
	}

	if poll {
		for _, obj := range gc.objs {
			waiting := false
			waitErr := wait.PollUntilContextTimeout(context.TODO(), poll_interval, poll_timeout, true, func(ctx context.Context) (done bool, err error) {
				if obj.State.Name == "active" {
					log.WithField("Name", obj.ObjectMeta.ObjectMeta.Name).Infof("%v is active", gc.ObjType)
					return true, nil
				}

				if !waiting {
					log.WithField("Name", obj.ObjectMeta.ObjectMeta.Name).Infof("Polling %v until active", gc.ObjType)
					waiting = true
				}

				return false, nil
			})
			if waitErr != nil {
				return err
			}
		}
	}

	return nil
}

func (gc *GenericCreate) Revert(poll bool, poll_interval time.Duration, poll_timeout time.Duration) (err error) {
	for _, obj := range gc.objs {
		err := gc.Client.Steve.SteveType(gc.ObjType).Delete(&obj)
		if err != nil {
			return err
		}
		log.WithField("Name", obj.ObjectMeta.ObjectMeta.Name).Infof("Deleting %v", gc.ObjType)
	}

	//currently not functional work in progress
	if poll {
		for _, obj := range gc.objs {
			waiting := false
			waitErr := wait.PollUntilContextTimeout(context.TODO(), poll_interval, poll_timeout, true, func(ctx context.Context) (done bool, err error) {

				if obj.State.Name != "active" {
					log.Info("INACTIVE")
					return true, nil
				}

				if !waiting {
					waiting = true
				}

				return false, nil
			})

			if waitErr != nil {
				return err
			}
		}
	}
	return nil
}
