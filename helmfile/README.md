### TLS termination issue solving

problems:
- Some TLS certificates were required to be passed to the chart (enforcing TLS passthrough)
  - We do TLS termination at the load balancer level, not at the pod level, so we cannot pass certificates to the chart.
- Some Ingress annotations were hardcoded with the nginx load balancer controller
  - We want to have the freedom to select the load balancer controller we want to use (in our case the default one provisioned by aws).

Modifications in the `chart` folder:
- The `Ingress` leaves the `annotations` block to be fully defined from the values
- Remove any related code to tls, certificate, CA, encryption

Here it's my helmfile (a meta tool over helm) configuration to use the modified rancher chart with my own specific arguments.
Check the `rancher-values.yaml` file for more details.

To apply it all together do (might require `helm diff` plugin):
```shell
CTX=debug # assuming that your kubeContext alias is called "debug"
helmfile -e $CTX -l app=rancher -i apply --context 1
```
