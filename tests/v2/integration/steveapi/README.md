Steve API Integration Tests
===========================

This test suite tests the steve resource listing API using secrets as the main
test resource, since they are quick to create. The suite uses three user
scenarios, one user who is a project member, one user who has access to one
namespace in the project, and one user who has access to a few resources in a
few namespaces in the project, in order to demonstrate steve's ability to
collect and return resources across multiple access partitions. There are 25
sample secrets across 5 namespaces. Some of the sample secrets have labels or
annotations to demonstrate query parameters that use such fields.

Users
-----

| User   | Access                                                                       |
|--------|------------------------------------------------------------------------------|
| user-a | Project Owner                                                                |
| user-b | get,list for namespace test-ns-1                                             |
| user-c | get,list for secrets test1,test2 in namespaces test-ns-1,test-ns-2,test-ns-3 |

Running
-------
Create a `steveapi.yaml` file like the following:

```yaml
rancher:
    host: localhost:8444
    adminToken: token-XXX:YYY
```

`adminToken` can be obtained by logging in as the `admin` user into Rancher, then clicking on the user icon (in the top
right corner) -> Account & API Keys -> Create API Key. Choose "No Scope" for the Scope, click Create and copy the Bearer
Token string.

Then run as a normal go test, from your IDE or via:

```shell
go test -count=1 -v -run TestSteveLocal
```

