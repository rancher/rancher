# how to write RBAC-related testing functions
A set of global variables and functions are introduced to make it easy
to write tests for rancher's features around RBAC.

When the testing environment is initializing, the following resources are created:
- a new project that new users and user roles are bound to
- a namespace in that project
- five standard users
- another unshared project that no of those users are bound to
- a namespace and a workload in this project

Those users are assigned to different roles:
 - cluster owner
 - cluster member
 - project owner
 - project member
 - project read-only member

The following functions are provided:
- to get a specific user: `rbac_get_user_by_role(role_template_id)`
- to get a specific user's token: `rbac_get_user_token_by_role(role_template_id)`
- to get the project: `rbac_get_project()`
- to get the namespace in the project: `rbac_get_namespace()`
- to get the unshared proejct: `rbac_get_unshared_project()`
- to get the unshared namespace: `rbac_get_unshared_ns()`
- to get the unshared workload: `rbac_get_unshared_workload()`


The following are some functions using the above resources in `test_workload.py`:
- test_wl_rbac_cluster_owner
- test_wl_rbac_cluster_member
- test_wl_rbac_project_member


# how to use the new fixture: `remove_resource`
This fixture handles the deletion of any resource that is created in the
function. Here is an example:
```
def test_wl_rbac_project_member(remove_resource):
    ...
    workload = p_client.create_workload(name=name, containers=con, namespaceId=ns.id)
    remove_resource(workload)
    ...
```
First, we need to pass the fixture as an argument to the function to registry it to the scope of the function;
second, we need to call this fixture after creating a new resource, in this example it is a workload.
In such way, the workload will be removed automatically when this test finishes no matter it succeeds or fails.
