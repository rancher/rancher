[EXPERIMENTAL] terraform-controller
========

## ***Use K8s to Run Terraform***

**NOTE:** We are actively experimenting with this in the open. Consider this ALPHA software and subject to change.

Terraform-controller - This is a low level tool to run Git controlled Terraform modules in Kubernetes. The controller manages the TF state file using Kubernetes as a remote statefile backend! [Backend upstream PR](https://github.com/hashicorp/terraform/pull/19525) You can have changes auto-applied or wait for an explicit "OK" before running. 

There are two parts to the stack, the controller and the executor. 

The controller creates three CRDs and runs controllers for modules and executions. A module is the building block and is the same as a terraform module. This is referenced from an execution which is used to combine all information needed to run Terraform. The execution combines Terraform variables and environment variables from secrets and/or config maps to provide to the executor. 

The executor is a job that runs Terraform. Taking input from the execution run CRD the executor runs `terraform init`, `terraform plan` and `terraform create/destroy` depending on the context.

Executions have a 1-to-many relationship with execution runs, as updates or changes are made in the module or execution additional runs are created to update the terraform resources.

# Deploying
Use provided manifests `kubectl create -f ./manifests` to deploy to an existing k8s cluster. Manifests will create all CRDs necessary and a Deployment with the rancher/terraform-controller image. 

## Verify
```
~ kubectl get all -n terraform-controller
NAME                                        READY   STATUS    RESTARTS   AGE
pod/terraform-controller-8494cf85c5-x97sn   1/1     Running   0          17s

NAME                                   READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/terraform-controller   1/1     1            1           18s

NAME                                              DESIRED   CURRENT   READY   AGE
replicaset.apps/terraform-controller-8494cf85c5   1         1         1       18s
```

## Namespace
Everything is put in the `terraform-controller` namespace with these provided manifests. Edit metadata.namespace in files to change name space or remove to run in default. You will need to update the args for the command in the deployment to update or remove `--namespace` argument for the executable. Passing in the flag limits the controller to only watching CRD objects in it's namespace, remove this param to let the terraform-controller see all CRD objects in any namespace.

## Quickstart Appliance + k3s
Use (k3d)[https://github.com/rancher/k3d/releases] to spin up small (k3s)[https://github.com/rancher/k3s] clusters for a quick start for using the Terraform Controller. The appliance image comes pre-built with the deployment manifests and will auto-create verything the Terraform Controller needs when they boot.

```shell
~ k3d create --name terraform-controller --image rancher/terraform-controller-appliance
~ export KUBECONFIG="$(k3d get-kubeconfig --name='terraform-controller')"
~ kubectl get all -n terraform-controller
NAME                                       READY   STATUS    RESTARTS   AGE
pod/terraform-controller-d774bbd44-w4mzk   0/1     Pending   0          1s

NAME                                   READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/terraform-controller   0/1     1            0           14s

NAME                                             DESIRED   CURRENT   READY   AGE
replicaset.apps/terraform-controller-d774bbd44   1         1         0       1s

~  kubectl get crd  | grep terraformcontroller
executionruns.terraformcontroller.cattle.io   2019-05-22T17:19:01Z
executions.terraformcontroller.cattle.io      2019-05-22T17:19:01Z
modules.terraformcontroller.cattle.io         2019-05-22T17:19:01Z

~ k3d delete --name terraform
```

# Example
With the controller running you can run the provided example in the `./example` directory which shows you how to run a basic [Digital Ocean Terraform module](https://github.com/dramich/domodule) which takes a do_token and do_name and creates a droplet.

Modify `./example/00-secret.yaml` with your Digital Ocean API token and desired droplet name.

Run `kubectl create -f ./example -n terraform-controller` to create all envvars/secrets/module and the execution which will automatically run. The controller creates Jobs for the Terraform runs so to access logs check the pod logs for the executor create and destroy jobs. This example is setup to auto-confirm and auto-delete when the CRD object is destroyed.

Delete the droplet by deleting the CRD `kubectl delete -f ./example/20-deployment.yaml -n terraform-controller`. 

## Approving a Plan
In `./example/20-execution.yaml` its pre-configured to auto-approve and auto-delete when you make the execution CRD. You can turn off `spec.destroyOnDelete` and `spec.autoConfirm` and do these by hand doing the following.

To get the plan check logs of the pods used to run the job.
`kubectl logs [executer-pod-name] -n terraform-controller`

Assuming the action Terraform is going to perform is correct annotate the Execution Run to approve the changes:

`kubectl annotate executionruns.terraform-controller.cattle.io [execution-run-name] -n terraform-controller approved="yes" --overwrite`

Once the job completes, you can see the outputs from Terraform by checking the Execution Run:

`kubectl get executionruns.terraform-controller.cattle.io [execution-run-name] -n terraform-controller -o yaml`

With destroyOnDelete turned off you will have to delete the Droplet by hand as a destroy job will not kick off.

## Building Custom Execution Environment

Create a Dockerfile

```
FROM rancher/terraform-controller-executor:v0.0.3 #Or whatever the release is
RUN curl https://myurl.com/get-some-binary
```

Build that image and push to a registry.

When creating the execution define the image:
```
apiVersion: terraformcontroller.cattle.io/v1
kind: Execution
metadata:
  name: cluster-create
spec:
  moduleName: cluster-modules
  destroyOnDelete: true
  autoConfirm: false
  image: cloudnautique/tf-executor-rancher2-provider:v0.0.3 # Custom IMAGE
  variables:
    SecretNames:
    - my-secret
    envConfigNames:
    - env-config
```

If you already have an execution, edit the CR via kubectl and add the image field.

## Building
`make`

### Local Execution
Use `./bin/terraform-controller`

### Running the Executor in Docker - Useful for testing the Executor
docker run -d -v "/Path/To/Kubeconfig:/root/.kube/config" -e "KUBECONFIG=/root/.kube/config" -e "EXECUTOR_RUN_NAME=RUN_NAME" -e "EXECUTOR_ACTION=create" rancher/terraform-controller-executor:dev

## License
Copyright (c) 2019 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
