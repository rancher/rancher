package schema

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rancher/steve/pkg/attributes"
	schema2 "github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schema/converter"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/resources/common"
	apiextcontrollerv1beta1 "github.com/rancher/wrangler-api/pkg/generated/controllers/apiextensions.k8s.io/v1beta1"
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/apiregistration.k8s.io/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/discovery"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
	apiv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	listPool = semaphore.NewWeighted(10)
)

type SchemasHandler interface {
	OnSchemas(schemas *schema2.Collection) error
}

type handler struct {
	sync.Mutex

	ctx     context.Context
	toSync  int32
	schemas *schema2.Collection
	client  discovery.DiscoveryInterface
	cols    *common.DynamicColumns
	crd     apiextcontrollerv1beta1.CustomResourceDefinitionClient
	ssar    authorizationv1client.SelfSubjectAccessReviewInterface
	handler SchemasHandler
}

func Register(ctx context.Context,
	cols *common.DynamicColumns,
	discovery discovery.DiscoveryInterface,
	crd apiextcontrollerv1beta1.CustomResourceDefinitionController,
	apiService v1.APIServiceController,
	ssar authorizationv1client.SelfSubjectAccessReviewInterface,
	schemasHandler SchemasHandler,
	schemas *schema2.Collection) (init func() error) {

	h := &handler{
		ctx:     ctx,
		cols:    cols,
		client:  discovery,
		schemas: schemas,
		handler: schemasHandler,
		crd:     crd,
		ssar:    ssar,
	}

	apiService.OnChange(ctx, "schema", h.OnChangeAPIService)
	crd.OnChange(ctx, "schema", h.OnChangeCRD)

	return func() error {
		h.queueRefresh()
		return h.refreshAll()
	}
}

func (h *handler) OnChangeCRD(key string, crd *v1beta1.CustomResourceDefinition) (*v1beta1.CustomResourceDefinition, error) {
	h.queueRefresh()
	return crd, nil
}

func (h *handler) OnChangeAPIService(key string, api *apiv1.APIService) (*apiv1.APIService, error) {
	h.queueRefresh()
	return api, nil
}

func (h *handler) queueRefresh() {
	atomic.StoreInt32(&h.toSync, 1)

	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := h.refreshAll(); err != nil {
			logrus.Errorf("failed to sync schemas: %v", err)
			atomic.StoreInt32(&h.toSync, 1)
		}
	}()
}

func isListWatchable(schema *types.APISchema) bool {
	var (
		canList  bool
		canWatch bool
	)

	for _, verb := range attributes.Verbs(schema) {
		switch verb {
		case "list":
			canList = true
		case "watch":
			canWatch = true
		}
	}

	return canList && canWatch
}

func (h *handler) getColumns(ctx context.Context, schemas map[string]*types.APISchema) error {
	eg := errgroup.Group{}

	for _, schema := range schemas {
		if !isListWatchable(schema) {
			continue
		}

		if err := listPool.Acquire(ctx, 1); err != nil {
			return err
		}

		s := schema
		eg.Go(func() error {
			defer listPool.Release(1)
			return h.cols.SetColumns(s)
		})
	}

	return eg.Wait()
}

func (h *handler) refreshAll() error {
	h.Lock()
	defer h.Unlock()

	if !h.needToSync() {
		return nil
	}

	logrus.Info("Refreshing all schemas")
	schemas, err := converter.ToSchemas(h.crd, h.client)
	if err != nil {
		return err
	}

	filteredSchemas := map[string]*types.APISchema{}
	for id, schema := range schemas {
		if isListWatchable(schema) {
			if ok, err := h.allowed(schema); err != nil {
				return err
			} else if !ok {
				continue
			}
		}
		filteredSchemas[id] = schema
	}

	if err := h.getColumns(h.ctx, filteredSchemas); err != nil {
		return err
	}

	h.schemas.Reset(filteredSchemas)
	if h.handler != nil {
		return h.handler.OnSchemas(h.schemas)
	}

	return nil
}

func (h *handler) allowed(schema *types.APISchema) (bool, error) {
	gvr := attributes.GVR(schema)
	ssar, err := h.ssar.Create(&authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     "list",
				Group:    gvr.Group,
				Version:  gvr.Version,
				Resource: gvr.Resource,
			},
		},
	})
	if err != nil {
		return false, err
	}
	return ssar.Status.Allowed && !ssar.Status.Denied, nil
}

func (h *handler) needToSync() bool {
	old := atomic.SwapInt32(&h.toSync, 0)
	return old == 1
}
