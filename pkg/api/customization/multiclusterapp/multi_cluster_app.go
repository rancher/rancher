package multiclusterapp

import (
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
)

func ListHandler(request *types.APIContext, next types.RequestHandler) error {
	logrus.Debugf("Multi cluster app list handler called")

	return handler.ListHandler(request, next)
}
