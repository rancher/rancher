/*
Copyright 2024 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v3

import (
	v3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/wrangler/v2/pkg/generic"
)

// BasicAuthController interface for managing BasicAuth resources.
type BasicAuthController interface {
	generic.ControllerInterface[*v3.BasicAuth, *v3.BasicAuthList]
}

// BasicAuthClient interface for managing BasicAuth resources in Kubernetes.
type BasicAuthClient interface {
	generic.ClientInterface[*v3.BasicAuth, *v3.BasicAuthList]
}

// BasicAuthCache interface for retrieving BasicAuth resources in memory.
type BasicAuthCache interface {
	generic.CacheInterface[*v3.BasicAuth]
}
