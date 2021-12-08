# New rancher client for new API v1
Before we dive into the technical details, let's clarify a confusion here:

Yes, the new API is v1, and the old API is v3.

The new API is v1 because it is the beginning of Rancher's new API framework
which is used for Rancher's new UI, i.e. the cluster explorer, to talk with the backend.

The support for the new v1 API is added to the [Rancher client module](https://github.com/rancher/client-python).

There are two levels of the client: admin client and cluster client.

In fact, the admin client is the cluster client of the local cluster which only the admin can access to. 

# Naming the testing methods

The format

    test_{resource_name}_[with_{field}]_{operation}[_{order_number}]
where:
- resource_name: [required] the name of the resource to test.
                            Examples: namespace, deployment, monitoring
- field:         [optional] The name of the additional field added to the basic.
                            Examples: secret, lb, label
- operation:     [required] the verb of operation.
                            Examples: create, update, delete
- order_number:  [optional] when there are more than 1 test case, the number starts from 2

Examples:
- test_deployment()
- test_deployment_update()
- test_deployment_update_2()
- test_deployment_with_secret_create()
- test_deployment_with_secret_update()
- test_monitoring_enable()
- test_monitoring_update()
- test_notifier_create()
