package controller

import (
	"context"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type HandlerFunc func(key string, obj interface{}) (interface{}, error)

type GenericController interface {
	Informer() cache.SharedIndexInformer
	AddHandler(ctx context.Context, name string, handler HandlerFunc)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, after time.Duration)
}

type genericController struct {
	controller controller.SharedController
	informer   cache.SharedIndexInformer
	name       string
	namespace  string
}

func NewGenericController(namespace, name string, controller controller.SharedController) GenericController {
	return &genericController{
		controller: controller,
		informer:   controller.Informer(),
		name:       name,
		namespace:  namespace,
	}
}

func (g *genericController) Informer() cache.SharedIndexInformer {
	return g.informer
}

func (g *genericController) Enqueue(namespace, name string) {
	g.controller.Enqueue(namespace, name)
}

func (g *genericController) EnqueueAfter(namespace, name string, after time.Duration) {
	g.controller.EnqueueAfter(namespace, name, after)
}

func (g *genericController) AddHandler(ctx context.Context, name string, handler HandlerFunc) {
	g.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(func(key string, obj runtime.Object) (runtime.Object, error) {
		if !isNamespace(g.namespace, obj) {
			return obj, nil
		}
		logrus.Tracef("%s calling handler %s %s", g.name, name, key)
		result, err := handler(key, obj)
		runtimeObject, _ := result.(runtime.Object)
		if _, ok := err.(*ForgetError); ok {
			logrus.Tracef("%v %v completed with dropped err: %v", g.name, key, err)
			return runtimeObject, controller.ErrIgnore
		}
		return runtimeObject, err
	}))
}

func isNamespace(namespace string, obj runtime.Object) bool {
	if namespace == "" || obj == nil {
		return true
	}
	meta, err := meta.Accessor(obj)
	if err != nil {
		// if you can't figure out the namespace, just let it through
		return true
	}
	return meta.GetNamespace() == namespace
}
