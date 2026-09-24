# `workload_test.go` Summary

Verifies the Norman API for workloads (Deployments, StatefulSets) and related resources, testing workload creation via Norman and Kubernetes APIs, port mapping, private registry image pulling, port type changes with service cluster IP updates, probe persistence, scheduler field updates, volume mount subPath validation, redeploy action, read-only access permissions, and HorizontalPodAutoscaler creation.

## `TestDeploymentCreationKubectl`
Creates a Deployment directly via the Kubernetes API with a container port mapped to hostPort 8099, then polls the Norman workload API until the deployment appears with the correct port mapping translated to sourcePort + kind=HostPort.
- Checks the deployment appears in the Norman workload list for the project.
- Checks the port kind is "HostPort" with sourcePort 8099 and containerPort 80.

## `TestWorkloadPortKinds`
Creates 4 separate workloads via the Norman API, each with a container port using a different kind (HostPort 776, NodePort 777, LoadBalancer 778, ClusterIP 779) all on containerPort 80.
- Checks each workload is created successfully with the specified port kind.
- Checks the port kind and sourcePort are correctly stored and returned for HostPort, NodePort, LoadBalancer, and ClusterIP kinds.
- Checks containerPort is consistently 80 across all port types.

## `TestWorkloadImageChangePrivateRegistry`
Creates 2 docker credentials (for index.docker.io and quay.io), then creates a workload with a docker.io image, verifies registry1 is auto-selected as imagePullSecret, updates the workload image to quay.io, and verifies registry2 is now auto-selected.
- Checks workload is created with imagePullSecrets containing registry1 name for docker.io image.
- Checks updating the image to quay.io causes imagePullSecrets to be updated to registry2 name.

## `TestWorkloadPortsChange`
Creates a workload with no ports (expecting headless service with no cluster IP), updates it to add a ClusterIP port (expecting cluster IP to be assigned), then removes the port (expecting cluster IP to be cleared).
- Checks headless workload service has no cluster IP or empty cluster IP.
- Checks adding a ClusterIP port on containerPort 80 results in a non-empty cluster IP being assigned.
- Checks removing the port resets cluster IP to empty or null.

## `TestWorkloadProbes`
Creates a workload with a container that has both liveness and readiness probes configured with specific settings (failureThreshold 3, initialDelaySeconds 10, periodSeconds 2, etc.), updates the probe host fields to "updatedhost", and verifies persistence.
- Checks the workload is created with liveness and readiness probes configured with the specified values.
- Checks probe host fields can be updated and persist through reload.

## `TestWorkloadScheduling`
Creates a workload with a scheduler field set to "some-scheduler", updates it to "test-scheduler", and verifies the scheduler field persists.
- Checks the workload is created with the specified scheduler value.
- Checks the scheduler value can be updated and persists through reload.

## `TestStatefulSetWorkloadVolumeMountSubpath`
Creates a StatefulSet via the Norman API with a volumeMount, testing that subPath validation rejects absolute paths and paths containing "..".
- Checks workload creation with absolute subPath "/mysql" is rejected with HTTP 422.
- Checks workload creation with subPath containing ".." is rejected with HTTP 422.
- Checks workload creation with valid relative subPath "mysql" succeeds.
- Checks workload updates with invalid subPaths are also rejected with HTTP 422.

## `TestWorkloadRedeploy`
Creates a workload, triggers the redeploy action, and polls until the cattle.io/timestamp annotation appears on the workload.
- Checks the redeploy action succeeds (2xx status).
- Checks the workload gains a cattle.io/timestamp annotation after redeploy is triggered.

## `TestWorkloadActionReadOnly`
Creates a workload, then creates read-only and project-member users with respective role bindings, and tests that read-only user receives 404 on rollback action while project-member succeeds.
- Checks workload is created by admin.
- Checks read-only user attempting rollback receives HTTP 404.
- Checks project-member user attempting rollback succeeds with 2xx status.

## `TestHPA`
Creates a workload with CPU resource requests, then creates a HorizontalPodAutoscaler referencing the workload with 4 metric types (Resource-cpu at 50% utilization, Pods-test at average 50, External at value 50, Object-test at value 50) and maxReplicas 10.
- Checks HPA is created successfully with all 4 metric types configured.
- Checks HPA appears in the list with state "initializing".
