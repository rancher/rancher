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
user-a | https://localhost:8443/v1/secrets | [json/user-a_none_none.json](json/user-a_none_none.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1 | [json/user-a_test-ns-1_none.json](json/user-a_test-ns-1_none.json)
user-a | https://localhost:8443/v1/secrets/test-ns-5 | [json/user-a_test-ns-5_none.json](json/user-a_test-ns-5_none.json)
user-a | https://localhost:8443/v1/secrets?labelSelector=test-label=2 | [json/user-a_none_labelSelector=test-label=2.json](json/user-a_none_labelSelector=test-label=2.json)
user-a | https://localhost:8443/v1/secrets/test-ns-2?labelSelector=test-label=2 | [json/user-a_test-ns-2_labelSelector=test-label=2.json](json/user-a_test-ns-2_labelSelector=test-label=2.json)
user-a | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-1 | [json/user-a_none_fieldSelector=metadata.namespace=test-ns-1.json](json/user-a_none_fieldSelector=metadata.namespace=test-ns-1.json)
user-a | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test1 | [json/user-a_none_fieldSelector=metadata.name=test1.json](json/user-a_none_fieldSelector=metadata.name=test1.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-1 | [json/user-a_test-ns-1_fieldSelector=metadata.namespace=test-ns-1.json](json/user-a_test-ns-1_fieldSelector=metadata.namespace=test-ns-1.json)
user-a | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.namespace=test-ns-1 | [json/user-a_test-ns-2_fieldSelector=metadata.namespace=test-ns-1.json](json/user-a_test-ns-2_fieldSelector=metadata.namespace=test-ns-1.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test1 | [json/user-a_test-ns-1_fieldSelector=metadata.name=test1.json](json/user-a_test-ns-1_fieldSelector=metadata.name=test1.json)
user-a | https://localhost:8443/v1/secrets?limit=8 | [json/user-a_none_limit=8.json](json/user-a_none_limit=8.json)
user-a | https://localhost:8443/v1/secrets?limit=8&continue=nondeterministictoken | [json/user-a_none_limit=8&continue=nondeterministictoken.json](json/user-a_none_limit=8&continue=nondeterministictoken.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?limit=3 | [json/user-a_test-ns-1_limit=3.json](json/user-a_test-ns-1_limit=3.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?limit=3&continue=nondeterministictoken | [json/user-a_test-ns-1_limit=3&continue=nondeterministictoken.json](json/user-a_test-ns-1_limit=3&continue=nondeterministictoken.json)
user-a | https://localhost:8443/v1/secrets?filter=metadata.name=test1 | [json/user-a_none_filter=metadata.name=test1.json](json/user-a_none_filter=metadata.name=test1.json)
user-a | https://localhost:8443/v1/secrets?filter=metadata.name=test6 | [json/user-a_none_filter=metadata.name=test6.json](json/user-a_none_filter=metadata.name=test6.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?filter=metadata.name=test1 | [json/user-a_test-ns-1_filter=metadata.name=test1.json](json/user-a_test-ns-1_filter=metadata.name=test1.json)
user-a | https://localhost:8443/v1/secrets?sort=metadata.name | [json/user-a_none_sort=metadata.name.json](json/user-a_none_sort=metadata.name.json)
user-a | https://localhost:8443/v1/secrets?sort=-metadata.name | [json/user-a_none_sort=-metadata.name.json](json/user-a_none_sort=-metadata.name.json)
user-a | https://localhost:8443/v1/secrets?sort=metadata.name,metadata.namespace | [json/user-a_none_sort=metadata.name,metadata.namespace.json](json/user-a_none_sort=metadata.name,metadata.namespace.json)
user-a | https://localhost:8443/v1/secrets?sort=metadata.name,-metadata.namespace | [json/user-a_none_sort=metadata.name,-metadata.namespace.json](json/user-a_none_sort=metadata.name,-metadata.namespace.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?sort=metadata.name | [json/user-a_test-ns-1_sort=metadata.name.json](json/user-a_test-ns-1_sort=metadata.name.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?sort=-metadata.name | [json/user-a_test-ns-1_sort=-metadata.name.json](json/user-a_test-ns-1_sort=-metadata.name.json)
user-a | https://localhost:8443/v1/secrets?pagesize=8 | [json/user-a_none_pagesize=8.json](json/user-a_none_pagesize=8.json)
user-a | https://localhost:8443/v1/secrets?pagesize=8&page=2&revision=nondeterministicint | [json/user-a_none_pagesize=8&page=2&revision=nondeterministicint.json](json/user-a_none_pagesize=8&page=2&revision=nondeterministicint.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?pagesize=3 | [json/user-a_test-ns-1_pagesize=3.json](json/user-a_test-ns-1_pagesize=3.json)
user-a | https://localhost:8443/v1/secrets/test-ns-1?pagesize=3&page=2&revision=nondeterministicint | [json/user-a_test-ns-1_pagesize=3&page=2&revision=nondeterministicint.json](json/user-a_test-ns-1_pagesize=3&page=2&revision=nondeterministicint.json)
user-a | https://localhost:8443/v1/secrets?filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20 | [json/user-a_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20.json](json/user-a_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&limit=20.json)
user-a | https://localhost:8443/v1/secrets?filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=nondeterministicint&limit=20 | [json/user-a_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=nondeterministicint&limit=20.json](json/user-a_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=2&revision=nondeterministicint&limit=20.json)
user-a | https://localhost:8443/v1/secrets?filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=nondeterministictoken | [json/user-a_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=nondeterministictoken.json](json/user-a_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=6&page=1&limit=20&continue=nondeterministictoken.json)
user-b | https://localhost:8443/v1/secrets | [json/user-b_none_none.json](json/user-b_none_none.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1 | [json/user-b_test-ns-1_none.json](json/user-b_test-ns-1_none.json)
user-b | https://localhost:8443/v1/secrets/test-ns-5 | [json/user-b_test-ns-5_none.json](json/user-b_test-ns-5_none.json)
user-b | https://localhost:8443/v1/secrets?labelSelector=test-label=2 | [json/user-b_none_labelSelector=test-label=2.json](json/user-b_none_labelSelector=test-label=2.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?labelSelector=test-label=2 | [json/user-b_test-ns-1_labelSelector=test-label=2.json](json/user-b_test-ns-1_labelSelector=test-label=2.json)
user-b | https://localhost:8443/v1/secrets/test-ns-2?labelSelector=test-label=2 | [json/user-b_test-ns-2_labelSelector=test-label=2.json](json/user-b_test-ns-2_labelSelector=test-label=2.json)
user-b | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-1 | [json/user-b_none_fieldSelector=metadata.namespace=test-ns-1.json](json/user-b_none_fieldSelector=metadata.namespace=test-ns-1.json)
user-b | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-2 | [json/user-b_none_fieldSelector=metadata.namespace=test-ns-2.json](json/user-b_none_fieldSelector=metadata.namespace=test-ns-2.json)
user-b | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test1 | [json/user-b_none_fieldSelector=metadata.name=test1.json](json/user-b_none_fieldSelector=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-1 | [json/user-b_test-ns-1_fieldSelector=metadata.namespace=test-ns-1.json](json/user-b_test-ns-1_fieldSelector=metadata.namespace=test-ns-1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.namespace=test-ns-1 | [json/user-b_test-ns-2_fieldSelector=metadata.namespace=test-ns-1.json](json/user-b_test-ns-2_fieldSelector=metadata.namespace=test-ns-1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-2 | [json/user-b_test-ns-1_fieldSelector=metadata.namespace=test-ns-2.json](json/user-b_test-ns-1_fieldSelector=metadata.namespace=test-ns-2.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test1 | [json/user-b_test-ns-1_fieldSelector=metadata.name=test1.json](json/user-b_test-ns-1_fieldSelector=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.name=test1 | [json/user-b_test-ns-2_fieldSelector=metadata.name=test1.json](json/user-b_test-ns-2_fieldSelector=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets?limit=3 | [json/user-b_none_limit=3.json](json/user-b_none_limit=3.json)
user-b | https://localhost:8443/v1/secrets?limit=3&continue=nondeterministictoken | [json/user-b_none_limit=3&continue=nondeterministictoken.json](json/user-b_none_limit=3&continue=nondeterministictoken.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?limit=3 | [json/user-b_test-ns-1_limit=3.json](json/user-b_test-ns-1_limit=3.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?limit=3&continue=nondeterministictoken | [json/user-b_test-ns-1_limit=3&continue=nondeterministictoken.json](json/user-b_test-ns-1_limit=3&continue=nondeterministictoken.json)
user-b | https://localhost:8443/v1/secrets/test-ns-5?limit=3 | [json/user-b_test-ns-5_limit=3.json](json/user-b_test-ns-5_limit=3.json)
user-b | https://localhost:8443/v1/secrets?filter=metadata.name=test1 | [json/user-b_none_filter=metadata.name=test1.json](json/user-b_none_filter=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?filter=metadata.name=test1 | [json/user-b_test-ns-1_filter=metadata.name=test1.json](json/user-b_test-ns-1_filter=metadata.name=test1.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?filter=metadata.name=test6 | [json/user-b_test-ns-1_filter=metadata.name=test6.json](json/user-b_test-ns-1_filter=metadata.name=test6.json)
user-b | https://localhost:8443/v1/secrets?sort=metadata.name | [json/user-b_none_sort=metadata.name.json](json/user-b_none_sort=metadata.name.json)
user-b | https://localhost:8443/v1/secrets?sort=-metadata.name | [json/user-b_none_sort=-metadata.name.json](json/user-b_none_sort=-metadata.name.json)
user-b | https://localhost:8443/v1/secrets?sort=metadata.name,metadata.namespace | [json/user-b_none_sort=metadata.name,metadata.namespace.json](json/user-b_none_sort=metadata.name,metadata.namespace.json)
user-b | https://localhost:8443/v1/secrets?sort=-metadata.name,metadata.namespace | [json/user-b_none_sort=-metadata.name,metadata.namespace.json](json/user-b_none_sort=-metadata.name,metadata.namespace.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?sort=metadata.name | [json/user-b_test-ns-1_sort=metadata.name.json](json/user-b_test-ns-1_sort=metadata.name.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?sort=-metadata.name | [json/user-b_test-ns-1_sort=-metadata.name.json](json/user-b_test-ns-1_sort=-metadata.name.json)
user-b | https://localhost:8443/v1/secrets/test-ns-5?sort=metadata.name | [json/user-b_test-ns-5_sort=metadata.name.json](json/user-b_test-ns-5_sort=metadata.name.json)
user-b | https://localhost:8443/v1/secrets?pagesize=3 | [json/user-b_none_pagesize=3.json](json/user-b_none_pagesize=3.json)
user-b | https://localhost:8443/v1/secrets?pagesize=3&page=2&revision=nondeterministicint | [json/user-b_none_pagesize=3&page=2&revision=nondeterministicint.json](json/user-b_none_pagesize=3&page=2&revision=nondeterministicint.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?pagesize=3 | [json/user-b_test-ns-1_pagesize=3.json](json/user-b_test-ns-1_pagesize=3.json)
user-b | https://localhost:8443/v1/secrets/test-ns-1?pagesize=3&page=2&revision=nondeterministicint | [json/user-b_test-ns-1_pagesize=3&page=2&revision=nondeterministicint.json](json/user-b_test-ns-1_pagesize=3&page=2&revision=nondeterministicint.json)
user-b | https://localhost:8443/v1/secrets/test-ns-5?pagesize=3 | [json/user-b_test-ns-5_pagesize=3.json](json/user-b_test-ns-5_pagesize=3.json)
user-b | https://localhost:8443/v1/secrets?filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2 | [json/user-b_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2.json](json/user-b_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2.json)
user-b | https://localhost:8443/v1/secrets?filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=nondeterministicint | [json/user-b_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=nondeterministicint.json](json/user-b_none_filter=metadata.labels.test-label-gte=3&sort=-metadata.name&pagesize=2&page=2&revision=nondeterministicint.json)
user-c | https://localhost:8443/v1/secrets | [json/user-c_none_none.json](json/user-c_none_none.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1 | [json/user-c_test-ns-1_none.json](json/user-c_test-ns-1_none.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5 | [json/user-c_test-ns-5_none.json](json/user-c_test-ns-5_none.json)
user-c | https://localhost:8443/v1/secrets?labelSelector=test-label=2 | [json/user-c_none_labelSelector=test-label=2.json](json/user-c_none_labelSelector=test-label=2.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?labelSelector=test-label=2 | [json/user-c_test-ns-1_labelSelector=test-label=2.json](json/user-c_test-ns-1_labelSelector=test-label=2.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?labelSelector=test-label=2 | [json/user-c_test-ns-5_labelSelector=test-label=2.json](json/user-c_test-ns-5_labelSelector=test-label=2.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-1 | [json/user-c_none_fieldSelector=metadata.namespace=test-ns-1.json](json/user-c_none_fieldSelector=metadata.namespace=test-ns-1.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-2 | [json/user-c_none_fieldSelector=metadata.namespace=test-ns-2.json](json/user-c_none_fieldSelector=metadata.namespace=test-ns-2.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.namespace=test-ns-5 | [json/user-c_none_fieldSelector=metadata.namespace=test-ns-5.json](json/user-c_none_fieldSelector=metadata.namespace=test-ns-5.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test1 | [json/user-c_none_fieldSelector=metadata.name=test1.json](json/user-c_none_fieldSelector=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets?fieldSelector=metadata.name=test5 | [json/user-c_none_fieldSelector=metadata.name=test5.json](json/user-c_none_fieldSelector=metadata.name=test5.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-1 | [json/user-c_test-ns-1_fieldSelector=metadata.namespace=test-ns-1.json](json/user-c_test-ns-1_fieldSelector=metadata.namespace=test-ns-1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-2?fieldSelector=metadata.namespace=test-ns-1 | [json/user-c_test-ns-2_fieldSelector=metadata.namespace=test-ns-1.json](json/user-c_test-ns-2_fieldSelector=metadata.namespace=test-ns-1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.namespace=test-ns-2 | [json/user-c_test-ns-1_fieldSelector=metadata.namespace=test-ns-2.json](json/user-c_test-ns-1_fieldSelector=metadata.namespace=test-ns-2.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test1 | [json/user-c_test-ns-1_fieldSelector=metadata.name=test1.json](json/user-c_test-ns-1_fieldSelector=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?fieldSelector=metadata.name=test1 | [json/user-c_test-ns-5_fieldSelector=metadata.name=test1.json](json/user-c_test-ns-5_fieldSelector=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?fieldSelector=metadata.name=test5 | [json/user-c_test-ns-1_fieldSelector=metadata.name=test5.json](json/user-c_test-ns-1_fieldSelector=metadata.name=test5.json)
user-c | https://localhost:8443/v1/secrets?limit=3 | [json/user-c_none_limit=3.json](json/user-c_none_limit=3.json)
user-c | https://localhost:8443/v1/secrets?limit=3&continue=nondeterministictoken | [json/user-c_none_limit=3&continue=nondeterministictoken.json](json/user-c_none_limit=3&continue=nondeterministictoken.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?limit=3 | [json/user-c_test-ns-1_limit=3.json](json/user-c_test-ns-1_limit=3.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?limit=3 | [json/user-c_test-ns-5_limit=3.json](json/user-c_test-ns-5_limit=3.json)
user-c | https://localhost:8443/v1/secrets?filter=metadata.name=test1 | [json/user-c_none_filter=metadata.name=test1.json](json/user-c_none_filter=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?filter=metadata.name=test1 | [json/user-c_test-ns-1_filter=metadata.name=test1.json](json/user-c_test-ns-1_filter=metadata.name=test1.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?filter=metadata.name=test3 | [json/user-c_test-ns-1_filter=metadata.name=test3.json](json/user-c_test-ns-1_filter=metadata.name=test3.json)
user-c | https://localhost:8443/v1/secrets?sort=metadata.name | [json/user-c_none_sort=metadata.name.json](json/user-c_none_sort=metadata.name.json)
user-c | https://localhost:8443/v1/secrets?sort=-metadata.name | [json/user-c_none_sort=-metadata.name.json](json/user-c_none_sort=-metadata.name.json)
user-c | https://localhost:8443/v1/secrets?sort=metadata.name,metadata.namespace | [json/user-c_none_sort=metadata.name,metadata.namespace.json](json/user-c_none_sort=metadata.name,metadata.namespace.json)
user-c | https://localhost:8443/v1/secrets?sort=metadata.name,-metadata.namespace | [json/user-c_none_sort=metadata.name,-metadata.namespace.json](json/user-c_none_sort=metadata.name,-metadata.namespace.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?sort=metadata.name | [json/user-c_test-ns-1_sort=metadata.name.json](json/user-c_test-ns-1_sort=metadata.name.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?sort=-metadata.name | [json/user-c_test-ns-1_sort=-metadata.name.json](json/user-c_test-ns-1_sort=-metadata.name.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?sort=metadata.name,metadata.namespace | [json/user-c_test-ns-1_sort=metadata.name,metadata.namespace.json](json/user-c_test-ns-1_sort=metadata.name,metadata.namespace.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?sort=metadata.name,-metadata.namespace | [json/user-c_test-ns-1_sort=metadata.name,-metadata.namespace.json](json/user-c_test-ns-1_sort=metadata.name,-metadata.namespace.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?sort=metadata.name | [json/user-c_test-ns-5_sort=metadata.name.json](json/user-c_test-ns-5_sort=metadata.name.json)
user-c | https://localhost:8443/v1/secrets?pagesize=3 | [json/user-c_none_pagesize=3.json](json/user-c_none_pagesize=3.json)
user-c | https://localhost:8443/v1/secrets?pagesize=3&page=2&revision=nondeterministicint | [json/user-c_none_pagesize=3&page=2&revision=nondeterministicint.json](json/user-c_none_pagesize=3&page=2&revision=nondeterministicint.json)
user-c | https://localhost:8443/v1/secrets/test-ns-1?pagesize=3 | [json/user-c_test-ns-1_pagesize=3.json](json/user-c_test-ns-1_pagesize=3.json)
user-c | https://localhost:8443/v1/secrets/test-ns-5?pagesize=3 | [json/user-c_test-ns-5_pagesize=3.json](json/user-c_test-ns-5_pagesize=3.json)
user-c | https://localhost:8443/v1/secrets?filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1 | [json/user-c_none_filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1.json](json/user-c_none_filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1.json)
user-c | https://localhost:8443/v1/secrets?filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=nondeterministicint | [json/user-c_none_filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=nondeterministicint.json](json/user-c_none_filter=metadata.namespace=test-ns-3&sort=-metadata.name&pagesize=1&page=2&revision=nondeterministicint.json)

