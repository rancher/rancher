package v1

// Copyright 2019 Rancher Labs, Inc.
// SPDX-License-Identifier: Apache-2.0

import "errors"

var (
	ErrPlanUnresolvable = errors.New("cannot resolve plan: missing channel and version")
)
