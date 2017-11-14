machine-controller
========

Machine-Controller uses docker-machine and k8s CRD to provision machine on the cloud

## Building

`make`

## Usage

### Creating Machine

#### Install Docker-Machine first

https://docs.docker.com/machine/install-machine/

#### Create machine custom resource definition

`kubectl create -f example/machine-crd.yml`

#### Run the machine controller

`./bin/machine-controller --config ~/.kube/config`

#### Create a digitalocean machine

`kubectl create -f example/digitalocean-example.yml`

`kubectl create -f example/amazonec2-example.yml`

### Uploading Schemas

Each machine driver has its own driver options. We get these driver options and upload them to a schema CRD. Then later we can generate go type files base on these schemas.

#### Creating the schema CRD

`kubectl create -f example/schema-crd.yml`

#### Creating the Machine Driver CRD

`kubectl create -f example/machine-driver-crd.yml`

#### Creating machine drivers

`kubectl create -f example/amazonec2-machine-driver.yml`

`kubectl create -f example/digitalocean-machine-driver.yml`

```$xslt
$ kubectl get machinedrivers                                                                                                                                                                                                                                                                                                                             1  BR master 
NAME           KIND
amazonec2      MachineDriver.v3.management.cattle.io
digitalocean   MachineDriver.v3.management.cattle.io

```

Once the machine driver is created, a schema will be created automatically.

```$xslt
$ kubectl get dynamicschema                                                                                                                                                                                                                                                                                                                                 BR master 
NAME                 KIND
amazonec2config      DynamicSchema.v3.management.cattle.io
digitaloceanconfig   DynamicSchema.v3.management.cattle.io

```

#### Generate the go files base on schemas

`go generate`

```$xslt
package generator

type Amazonec2Config struct {
    
    AccessKey string `json:"accessKey,omitempty"`
    
    Ami string `json:"ami,omitempty"`
    
    BlockDurationMinutes string `json:"blockDurationMinutes,omitempty"`
    
    DeviceName string `json:"deviceName,omitempty"`
    
    Endpoint string `json:"endpoint,omitempty"`
    
    IamInstanceProfile string `json:"iamInstanceProfile,omitempty"`
    
    InsecureTransport bool `json:"insecureTransport,omitempty"`
    
    InstanceType string `json:"instanceType,omitempty"`
    
    KeypairName string `json:"keypairName,omitempty"`
    
    Monitoring bool `json:"monitoring,omitempty"`
    
    OpenPort []string `json:"openPort,omitempty"`
    
    PrivateAddressOnly bool `json:"privateAddressOnly,omitempty"`
    
    Region string `json:"region,omitempty"`
    
    RequestSpotInstance bool `json:"requestSpotInstance,omitempty"`
    
    Retries string `json:"retries,omitempty"`
    
    RootSize string `json:"rootSize,omitempty"`
    
    SecretKey string `json:"secretKey,omitempty"`
    
    SecurityGroup []string `json:"securityGroup,omitempty"`
    
    SessionToken string `json:"sessionToken,omitempty"`
    
    SpotPrice string `json:"spotPrice,omitempty"`
    
    SshKeypath string `json:"sshKeypath,omitempty"`
    
    SshUser string `json:"sshUser,omitempty"`
    
    SubnetId string `json:"subnetId,omitempty"`
    
    Tags string `json:"tags,omitempty"`
    
    UseEbsOptimizedInstance bool `json:"useEbsOptimizedInstance,omitempty"`
    
    UsePrivateAddress bool `json:"usePrivateAddress,omitempty"`
    
    Userdata string `json:"userdata,omitempty"`
    
    VolumeType string `json:"volumeType,omitempty"`
    
    VpcId string `json:"vpcId,omitempty"`
    
    Zone string `json:"zone,omitempty"`
    
}

```

## Running

`./bin/machine-controller`

## License
Copyright (c) 2014-2017 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
