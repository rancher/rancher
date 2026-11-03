# `workload_test.go` Summary

Verifies the Norman API for workloads and related resources, covering workload creation via Kubernetes and Norman APIs, port-kind handling, private-registry image-pull selection, probe/scheduler persistence, subPath validation, the redeploy and rollback actions, and HorizontalPodAutoscaler creation.

## `TestDeploymentCreationKubectl`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a Deployment directly via the Kubernetes API with a container port mapped to hostPort 8099.

**Assert:**
- Checks the deployment appears in the Norman workload list for the project.
- Checks the port is translated to kind "HostPort" with sourcePort 8099 and containerPort 80.

## `TestWorkloadPortKinds`
**Arrange:**
- Creates a project.

**Act:** Creates 4 workloads via the Norman API, each in its own namespace, with a container port of a different kind (HostPort 776, NodePort 777, LoadBalancer 778, ClusterIP 779), all on containerPort 80.

**Assert:**
- Checks each workload is created successfully.
- Checks the port kind and sourcePort are correctly stored and returned for each of the 4 kinds, with containerPort consistently 80.

## `TestWorkloadImageChangePrivateRegistry`
**Arrange:**
- Creates a project and a namespace.
- Creates two docker credentials: one for index.docker.io, one for quay.io.

**Act 1:** Creates a workload using a docker.io image.
**Assert 1:**
- Checks `imagePullSecrets` contains the index.docker.io credential.

**Act 2:** Updates the workload's image to a quay.io image.
**Assert 2:**
- Checks the container's image is updated to the quay.io image.
- Checks `imagePullSecrets` is now updated to the quay.io credential.

## `TestWorkloadPortsChange`
**Arrange:**
- Creates a project and a namespace.

**Act 1:** Creates a workload with no container ports.
**Assert 1:**
- Checks the backing ClusterIP service is headless (no cluster IP).

**Act 2:** Updates the workload to add a ClusterIP port on containerPort 80.
**Assert 2:**
- Checks the service is assigned a non-empty cluster IP.

**Act 3:** Updates the workload again to remove the port.
**Assert 3:**
- Checks the service's cluster IP is cleared back to empty/nil.

## `TestWorkloadProbes`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a workload with a container that has liveness and readiness probes configured (failureThreshold 3, initialDelaySeconds 10, periodSeconds 2, successThreshold 1, timeoutSeconds 2, host "localhost", port 80, among other settings).

**Assert:**
- Checks both probes are created with host "localhost".
- Checks updating both probes' host to "updatedhost" persists through reload.

## `TestWorkloadScheduling`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a workload with `scheduling.scheduler` set to "some-scheduler".

**Assert:**
- Checks the workload is created with scheduler "some-scheduler".
- Checks updating the scheduler to "test-scheduler" persists through reload.

## `TestStatefulSetWorkloadVolumeMountSubpath`
**Arrange:**
- Creates a project.
- Defines a shared StatefulSet config and persistentVolumeClaim volume used for every create/update attempt.

**Act:** Creates and updates a StatefulSet workload using a volumeMount subPath.

**Assert:**
- Checks creation with an absolute subPath ("/mysql") is rejected with HTTP 422.
- Checks creation with a subPath containing ".." ("../mysql") is rejected with HTTP 422.
- Checks creation with a valid relative subPath ("mysql") succeeds.
- Checks updating the created workload with either invalid subPath is also rejected with HTTP 422.

## `TestWorkloadRedeploy`
**Arrange:**
- Creates a project and a namespace.
- Creates a workload.

**Act:** Triggers the redeploy action on the workload.

**Assert:**
- Checks the redeploy action returns a 2xx status.
- Checks the workload gains a `cattle.io/timestamp` annotation after redeploy.

## `TestWorkloadActionReadOnly`
**Arrange:**
- Creates a project and a namespace.
- Creates a workload as admin and updates it once to produce a second revision.
- Creates a read-only user and a project-member user, each bound to the project with their respective role.
- Looks up the workload's replicaSet ID from its revisions.

**Act:** A read-only user and a project-member user each attempt the rollback action on the workload.

**Assert:**
- Checks the read-only user receives HTTP 404.
- Checks the project-member user succeeds with a 2xx status.

## `TestHPA`
**Arrange:**
- Creates a project and a namespace.
- Creates a workload with a CPU resource request.

**Act:** Creates a HorizontalPodAutoscaler referencing the workload with maxReplicas 10 and 4 metric types (Resource-cpu at 50% utilization, Pods averageValue 50, External value 50, Object value 50).

**Assert:**
- Checks the HPA is created successfully with all 4 metrics.
- Checks exactly one HPA appears in the list, with state "initializing".
