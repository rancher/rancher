import pytest
from rancher import ApiError
from .common import *  # NOQA

namespace = {"p_client": None,
             "ns": None,
             "cluster": None,
             "project": None,
             "pv": None,
             "pvc": None}

# this is the path to the mounted dir in the pod(workload)
MOUNT_PATH = "/var/nfs"
# if True then delete the NFS after finishing tests, otherwise False
DELETE_NFS = eval(os.environ.get('RANCHER_DELETE_NFS', "True"))


rbac_role_list = [
    (CLUSTER_OWNER),
    (PROJECT_OWNER),
    (PROJECT_MEMBER),
    (PROJECT_READ_ONLY),
    (CLUSTER_MEMBER),
]


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pv_create(role, remove_resource):
    nfs_ip = namespace["nfs_ip"]
    cluster = namespace["cluster"]
    url = CATTLE_TEST_URL + "/v3/clusters/" + cluster.id + "/persistentvolume"
    if (role == CLUSTER_OWNER):
        user_token = rbac_get_user_token_by_role(role)
        # Persistent volume can be created only using cluster client
        owner_clusterclient = get_cluster_client_for_token(cluster, user_token)
        pv = create_pv(owner_clusterclient, nfs_ip)
        remove_resource(pv)
    else:
        # Users other than cluster owner cannot create persistent volume
        # Other user clients do not have attribute to create persistent volume
        # Hence a post request is made as below
        user_token = rbac_get_user_token_by_role(role)
        headers = {"Content-Type": "application/json",
                   "Accept": "application/json",
                   "Authorization": "Bearer " + user_token}
        pv_config = {"type": "persistentVolume",
                     "accessModes": ["ReadWriteOnce"],
                     "name": "testpv",
                     "nfs": {"readOnly": "false",
                             "type": "nfsvolumesource",
                             "path": NFS_SERVER_MOUNT_PATH,
                             "server": nfs_ip
                             },
                     "capacity": {"storage": "10Gi"}
                     }

        response = requests.post(url, json=pv_config, verify=False,
                                 headers=headers)
        assert response.status_code == 403
        jsonresponse = json.loads(response.content)
        assert jsonresponse['code'] == "Forbidden"


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pv_list(role, remove_resource):
    # All users can list the persistent volume which is at the cluster level
    # A get request is performed as user clients do not have attrribute to list
    # persistent volumes
    nfs_ip = namespace["nfs_ip"]
    cluster = namespace["cluster"]

    user_token = rbac_get_user_token_by_role(role)
    owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    owner_clusterclient = get_cluster_client_for_token(cluster, owner_token)
    pv = create_pv(owner_clusterclient, nfs_ip)
    pvname = pv['name']
    url = CATTLE_TEST_URL + "/v3/cluster/" + cluster.id +\
        "/persistentvolumes/" + pvname

    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json",
        "Authorization": "Bearer " + user_token
    }
    response = requests.get(url, verify=False,
                            headers=headers)
    jsonresponse = json.loads(response.content)
    assert response.status_code == 200
    assert jsonresponse['type'] == "persistentVolume"
    assert jsonresponse['name'] == pvname
    remove_resource(pv)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pv_edit(role):
    nfs_ip = namespace["nfs_ip"]
    cluster = namespace["cluster"]
    clusterowner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    owner_clusterclient = get_cluster_client_for_token(cluster,
                                                       clusterowner_token)
    if(role == CLUSTER_OWNER):
        # Verify editing of PV as a Cluster Owner succeeds
        edit_pv(owner_clusterclient, nfs_ip, owner_clusterclient)

    else:
        user_token = rbac_get_user_token_by_role(role)
        user_clusterclient = get_cluster_client_for_token(cluster,
                                                          user_token)
        # Verify editing of PV is forbidden for other roles
        with pytest.raises(ApiError) as e:
            edit_pv(user_clusterclient, nfs_ip, owner_clusterclient)
        print(e.value.error.code)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pv_delete(role, remove_resource):
    nfs_ip = namespace["nfs_ip"]
    cluster = namespace["cluster"]
    clusterowner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
    owner_clusterclient = get_cluster_client_for_token(cluster,
                                                       clusterowner_token)
    if(role == CLUSTER_OWNER):
        pv = create_pv(owner_clusterclient, nfs_ip)
        delete_pv(owner_clusterclient, pv)
    else:
        user_token = rbac_get_user_token_by_role(role)
        user_clusterclient = get_cluster_client_for_token(cluster,
                                                          user_token)
        # As a Cluster Owner create PV object using cluster client
        pv = create_pv(owner_clusterclient, nfs_ip)
        # Verify other roles cannot delete the PV object
        with pytest.raises(ApiError) as e:
            delete_pv(user_clusterclient, pv)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(pv)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_create(role, remove_resource):
    user_project = None
    nfs_ip = namespace["nfs_ip"]
    cluster_client = namespace["cluster_client"]
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
        pv, pvc = create_pv_pvc(p_client, ns, nfs_ip, cluster_client)
        remove_resource(pv)
        remove_resource(pvc)
    else:
        project = rbac_get_project()
        ns = rbac_get_namespace()
        user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
        readonly_user_client = get_project_client_for_token(project,
                                                            user_token)
        # Verify Read Only member cannot create PVC objects
        with pytest.raises(ApiError) as e:
            create_pv_pvc(readonly_user_client, ns, nfs_ip, cluster_client)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
    if(user_project is not None):
        remove_resource(user_project)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_create_negative(role, remove_resource):
    nfs_ip = namespace["nfs_ip"]
    cluster_client = namespace["cluster_client"]
    if (role == CLUSTER_OWNER):
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        pv, pvc = create_pv_pvc(p_client, ns, nfs_ip, cluster_client)
        remove_resource(pv)
        remove_resource(pvc)
    else:
        unshared_project = rbac_get_unshared_project()
        user_token = rbac_get_user_token_by_role(role)
        ns = rbac_get_unshared_ns()
        p_client = get_project_client_for_token(unshared_project, user_token)
        # Verify other members cannot create PVC objects
        with pytest.raises(ApiError) as e:
            create_pv_pvc(p_client, ns, nfs_ip, cluster_client)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_edit_negative(role, remove_resource):
        # Editing of volume claims is not allowed for any role,
        # We are verifying that editing is forbidden in shared
        # and unshared projects

        nfs_ip = namespace["nfs_ip"]
        cluster_client = namespace["cluster_client"]
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        user_token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, user_token)
        # Cluster owner client created in the shared project
        owner_client = \
            get_project_client_for_token(project, cluster_owner_token)
        edit_pvc(p_client, ns, nfs_ip, cluster_client, owner_client)

        unshared_project = rbac_get_unshared_project()
        user_token = rbac_get_user_token_by_role(role)
        unshared_ns = rbac_get_unshared_ns()
        user_client = get_project_client_for_token(unshared_project,
                                                   user_token)

        # Cluster owner client created in the unshared project
        owner_client = \
            get_project_client_for_token(unshared_project, cluster_owner_token)

        nfs_ip = namespace["nfs_ip"]
        cluster_client = namespace["cluster_client"]
        edit_pvc(user_client, unshared_ns, nfs_ip, cluster_client,
                 owner_client)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_delete(role, remove_resource):
    nfs_ip = namespace["nfs_ip"]
    cluster_client = namespace["cluster_client"]
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
        pv, pvc = create_pv_pvc(p_client, ns, nfs_ip, cluster_client)
        delete_pvc(p_client, pvc, ns)
        remove_resource(pv)
    if user_project is not None:
        remove_resource(user_project)

    if (role == PROJECT_READ_ONLY):
        project = rbac_get_project()
        ns = rbac_get_namespace()
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        cluster_owner_p_client = \
            get_project_client_for_token(project, cluster_owner_token)
        user_token = rbac_get_user_token_by_role(PROJECT_READ_ONLY)
        readonly_client = get_project_client_for_token(project, user_token)

        # As a Cluster owner create a PVC object
        pv, pvc = create_pv_pvc(cluster_owner_p_client, ns, nfs_ip,
                                cluster_client)
        # Verify that the Read Only member cannot delete the PVC objects
        # created by Cluster Owner
        with pytest.raises(ApiError) as e:
            delete_pvc(readonly_client, pvc, ns)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(pv)
        remove_resource(pvc)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_delete_negative(role, remove_resource):
    nfs_ip = namespace["nfs_ip"]
    cluster_client = namespace["cluster_client"]
    if (role == CLUSTER_OWNER):
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        pv, pvc = create_pv_pvc(p_client, ns, nfs_ip,
                                cluster_client)
        delete_pvc(p_client, pvc, ns)
        remove_resource(pv)
    else:
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        owner_client = get_project_client_for_token(unshared_project,
                                                    cluster_owner_token)
        user_token = rbac_get_user_token_by_role(role)
        # As a Cluster Owner create pv, pvc
        pv, pvc = create_pv_pvc(owner_client, ns, nfs_ip,
                                cluster_client)
        user_client = get_project_client_for_token(unshared_project,
                                                   user_token)
        with pytest.raises(ApiError) as e:
            delete_pvc(user_client, pvc, ns)
        assert e.value.error.status == 403
        assert e.value.error.code == 'Forbidden'
        remove_resource(pv)
        remove_resource(pvc)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_list(remove_resource, role):
    user_project = None
    nfs_ip = namespace["nfs_ip"]
    cluster_client = namespace["cluster_client"]
    if(role == CLUSTER_MEMBER):
        cluster_member_token = rbac_get_user_token_by_role(CLUSTER_MEMBER)
        user_project, ns = \
            create_project_and_ns(cluster_member_token,
                                  namespace["cluster"],
                                  random_test_name("cluster-mem"))

        user_client = get_project_client_for_token(user_project,
                                                   cluster_member_token)
        # As a cluster member create a PVC and he should be able to list it
        pv, pvc = create_pv_pvc(user_client, ns, nfs_ip, cluster_client)
    else:
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        project = rbac_get_project()
        cluster_owner_p_client = \
            get_project_client_for_token(project, cluster_owner_token)
        user_token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        user_client = get_project_client_for_token(project, user_token)
        # As a Cluster Owner create pv, pvc
        pv, pvc = create_pv_pvc(cluster_owner_p_client, ns, nfs_ip,
                                cluster_client)

    pvcname = pvc["name"]
    pvcdict = user_client.list_persistentVolumeClaim(name=pvcname)
    print(pvcdict)
    pvcdata = pvcdict['data']
    assert len(pvcdata) == 1
    assert pvcdata[0].type == "persistentVolumeClaim"
    assert pvcdata[0].name == pvcname
    remove_resource(pvc)
    remove_resource(pv)
    if user_client is not None:
        remove_resource(user_project)


