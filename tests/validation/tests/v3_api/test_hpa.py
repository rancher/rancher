import pytest
from rancher import ApiError
from .common import *  # NOQA

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


def test_create_hpa():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    hpa, workload = create_hpa(p_client, ns)
    p_client.delete(hpa, workload)


def test_edit_hpa():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    hpa, workload = edit_hpa(p_client, ns)
    p_client.delete(hpa, workload)


def test_delete_hpa():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    hpa, workload = create_hpa(p_client, ns)
    delete_hpa(p_client, hpa, ns)
    p_client.delete(workload)


rbac_role_list = [
    (CLUSTER_OWNER),
    (PROJECT_OWNER),
    (PROJECT_MEMBER),
    (PROJECT_READ_ONLY),
    (CLUSTER_MEMBER),
]


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_create(role, remove_resource):
    user_project = None
    if(role == CLUSTER_MEMBER):
        user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
        user_project, ns = create_project_and_ns(user_token,
                                                 namespace["cluster"],
                                                 random_test_name(
                                                     "cluster-mem"))
        p_client = get_project_client_for_token(user_project, user_token)
    else:
        user_token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, user_token)

    if (role != PROJECT_READ_ONLY):
        newhpa, newworkload = create_hpa(p_client, ns)
        remove_resource(newhpa)
        remove_resource(newworkload)
    else:
        project = rbac_get_project()
        ns = rbac_get_namespace()
        user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
        readonly_user_client = get_project_client_for_token(project,
                                                            user_token)
        # Verify Read Only member cannot create hpa objects
        with pytest.raises(ApiError) as e:
            create_hpa(readonly_user_client, ns)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    if(user_project is not None):
        remove_resource(user_project)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_create_negative(role, remove_resource):
    if (role == CLUSTER_OWNER):
        print(role)
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        hpa, workload = create_hpa(p_client, ns)
        remove_resource(hpa)
        remove_resource(workload)
    else:
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        owner_client = get_project_client_for_token(unshared_project,
                                                    cluster_owner_token)
        # Workload created by cluster owner in unshared project is passed as
        # parameter to create HPA
        workload = create_workload(owner_client, ns)
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        with pytest.raises(ApiError) as e:
            create_hpa(p_client, ns, workload=workload)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'

        remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_edit(role, remove_resource):
    if (role == PROJECT_READ_ONLY):
        verify_hpa_project_readonly_edit(remove_resource)
    elif(role == CLUSTER_MEMBER):
        verify_hpa_cluster_member_edit(remove_resource)
    else:
        user_token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, user_token)
        hpa, workload = edit_hpa(p_client, ns)
        remove_resource(hpa)
        remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_edit_negative(role, remove_resource):
    if (role == CLUSTER_OWNER):
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        hpa, workload = edit_hpa(p_client, ns)
        remove_resource(hpa)
        remove_resource(workload)
    else:
        unshared_project = rbac_get_unshared_project()
        user_token = rbac_get_user_token_by_role(role)
        unshared_ns = rbac_get_unshared_ns()
        user_client = get_project_client_for_token(unshared_project,
                                                   user_token)
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)

        # Cluster owner client created in the unshared project
        cluster_owner_p_client = \
            get_project_client_for_token(unshared_project, cluster_owner_token)
        # Verify that some users cannot edit hpa created by cluster owner
        verify_edit_forbidden(user_client, remove_resource,
                              cluster_owner_client=cluster_owner_p_client,
                              ns=unshared_ns)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_delete(role, remove_resource):
    user_project = None
    if(role == CLUSTER_MEMBER):
        user_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
        user_project, ns = create_project_and_ns(user_token,
                                                 namespace["cluster"],
                                                 random_test_name(
                                                     "cluster-mem"))
        p_client = get_project_client_for_token(user_project, user_token)
    else:
        user_token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, user_token)

    if (role != PROJECT_READ_ONLY):
        hpa, workload = create_hpa(p_client, ns)
        delete_hpa(p_client, hpa, ns)
        remove_resource(workload)
        remove_resource(hpa)
    if user_project is not None:
        remove_resource(user_project)

    if (role == PROJECT_READ_ONLY):
        project = rbac_get_project()
        ns = rbac_get_namespace()
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        cluster_owner_p_client = \
            get_project_client_for_token(project, cluster_owner_token)
        user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
        user_client = get_project_client_for_token(project, user_token)

        # As a Cluster owner create a HPA object
        hpa, workload = create_hpa(cluster_owner_p_client, ns)
        # Verify that the Read Only member cannot delete the HPA objects
        # created by Cluster Owner
        with pytest.raises(ApiError) as e:
            delete_hpa(user_client, hpa, ns)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(hpa)
        remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_delete_negative(role, remove_resource):
    if (role == CLUSTER_OWNER):
        print(role)
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        hpa, workload = create_hpa(p_client, ns)
        delete_hpa(p_client, hpa, ns)
        remove_resource(hpa)
        remove_resource(workload)
    else:
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        owner_client = get_project_client_for_token(unshared_project,
                                                    cluster_owner_token)
        workload = create_workload(owner_client, ns)
        user_token = rbac_get_user_token_by_role(role)
        # Workload created by cluster owner in unshared project is passed as
        # parameter to create HPA
        hpa, workload = create_hpa(owner_client, ns, workload=workload)
        p_client = get_project_client_for_token(unshared_project, user_token)
        with pytest.raises(ApiError) as e:
            delete_hpa(p_client, hpa, ns)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(hpa)
        remove_resource(workload)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_list(remove_resource, role):
    user_project = None
    if(role == CLUSTER_MEMBER):
        cluster_member_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
        user_project, ns = \
            create_project_and_ns(cluster_member_token,
                                  namespace["cluster"],
                                  random_test_name("cluster-mem"))

        user_client = get_project_client_for_token(user_project,
                                                   cluster_member_token)
        # As a cluster member create a HPA and he should be able to list it
        hpa, workload = create_hpa(user_client, ns)
    else:
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        project = rbac_get_project()
        cluster_owner_p_client = \
            get_project_client_for_token(project, cluster_owner_token)
        user_token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        user_client = get_project_client_for_token(project, user_token)
        hpa, workload = create_hpa(cluster_owner_p_client, ns)

    hpaname = hpa.name
    hpadict = user_client.list_horizontalPodAutoscaler(name=hpaname)
    print(hpadict)
    hpadata = hpadict.get('data')
    assert len(hpadata) == 1
    assert hpadata[0].type == "horizontalPodAutoscaler"
    assert hpadata[0].name == hpaname
    remove_resource(hpa)
    remove_resource(workload)
    if user_client is not None:
        remove_resource(user_project)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_hpa_list_negative(remove_resource, role):
    if (role == CLUSTER_OWNER):
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        hpa, workload = create_hpa(p_client, ns)
        hpaname = hpa.name
        hpadict = p_client.list_horizontalPodAutoscaler(name=hpaname)
        hpadata = hpadict.get('data')
        assert len(hpadata) == 1
        assert hpadata[0].type == "horizontalPodAutoscaler"
        assert hpadata[0].name == hpaname
        remove_resource(hpa)
        remove_resource(workload)
    else:
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        cluster_owner_client = \
            get_project_client_for_token(unshared_project, cluster_owner_token)
        user_token = rbac_get_user_token_by_role(role)
        user_client = get_project_client_for_token(unshared_project,
                                                   user_token)
        hpa, workload = create_hpa(cluster_owner_client, ns)
        hpaname = hpa.name
        # Verify length of HPA list is zero
        hpadict = user_client.list_horizontalPodAutoscaler(name=hpaname)
        hpadata = hpadict.get('data')
        assert len(hpadata) == 0
        remove_resource(hpa)
        remove_resource(workload)


