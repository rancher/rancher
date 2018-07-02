package rkenodeconfigclient

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/sirupsen/logrus"
)

const (
	defaultResourceNodeFoundRetryCount = 3
)

func ConfigClientWhileWindows(ctx context.Context, httpClient *http.Client, url string, header http.Header, writeCertOnly bool) error {
	if httpClient == nil {
		httpClient = client
	}

	resourceNotFoundRetryCount := defaultResourceNodeFoundRetryCount

	for {
		nc, err := getConfig(httpClient, url, header)
		if err != nil {
			if _, ok := err.(*ErrNodeOrClusterNotFound); ok {
				if resourceNotFoundRetryCount <= 0 {
					return err
				}
				resourceNotFoundRetryCount -= 1
			} else {
				logrus.Warn("Getting agent config:", err.Error())
			}

			time.Sleep(10 * time.Second)
			continue
		}

		if nc != nil {
			return rkeworker.ExecutePlan(ctx, nc, writeCertOnly)
		}

		logrus.Info("Waiting for node to register")
		time.Sleep(2 * time.Second)
	}
}
