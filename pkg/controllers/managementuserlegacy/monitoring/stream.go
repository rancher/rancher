package monitoring

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func stream(funcs ...func() error) error {
	return utilerrors.AggregateGoroutines(funcs...)
}
