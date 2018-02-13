package hooks

import (
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
)

type WebhookHandler struct {
}

func New(management *config.ScaledContext) *WebhookHandler {
	webhookHandler := &WebhookHandler{}
	webhookHandler.initDrivers(management)
	return webhookHandler
}

func (h *WebhookHandler) initDrivers(management *config.ScaledContext) {
	RegisterDrivers(management)
}

func (h *WebhookHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	for key, driver := range Drivers {
		if exist := req.Header.Get(key); exist != "" {
			code, err := driver.Execute(req)
			if err != nil {
				e := map[string]interface{}{
					"type":    "error",
					"code":    code,
					"message": err.Error(),
				}
				logrus.Errorf("executing %s driver got error: %v", key, err)
				rw.WriteHeader(code)
				responseBody, _ := json.Marshal(e)
				rw.Write(responseBody)
			}
			rw.WriteHeader(http.StatusOK)
		}
	}

}
