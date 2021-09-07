package wait

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func WaitForCondition(result watch.Interface, conditionFunc func(obj runtime.Object) (bool, error), errorCheckFun func(obj runtime.Object) error) error {
	defer func() {
		result.Stop()
	}()

	for {
		select {
		case event, open := <-result.ResultChan():
			if !open {
				return fmt.Errorf("timeout waiting on condition")
			}
			switch event.Type {
			case watch.Error:
				err := errorCheckFun(event.Object)
				return fmt.Errorf("there was an error: %v", err)
			case watch.Modified:
				done, err := conditionFunc(event.Object)
				if err != nil {
					return err
				} else if done {
					return nil
				}
			case watch.Deleted:
				done, err := conditionFunc(event.Object)
				if err != nil {
					return err
				} else if done {
					return nil
				}
			}
		}
	}
}
