
# Proxy Provisioning Configs

Please refer to [RKE1 Provisioning](../rke1/README.md) and [RKE2 Provisioning](../rke2/README.md) to build config file with basic `provisioningInput` parameters.

Your GO test_package should be set to `provisioning/proxy`.
Your GO suite should be set to `-run ^TestRKE2ProxyTestSuite$`.
Please see below for more details for your proxy config. Please note that the config can be in either JSON or YAML (all examples are illustrated in YAML).

For provisioning tests, include the following parameters into `agentEnvVars` and/or `agentEnvVarsRKE1` inside your `provisioningInput` 

```yaml
provisioningInput:
  agentEnvVars:
  - name: HTTPS_PROXY
    value: #proxy server internal ip address:port
  - name: HTTP_PROXY
    value: #proxy server internal ip address:port
  - name: NO_PROXY
    value: localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,172.16.0.0/12,cattle-system.svc
  agentEnvVarsRKE1:
  - name: HTTPS_PROXY
    value: #proxy server internal ip address:port
  - name: HTTP_PROXY
    value: #proxy server internal ip address:port
  - name: NO_PROXY
    value: localhost,127.0.0.1,0.0.0.0,10.0.0.0/8,172.16.0.0/12,cattle-system.svc
```

You should have a basic understanding of Corral before running the custom cluster tests. 
In your config file, set the following:
```yaml
corralPackages:
  corralPackageImages:
    airgapCustomCluster: "/dist/aws-rancher-custom-cluster-false-true"
    rancherHA: "/dist/aws-aws-proxy-standalone-rke2-rancher-proxy-calico-true-2.15.1-1.11.0" # the name of the corral rancher is configurable with config entry above
    ...
  hasDebug: <bool, default=false>
  hasCleanup: <bool, default=true>
corralConfigs:
  corralConfigUser: <string, default="jenkauto">
  corralConfigVars:
    <var1>: <string, "val1"> # for now only aws is supported, so use the appropriate aws vars
    registry_ip: <addr> # if the proxied rancher instance is created beforehand (not in the same job) set this to the registry public IP, otherwise it is automatically done in the job. 
    registry_private_ip: <addr> # if the proxied rancher instance is created beforehand (not in the same job) set this to the registry private IP, otherwise it is automatically done in the job.
    rancher_chart_repo: <val> #  
    rancher_version: <val> #
    kubernetes_version: <val> #
    corral_private_key: <val> # only set this if you have created the proxied rancher instance beforehand. By doing `corral vars <corral> corral_private_key`
    corral_public_key: <val> # only set this if you have created the proxied rancher instance beforehand. By doing `corral vars <corral> corral_private_key`
    ...
  corralSSHPath: <string, optional, mostly for local testing>
corralRancherHA:
  name: rancherha # this is the name of your aigap corral package if it hasn't been created beforehand
```

Note: `corralConfigUser` will be the prefix for all resources created in your provider. 
From there, your `corralConfigVars` should contain the parameters necessary to run the test. You can see what variables need to be set by navigating to your corral package folder and checking the `manifest.yaml` variables.

In order to run the entire proxy package set the package to `proxy/...` Your GO suite should be set to blank. 

Formatting `corral_private_key`:
1. Output key to file `corral vars <corral> corral_private_key > temp`
2. Copy single line version of key `awk -v ORS='\\n' '1' temp | pbcopy`
3. Paste into config (example yaml format `corral_private_key: '"<key>"'`)