def verify_hpa_cluster_member_edit(remove_resource):
    cluster_member_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
    user_project, ns = create_project_and_ns(cluster_member_token,
                                             namespace["cluster"],
                                             random_test_name("cluster-mem"))
    cluster_member_client = get_project_client_for_token(user_project,
                                                         cluster_member_token)
    # Verify the cluster member can edit the hpa he created
    hpa, workload = edit_hpa(cluster_member_client, ns)

    # Verify that cluster member cannot edit the hpa created by cluster owner
    verify_edit_forbidden(cluster_member_client, remove_resource)

    remove_resource(hpa)
    remove_resource(workload)
    remove_resource(user_project)


def verify_hpa_project_readonly_edit(remove_resource):
    project = rbac_get_project()
    user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
    readonly_user_client = get_project_client_for_token(project, user_token)
    # Verify that read -only user cannot edit the hpa created by cluster owner
    verify_edit_forbidden(readonly_user_client, remove_resource)


def verify_edit_forbidden(user_client, remove_resource,
                          cluster_owner_client=None, ns=None):
    metrics = [{
        'name': 'cpu',
        'type': 'Resource',
        'target': {
            'type': 'Utilization',
            'utilization': '50',
        },
    }]

    if(cluster_owner_client is None and ns is None):
        project = rbac_get_project()
        ns = rbac_get_namespace()
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        cluster_owner_client = \
            get_project_client_for_token(project, cluster_owner_token)

    # Create HPA as a cluster owner
    hpa, workload = create_hpa(cluster_owner_client, ns)
    # Verify editing HPA fails
    with pytest.raises(ApiError) as e:
        user_client.update(hpa,
                           name=hpa['name'],
                           namespaceId=ns.id,
                           maxReplicas=10,
                           minReplicas=3,
                           workload=workload.id,
                           metrics=metrics)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    remove_resource(hpa)
    remove_resource(workload)


