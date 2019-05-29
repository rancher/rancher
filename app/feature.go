package app

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/features"
)

func addFeatures(cfg Config) error {
	featuresAndValues := strings.Split(cfg.Features, ",")

	if len(featuresAndValues) == 0 {
		return nil
	}

	for _, val := range featuresAndValues {
		feat := strings.Split(val, "=")
		if len(feat) != 2 {
			return httperror.NewAPIError(httperror.InvalidBodyContent, "features value must be in \"featureName=boolValue\" format")
		}
		if err := features.SetFeature(feat[0], feat[1]); err != nil {
			return httperror.NewAPIError(httperror.InvalidBodyContent, "features value must be in \"featureName=boolValue\" format")
		}
	}

	return nil
}

