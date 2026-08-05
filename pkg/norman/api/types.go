package api

import "github.com/rancher/rancher/pkg/norman/types"

type ResponseWriter interface {
	Write(apiContext *types.APIContext, code int, obj interface{})
}
