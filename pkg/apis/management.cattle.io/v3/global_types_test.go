package v3

import (
	"testing"
)

func Test_setValue(t *testing.T) {
	f := &Feature{
		Spec: FeatureSpec{
			Value: nil,
		},
	}

	if f.Spec.Value != nil {
		t.Fail()
	}

	// Check if setting value to true works, if the value was previously unset
	f.SetValue(true)
	if f.Spec.Value == nil || *f.Spec.Value != true {
		t.Fail()
	}

	// Check that setting a value _again_ doesn't change the pointer
	ptr := f.Spec.Value
	f.SetValue(true)
	if f.Spec.Value != ptr {
		t.Fail()
	}

	// Check that setting the value to a different value works, if the value was
	// previously set
	f.SetValue(false)
	if f.Spec.Value == nil || *f.Spec.Value != false {
		t.Fail()
	}
}
