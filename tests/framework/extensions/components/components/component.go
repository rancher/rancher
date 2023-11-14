package components

import (
	"context"
	"time"

	"github.com/rancher/machine/libmachine/log"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Component interface {
	Apply(poll bool, poll_interval int, async bool) (err error)
	Revert(poll bool, poll_interval int, async bool) (err error)
}

type GenericCreate struct {
	ObjSpecs []interface{}
	objs     []v1.SteveAPIObject
	ObjType  string
	Client   *rancher.Client
}

func (gc *GenericCreate) Apply(poll bool, poll_interval int, async bool) (err error) {
	for _, objSpec := range gc.ObjSpecs {
		resp, err := gc.Client.Steve.SteveType(gc.ObjType).Create(objSpec)
		if err != nil {
			return err
		}
		gc.objs = append(gc.objs, *resp)

		waiting := false
		waitErr := wait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, 30*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			if resp.State.Name == "active" {
				log.Info("%v is active")
				return true, nil
			}

			if !waiting {
				log.Info("waiting")
				waiting = true
			}

			return false, nil
		})
		if waitErr != nil {
			log.Info("%+v ")
		}
	}
	return nil
}

func (gc *GenericCreate) Revert(poll bool, poll_interval int, async bool) (err error) {
	for _, obj := range gc.objs {
		err := gc.Client.Steve.SteveType(gc.ObjType).Delete(&obj)
		if err != nil {
			return err
		}
	}

	return nil
}
