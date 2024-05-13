# Import Provisioning Config

For your config, you will need everything in the Prerequisites section on the previous readme, [Define your test](#provisioning-input), and a corral config that points to a package that builds a functional cluster and has a `kubeconfig` var once the corral has built successfully. This test is setup to run the k3s corral-package, but technically as long as the corral is successful and finishes with a valid `kubeconfig`, the cluster import job will still operate properly and successfully. 

Your GO test_package should be set to `provisioning/import`.
Your GO suite should be set to `-run ^TestImportProvisioningTestSuite$`.
Please see below for more details for your config. 


## Table of Contents
1. [Prerequisites](../README.md)
2. [Configuring test flags](#Flags)
3. [Define your Corral](#corral-input)
4. [Back to general provisioning](../README.md)

## Flags
Flags are used to determine which static table tests are run (has no effect on dynamic tests) 
`Long` Will run the long version of the table tests (usually all of them)
`Short` Will run the subset of table tests with the short flag.

```yaml
flags:
  desiredflags: "Long"   #required (static tests only)
```

## Corral Input

```yaml

corralConfigs:
    corralConfigVars:
        agent_count: 0
        aws_access_key: "your_key"
        aws_ami: "your_ami"
        aws_hostname_prefix: autoimport
        aws_region: us-west-1
        aws_route53_zone: "your_zone"
        aws_secret_key: "your_secret_key"
        aws_security_group: "your_security_group"
        aws_ssh_user: "your_user"
        aws_subnet: "your_subnet"
        aws_volume_size: 50
        aws_volume_type: gp3
        hostname: "your_hostname(should match the prefix + route_53 zone)"
        aws_vpc: "your_vpc"
        instance_type: t3a.medium
        # kubernetes_version: not supported with suggested package
        node_count: 3
        server_count: 3

    corralSSHPath: "/root/go/src/github.com/rancher/rancher/tests/v2/validation/.ssh/<your_ssh_key>"
corralPackages:
    corralPackageImages:
        # NOTE: k3sToImport is the **Required** package name. It can point to any path, as long as the package name is k3sToImport.
        k3sToImport: /root/go/src/github.com/rancherlabs/corral-packages/dist/aws-k3s-v1.26.8-k3s1
```

**k3sToImport is the **Required** package name. It can point to any path, as long as the package name is k3sToImport.**