@if_test_rbac
@pytest.mark.parametrize("role", rbac_role_list)
def test_rbac_pvc_list_negative(remove_resource, role):
    nfs_ip = namespace["nfs_ip"]
    cluster_client = namespace["cluster_client"]
    if (role == CLUSTER_OWNER):
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        user_token = rbac_get_user_token_by_role(role)
        p_client = get_project_client_for_token(unshared_project, user_token)
        pv, pvc = create_pv_pvc(p_client, ns, nfs_ip, cluster_client)
        pvcname = pvc['name']
        pvcdict = p_client.list_persistentVolumeClaim(name=pvcname)
        pvcdata = pvcdict.get('data')
        assert len(pvcdata) == 1
        assert pvcdata[0].type == "persistentVolumeClaim"
        assert pvcdata[0].name == pvcname
        remove_resource(pv)
        remove_resource(pvc)
    else:
        cluster_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        unshared_project = rbac_get_unshared_project()
        ns = rbac_get_unshared_ns()
        cluster_owner_client = \
            get_project_client_for_token(unshared_project, cluster_owner_token)
        user_token = rbac_get_user_token_by_role(role)
        user_client = get_project_client_for_token(unshared_project,
                                                   user_token)
        # As a Cluster Owner create pv, pvc
        pv, pvc = create_pv_pvc(cluster_owner_client, ns, nfs_ip,
                                cluster_client)
        pvcname = pvc['name']
        # Verify length of PVC list is zero for users with other roles
        pvcdict = user_client.list_horizontalPodAutoscaler(name=pvcname)
        pvcdata = pvcdict.get('data')
        assert len(pvcdata) == 0
        remove_resource(pv)
        remove_resource(pvcname)


