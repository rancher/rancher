//go:build go1.18
// +build go1.18

// Copyright (c) 2014-2026 Rancher Labs, Inc.
// SPDX-License-Identifier: Apache-2.0

package rancher_test

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/providers/scim"
)

// FuzzSCIMFilterParse tests SCIM filter parsing with arbitrary
// attacker-controlled filter strings.
//
// Rancher is a container management platform with 25K+ stars
// and 30 GitHub Security Advisories. SCIM filters control
// user/group provisioning in identity management.
func FuzzSCIMFilterParse(f *testing.F) {
	f.Add(`userName eq "test@example.com"`)
	f.Add(`name co "admin"`)
	f.Add("")
	f.Add(`(`)
	f.Add(string(make([]byte, 1000)))

	f.Fuzz(func(t *testing.T, filter string) {
		if len(filter) > 1<<16 {
			return
		}
		// Parse must never panic
		_, _ = scim.ParseFilter(filter)
	})
}

// FuzzGenericMapUnmarshal tests JSON unmarshaling with arbitrary
// byte input into Rancher's GenericMap type.
func FuzzGenericMapUnmarshal(f *testing.F) {
	f.Add([]byte(`{"key":"value"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}
		var m rkev1.GenericMap
		_ = m.UnmarshalJSON(data)
	})
}
