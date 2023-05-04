# Standalone Configs - Corral

## Getting Started
You should have a basic understanding of Corral before running these tests. Your GO suite should be set to `-run ^TestCorralStandaloneTestSuite$`. 
In your config file, set the following:
```yaml
corralPackages:
  corralPackageImages:
    <nameOfPackage1>: <public corral image to deploy "ghcr.io/rancherlabs/corral/$pkg:latest>
    ...
  hasDebug: <bool, default=false>
  hasCleanup: <bool, default=true>
  hasCustomRepo: <string, suggeseted=https://github.com/rancherlabs/corral-packages.git>

corralConfigs:
  corralConfigUser: <string, default="jenkauto">
  corralConfigVars:
    <var1>: <string, "val1">
    ...
  corralSSHPath: <string, optional, mostly for local testing>
```
Note: `corralConfigUser` will be the prefix for all resources created in your provider. 
From there, your `corralConfigVars` should contain the parameters necessary to run the test. You can see what variables need to be set by navigating to your corral package folder and checking the `manifest.yaml` variables.