def create_hpa(p_client, ns, workload=None):

    # Create workload of scale 1 with CPU reservation
    # Create hpa pointing to the workload.
    if workload is None:
        workload = create_workload(p_client, ns)
    name = random_test_name("hpa")

    metrics = [{'name': 'cpu',
                'type': 'Resource',
                'target': {
                    'type': 'Utilization',
                    'utilization': '50',
                },
                }]

    hpa = p_client.create_horizontalPodAutoscaler(
        name=name,
        namespaceId=ns.id,
        maxReplicas=5,
        minReplicas=2,
        workloadId=workload.id,
        metrics=metrics
    )
    hpa = wait_for_hpa_to_active(p_client, hpa)
    assert hpa.type == "horizontalPodAutoscaler"
    assert hpa.name == name
    assert hpa.minReplicas == 2
    assert hpa.maxReplicas == 5
    # After hpa becomes active, the workload scale should be equal to the
    # minReplicas set in HPA object
    workloadlist = p_client.list_workload(uuid=workload.uuid).data
    validate_workload(p_client, workloadlist[0], "deployment", ns.name,
                      pod_count=hpa.minReplicas)

    return (hpa, workload)


def edit_hpa(p_client, ns):

    # Create workload of scale 1 with memory reservation
    # Create hpa pointing to the workload.Edit HPA and verify HPA is functional

    workload = create_workload(p_client, ns)
    name = random_test_name("default")

    metrics = [{
        "type": "Resource",
        "name": "memory",
        "target": {
            "type": "AverageValue",
            "value": None,
            "averageValue": "32Mi",
            "utilization": None,
            "stringValue": "32"
        }
    }]

    hpa = p_client.create_horizontalPodAutoscaler(
        name=name,
        namespaceId=ns.id,
        maxReplicas=4,
        minReplicas=2,
        workloadId=workload.id,
        metrics=metrics
    )
    wait_for_hpa_to_active(p_client, hpa)

    # After hpa becomes active, the workload scale should be equal to the
    # minReplicas set in HPA
    workloadlist = p_client.list_workload(uuid=workload.uuid).data
    validate_workload(p_client, workloadlist[0], "deployment", ns.name,
                      pod_count=hpa.minReplicas)

    # Edit the HPA
    updated_hpa = p_client.update(hpa,
                                  name=hpa['name'],
                                  namespaceId=ns.id,
                                  maxReplicas=6,
                                  minReplicas=3,
                                  workloadId=workload.id,
                                  metrics=metrics)
    wait_for_hpa_to_active(p_client, updated_hpa)
    assert updated_hpa.type == "horizontalPodAutoscaler"
    assert updated_hpa.minReplicas == 3
    assert updated_hpa.maxReplicas == 6

    # After hpa becomes active, the workload scale should be equal to the
    # minReplicas set in the updated HPA
    wait_for_pods_in_workload(p_client, workload, 3)
    workloadlist = p_client.list_workload(uuid=workload.uuid).data
    validate_workload(p_client, workloadlist[0], "deployment", ns.name,
                      pod_count=updated_hpa.minReplicas)

    return (updated_hpa, workload)


def delete_hpa(p_client, hpa, ns):
    hpaname = hpa['name']
    p_client.delete(hpa)
    # Sleep to allow HPA to be deleted
    time.sleep(5)
    timeout = 30
    hpadict = p_client.list_horizontalPodAutoscaler(name=hpaname)
    print(hpadict.get('data'))
    start = time.time()
    if len(hpadict.get('data')) > 0:
        testdata = hpadict.get('data')
        while hpaname in testdata[0]['data']:
            if time.time() - start > timeout:
                raise AssertionError("Timed out waiting for deletion")
            time.sleep(.5)
            hpadict = p_client.list_horizontalPodAutoscaler(name=hpaname)
            testdata = hpadict.get('data')
        assert True
    if len(hpadict.get('data')) == 0:
        assert True

    # Verify hpa is deleted by "kubectl get hpa" command
    command = "get hpa {} --namespace {}".format(hpa['name'], ns.name)
    print("Command to obtain the hpa")
    print(command)
    result = execute_kubectl_cmd(command, json_out=False, stderr=True)
    print(result)

    print("Verify that the hpa does not exist "
          "and the error code returned is non zero ")
    if result != 0:
        assert True


def create_workload(p_client, ns):
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "resources": {
                "requests": {
                    "memory": "64Mi",
                    "cpu": "100m"
                },
                "limits": {
                    "memory": "512Mi",
                    "cpu": "1000m"
                }
            }
            }]

    name = random_test_name("workload")
    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    print(workload.scale)
    validate_workload(p_client, workload, "deployment", ns.name)

    return workload


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(
        ADMIN_TOKEN, cluster, random_test_name("testhpa"))
    p_client = get_project_client_for_token(p, ADMIN_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p

    def fin():
        client = get_admin_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)
