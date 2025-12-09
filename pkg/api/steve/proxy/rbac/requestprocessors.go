package rbac

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	LabelSelector = "labelSelector"
	ProjectLabel  = "field.cattle.io/projectId"
)

var verifyList = []string{
	"kubeovn.io.vpcbundles",
	"kubeovn.io.ovn-eips",
	"kubeovn.io.ovnsnatrule",
	"kubeovn.io.ovndnatrule",
	"kubeovn.io.ovn-fips",
}

type ProjectIDInjector struct{}

func (p *ProjectIDInjector) Process(r *http.Request) error {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		return fmt.Errorf("projectId cannot be null")
	}

	q := r.URL.Query()
	selector := fmt.Sprintf("%s=%s", ProjectLabel, projectID)

	if old := q.Get(LabelSelector); old != "" {
		q.Set(LabelSelector, old+","+selector)
	} else {
		q.Set(LabelSelector, selector)
	}

	r.URL.RawQuery = q.Encode()
	logrus.Infof("URL %s has been injected with projectId", r.URL)

	return nil
}

func (p *ProjectIDInjector) Match(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	for _, sub := range verifyList {
		if strings.Contains(r.URL.Path, sub) {
			return true
		}
	}
	return false
}
