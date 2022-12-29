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


API examples
------------

This table is automatically generated from the output of the integration tests. If you add or update any tests, update this table by:

1. Run the integration tests locally:

```
go test -count=1 -v ./tests/v2/integration/steveapi/
```

2. Use the [included script](./make-table.sh) to validate the JSON files and update the markdown table:

```
cd ./tests/v2/integration/steveapi/
./make-table.sh
```

<!-- INSERT TABLE HERE -->
user | url | response
---|---|---
user-a | https://localhost:8443/v1/secrets | [json/user:user-a,namespace:none,query:none.json](json/user:user-a,namespace:none,query:none.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1 | [json/user:user-a,namespace:test-ns-1,query:none.json](json/user:user-a,namespace:test-ns-1,query:none.json)
user-a | https://localhost:8443/v1/secrets/test-ns-5 | [json/user:user-a,namespace:test-ns-5,query:none.json](json/user:user-a,namespace:test-ns-5,query:none.json)
user-a | https://localhost:8443/v1/secrets?labelSelector=test-label=2 | [json/user:user-a,namespace:none,query:labelSelector=test-label=2.json](json/user:user-a,namespace:none,query:labelSelector=test-label=2.json)
user-a | https://localhost:8443/v1/secrets/test-ns-2?labelSelector=test-label=2 | [json/user:user-a,namespace:test-ns-2,query:labelSelector=test-label=2.json](json/user:user-a,namespace:test-ns-2,query:labelSelector=test-label=2.json)
user-a | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-a,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-a,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-a | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test1 | [json/user:user-a,namespace:none,query:fieldSelector=metadata.name=test1.json](json/user:user-a,namespace:none,query:fieldSelector=metadata.name=test1.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-a | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-a,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-a,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test1 | [json/user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.name=test1.json](json/user:user-a,namespace:test-ns-1,query:fieldSelector=metadata.name=test1.json)
user-a | https://localhost:8443/v1/secrets?limit=8 | [json/user:user-a,namespace:none,query:limit=8.json](json/user:user-a,namespace:none,query:limit=8.json)
user-a | https://localhost:8443/v1/secrets?limit=8&continue=nondeterministictoken | [json/user:user-a,namespace:none,query:limit=8&continue=nondeterministictoken.json](json/user:user-a,namespace:none,query:limit=8&continue=nondeterministictoken.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?limit=3 | [json/user:user-a,namespace:test-ns-1,query:limit=3.json](json/user:user-a,namespace:test-ns-1,query:limit=3.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?limit=3&continue=nondeterministictoken | [json/user:user-a,namespace:test-ns-1,query:limit=3&continue=nondeterministictoken.json](json/user:user-a,namespace:test-ns-1,query:limit=3&continue=nondeterministictoken.json)
user-b | https://localhost:8443/v1/secrets | [json/user:user-b,namespace:none,query:none.json](json/user:user-b,namespace:none,query:none.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1 | [json/user:user-b,namespace:test-ns-1,query:none.json](json/user:user-b,namespace:test-ns-1,query:none.json)
user-b | https://localhost:8443/v1/secrets/test-ns-5 | [json/user:user-b,namespace:test-ns-5,query:none.json](json/user:user-b,namespace:test-ns-5,query:none.json)
user-b | https://localhost:8443/v1/secrets?labelSelector=test-label=2 | [json/user:user-b,namespace:none,query:labelSelector=test-label=2.json](json/user:user-b,namespace:none,query:labelSelector=test-label=2.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?labelSelector=test-label=2 | [json/user:user-b,namespace:test-ns-1,query:labelSelector=test-label=2.json](json/user:user-b,namespace:test-ns-1,query:labelSelector=test-label=2.json)
user-b | https://localhost:8443/v1/secrets/test-ns-2?labelSelector=test-label=2 | [json/user:user-b,namespace:test-ns-2,query:labelSelector=test-label=2.json](json/user:user-b,namespace:test-ns-2,query:labelSelector=test-label=2.json)
user-b | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-b | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-2 | [json/user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2.json](json/user:user-b,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2.json)
user-b | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test1 | [json/user:user-b,namespace:none,query:fieldSelector=metadata.name=test1.json](json/user:user-b,namespace:none,query:fieldSelector=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-2 | [json/user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2.json](json/user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test1 | [json/user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.name=test1.json](json/user:user-b,namespace:test-ns-1,query:fieldSelector=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.name=test1 | [json/user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.name=test1.json](json/user:user-b,namespace:test-ns-2,query:fieldSelector=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets?limit=3 | [json/user:user-b,namespace:none,query:limit=3.json](json/user:user-b,namespace:none,query:limit=3.json)
user-b | https://localhost:8443/v1/secrets?limit=3&continue=nondeterministictoken | [json/user:user-b,namespace:none,query:limit=3&continue=nondeterministictoken.json](json/user:user-b,namespace:none,query:limit=3&continue=nondeterministictoken.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?limit=3 | [json/user:user-b,namespace:test-ns-1,query:limit=3.json](json/user:user-b,namespace:test-ns-1,query:limit=3.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?limit=3&continue=nondeterministictoken | [json/user:user-b,namespace:test-ns-1,query:limit=3&continue=nondeterministictoken.json](json/user:user-b,namespace:test-ns-1,query:limit=3&continue=nondeterministictoken.json)
user-b | https://localhost:8443/v1/secrets/test-ns-5?limit=3 | [json/user:user-b,namespace:test-ns-5,query:limit=3.json](json/user:user-b,namespace:test-ns-5,query:limit=3.json)
user-c | https://localhost:8443/v1/secrets | [json/user:user-c,namespace:none,query:none.json](json/user:user-c,namespace:none,query:none.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1 | [json/user:user-c,namespace:test-ns-1,query:none.json](json/user:user-c,namespace:test-ns-1,query:none.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5 | [json/user:user-c,namespace:test-ns-5,query:none.json](json/user:user-c,namespace:test-ns-5,query:none.json)
user-c | https://localhost:8443/v1/secrets?labelSelector=test-label=2 | [json/user:user-c,namespace:none,query:labelSelector=test-label=2.json](json/user:user-c,namespace:none,query:labelSelector=test-label=2.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?labelSelector=test-label=2 | [json/user:user-c,namespace:test-ns-1,query:labelSelector=test-label=2.json](json/user:user-c,namespace:test-ns-1,query:labelSelector=test-label=2.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?labelSelector=test-label=2 | [json/user:user-c,namespace:test-ns-5,query:labelSelector=test-label=2.json](json/user:user-c,namespace:test-ns-5,query:labelSelector=test-label=2.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-2 | [json/user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2.json](json/user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-2.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-5 | [json/user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-5.json](json/user:user-c,namespace:none,query:fieldSelector=metadata.namespace=test-ns-5.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test1 | [json/user:user-c,namespace:none,query:fieldSelector=metadata.name=test1.json](json/user:user-c,namespace:none,query:fieldSelector=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test5 | [json/user:user-c,namespace:none,query:fieldSelector=metadata.name=test5.json](json/user:user-c,namespace:none,query:fieldSelector=metadata.name=test5.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.namespace=test-ns-1 | [json/user:user-c,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1.json](json/user:user-c,namespace:test-ns-2,query:fieldSelector=metadata.namespace=test-ns-1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-2 | [json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2.json](json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.namespace=test-ns-2.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test1 | [json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test1.json](json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?fieldSelector=metadata.name=test1 | [json/user:user-c,namespace:test-ns-5,query:fieldSelector=metadata.name=test1.json](json/user:user-c,namespace:test-ns-5,query:fieldSelector=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test5 | [json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test5.json](json/user:user-c,namespace:test-ns-1,query:fieldSelector=metadata.name=test5.json)
user-c | https://localhost:8443/v1/secrets?limit=3 | [json/user:user-c,namespace:none,query:limit=3.json](json/user:user-c,namespace:none,query:limit=3.json)
user-c | https://localhost:8443/v1/secrets?limit=3&continue=nondeterministictoken | [json/user:user-c,namespace:none,query:limit=3&continue=nondeterministictoken.json](json/user:user-c,namespace:none,query:limit=3&continue=nondeterministictoken.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?limit=3 | [json/user:user-c,namespace:test-ns-1,query:limit=3.json](json/user:user-c,namespace:test-ns-1,query:limit=3.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?limit=3 | [json/user:user-c,namespace:test-ns-5,query:limit=3.json](json/user:user-c,namespace:test-ns-5,query:limit=3.json)

