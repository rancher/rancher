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

// DockerCredentialController interface for managing DockerCredential resources.
type DockerCredentialController interface {
	generic.ControllerInterface[*v3.DockerCredential, *v3.DockerCredentialList]
}

// DockerCredentialClient interface for managing DockerCredential resources in Kubernetes.
type DockerCredentialClient interface {
	generic.ClientInterface[*v3.DockerCredential, *v3.DockerCredentialList]
}

// DockerCredentialCache interface for retrieving DockerCredential resources in memory.
type DockerCredentialCache interface {
	generic.CacheInterface[*v3.DockerCredential]
}