def edit_pvc(user_client, ns, nfs_ip, cluster_client,
             cluster_owner_client):
    # Create pv and pvc as cluster owner which will be used during edit_pvc
    # negative test cases since roles other than cluster owner cannot
    # create pvc in unshared projects

    pv, pvc = create_pv_pvc(cluster_owner_client, ns, nfs_ip, cluster_client)

    updated_pvc_config = {
        "accessModes": ["ReadWriteOnce"],
        "name": pvc['name'],
        "volumeId": pv.id,
        "namespaceId": ns.id,
        "storageClassId": "",
        "resources": {"requests": {"storage": "15Gi"}}
    }
    with pytest.raises(ApiError) as e:
        user_client.update(pvc, updated_pvc_config)
    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    cluster_owner_client.delete(pvc)
    cluster_client.delete(pv)


def delete_pvc(client, pvc, ns):
    pvcname = pvc['name']
    print(pvcname)
    client.delete(pvc)
    # Sleep to allow PVC to be deleted
    time.sleep(5)
    timeout = 30
    pvcdict = client.list_persistentVolumeClaim(name=pvcname)
    start = time.time()
    if len(pvcdict.get('data')) > 0:
        testdata = pvcdict.get('data')
        print(testdata)
        while pvcname in testdata[0]:
            if time.time() - start > timeout:
                raise AssertionError("Timed out waiting for deletion")
            time.sleep(.5)
            pvcdict = client.list_persistentVolumeClaim(name=pvcname)
            testdata = pvcdict.get('data')
        assert True
    assert len(pvcdict.get('data')) == 0, "Failed to delete the PVC"

    # Verify pvc is deleted by "kubectl get pvc" command
    command = "get pvc {} --namespace {}".format(pvc['name'], ns.name)
    print("Command to obtain the pvc")
    print(command)
    result = execute_kubectl_cmd(command, json_out=False, stderr=True)
    print(result)

    print("Verify that the pvc does not exist "
          "and the error code returned is non zero ")
    assert result != 0, "Result should be a non zero value"


def delete_pv(cluster_client, pv):
    pvname = pv['name']
    print(pvname)
    cluster_client.delete(pv)
    # Sleep to allow PVC to be deleted
    time.sleep(5)
    timeout = 30
    pvdict = cluster_client.list_persistent_volume(name=pvname)
    print(pvdict)
    start = time.time()
    if len(pvdict.get('data')) > 0:
        testdata = pvdict.get('data')
        print(testdata)
        while pvname in testdata[0]:
            if time.time() - start > timeout:
                raise AssertionError("Timed out waiting for deletion")
            time.sleep(.5)
            pvcdict = cluster_client.list_persistent_volume(name=pvname)
            testdata = pvcdict.get('data')
        assert True
    assert len(pvdict.get('data')) == 0, "Failed to delete the PV"

    # Verify pv is deleted by "kubectl get pv" command
    command = "get pv {} ".format(pv['name'])
    print("Command to obtain the pvc")
    print(command)
    result = execute_kubectl_cmd(command, json_out=False, stderr=True)
    print(result)
    print("Verify that the pv does not exist "
          "and the error code returned is non zero ")
    assert result != 0, "Result should be a non zero value"


def edit_pv(client, nfs_ip, cluster_owner_client):
    pv = create_pv(cluster_owner_client, nfs_ip)

    updated_pv_config = {
        "type": "persistentVolume",
        "accessModes": ["ReadWriteOnce"],
        "name": pv['name'],
        "nfs": {"readOnly": "false",
                "type": "nfsvolumesource",
                "path": NFS_SERVER_MOUNT_PATH,
                "server": nfs_ip
                },
        "capacity": {"storage": "20Gi"}
    }
    updated_pv = client.update(pv, updated_pv_config)
    capacitydict = updated_pv['capacity']
    assert capacitydict['storage'] == '20Gi'
    assert pv['type'] == 'persistentVolume'
    cluster_owner_client.delete(updated_pv)


@pytest.fixture(scope="module", autouse="True")
def volumes_setup(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    project, ns = create_project_and_ns(USER_TOKEN, cluster, "project-volumes")
    p_client = get_project_client_for_token(project, USER_TOKEN)
    nfs_node = provision_nfs_server()
    nfs_ip = nfs_node.get_public_ip()
    print("IP of the NFS Server: ", nfs_ip)

    # add  persistent volume to the cluster
    cluster_client = get_cluster_client_for_token(cluster, USER_TOKEN)

    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = project
    namespace["nfs_ip"] = nfs_ip
    namespace["cluster_client"] = cluster_client

    def fin():
        cluster_client = get_cluster_client_for_token(namespace["cluster"],
                                                      USER_TOKEN)
        cluster_client.delete(namespace["project"])
        cluster_client.delete(namespace["pv"])
        if DELETE_NFS is True:
            AmazonWebServices().delete_node(nfs_node)
    request.addfinalizer(fin)
