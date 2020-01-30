import pytest
from .common import *  # NOQA

namespace = {"p_client": None,
             "ns": None,
             "cluster": None,
             "project": None,
             "pv": None,
             "pvc": None}

# this is the path to the mounted dir in the NFS server
NFS_SERVER_MOUNT_PATH = "/nfs"
# this is the path to the mounted dir in the pod(workload)
MOUNT_PATH = "/var/nfs"
# if True then delete the NFS after finishing tests, otherwise False
DELETE_NFS = eval(os.environ.get('RANCHER_DELETE_NFS', "True"))


def test_nfs_wl_deployment():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    pvc_name = namespace["pvc"].name
    wl_name = sub_path = random_test_name("deployment")
    content = "from-test-wl-deployment"
    file_path = MOUNT_PATH + "/wl-deployment.txt"

    # deploy the first workload
    wl = create_wl_with_nfs(p_client, ns.id, pvc_name, wl_name,
                            mount_path=MOUNT_PATH, sub_path=sub_path)
    validate_workload(p_client, wl, "deployment", ns.name)

    # check if it can write data
    pods = p_client.list_pod(workloadId=wl.id).data
    assert len(pods) == 1
    pod = pods[0]
    write_content_to_file(pod, content, file_path)
    validate_file_content(pod, content, file_path)
    # delete the workload
    p_client.delete(wl)

    # deploy second workload for testing
    wl2 = create_wl_with_nfs(p_client, ns.id, pvc_name, "deployment-wl2",
                             mount_path=MOUNT_PATH, sub_path=sub_path)
    validate_workload(p_client, wl2, "deployment", ns.name)
    # check if it can read existing data
    pods.clear()
    pods = p_client.list_pod(workloadId=wl2.id).data
    assert len(pods) == 1
    pod = pods[0]
    validate_file_content(pod, content, file_path)


def test_nfs_wl_scale_up():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    pvc_name = namespace["pvc"].name
    wl_name = sub_path = random_test_name("scale-up")
    content = "from-nfs-wl-scale-up"
    file_path = MOUNT_PATH + "/wl-scale-up.txt"
    # deploy the workload
    wl = create_wl_with_nfs(p_client, ns.id, pvc_name, wl_name,
                            mount_path=MOUNT_PATH, sub_path=sub_path)
    validate_workload(p_client, wl, "deployment", ns.name)

    # write some data
    pods = p_client.list_pod(workloadId=wl.id).data
    assert len(pods) == 1
    pod = pods[0]
    write_content_to_file(pod, content, file_path)
    validate_file_content(pod, content, file_path)

    # scale up the workload
    p_client.update(wl, scale=2)
    wl = wait_for_wl_to_active(p_client, wl)
    pods.clear()
    pods = wait_for_pods_in_workload(p_client, wl, 2)
    assert len(pods) == 2
    for pod in pods:
        validate_file_content(pod, content, file_path)


def test_nfs_wl_upgrade():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    pvc_name = namespace["pvc"].name
    wl_name = sub_path = random_test_name("upgrade")
    content = "from-nfs-wl-upgrade"
    file_path = MOUNT_PATH + "/wl-upgrade.txt"
    # deploy the workload
    wl = create_wl_with_nfs(p_client, ns.id, pvc_name, wl_name,
                            mount_path=MOUNT_PATH, sub_path=sub_path)
    validate_workload(p_client, wl, "deployment", ns.name)

    pods = p_client.list_pod(workloadId=wl.id).data
    assert len(pods) == 1
    pod = pods[0]
    # write some data
    write_content_to_file(pod, content, file_path)
    validate_file_content(pod, content, file_path)

    # upgrade the workload
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "volumeMounts": [{"readOnly": "False",
                              "type": "volumeMount",
                              "mountPath": "/var/nfs",
                              "subPath": sub_path,
                              "name": "vol1"
                              }],
            "environment": {"REASON": "upgrade"}
            }]
    p_client.update(wl, containers=con)
    wl = wait_for_wl_to_active(p_client, wl)
    # check if it can read existing data
    pods.clear()
    pods = wait_for_pods_in_workload(p_client, wl, 1)
    assert len(pods) == 1
    pod = pods[0]
    validate_file_content(pod, content, file_path)

    # check if it can write some data
    content = content + "+after-upgrade"
    write_content_to_file(pod, content, file_path)
    validate_file_content(pod, content, file_path)


def test_nfs_wl_daemonSet():
    p_client = namespace["p_client"]
    cluster = namespace["cluster"]
    ns = namespace["ns"]
    pvc_name = namespace["pvc"].name
    wl_name = sub_path = random_test_name("daemon-set")
    content = "from-nfs-wl-daemon-set"
    file_path = MOUNT_PATH + "/" + "/wl-daemon-set.txt"

    # deploy the workload
    wl = create_wl_with_nfs(p_client, ns.id, pvc_name, wl_name,
                            MOUNT_PATH, sub_path, is_daemonSet=True)
    schedulable_node_count = len(get_schedulable_nodes(cluster))
    validate_workload(p_client, wl, "daemonSet",
                      ns.name, schedulable_node_count)

    # for each pod, write some data to the file,
    # then check if changes can be seen in all pods
    pods = p_client.list_pod(workloadId=wl.id).data
    for pod in pods:
        content = content + "+" + pod.name
        write_content_to_file(pod, content, file_path)
        for item in pods:
            validate_file_content(item, content, file_path)


@pytest.fixture(scope="module", autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "project-test-nfs")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    nfs_node = provision_nfs_server()
    nfs_ip = nfs_node.get_public_ip()
    print("the IP of the NFS: ", nfs_ip)

    # add  persistent volume to the cluster
    cluster_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    pv_name = random_test_name("pv")
    pv_config = {"type": "persistentVolume",
                 "accessModes": ["ReadWriteOnce"],
                 "name": pv_name,
                 "nfs": {"readOnly": "false",
                         "type": "nfsvolumesource",
                         "path": NFS_SERVER_MOUNT_PATH,
                         "server": nfs_ip
                         },
                 "capacity": {"storage": "10Gi"}
                 }
    pv_object = cluster_client.create_persistent_volume(pv_config)
    pv_object = wait_for_pv_to_be_available(cluster_client, pv_object)

    # add persistent volume claim to the project
    pvc_name = random_test_name("pvc")
    pvc_config = {"accessModes": ["ReadWriteOnce"],
                  "name": pvc_name,
                  "volumeId": pv_object.id,
                  "namespaceId": ns.id,
                  "storageClassId": "",
                  "resources": {"requests": {"storage": "10Gi"}}
                  }
    pvc_object = p_client.create_persistent_volume_claim(pvc_config)
    pvc_object = wait_for_pvc_to_be_bound(p_client, pvc_object)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["pv"] = pv_object
    namespace["pvc"] = pvc_object

    def fin():
        cluster_client = get_cluster_client_for_token(namespace["cluster"],
                                                      USER_TOKEN)
        cluster_client.delete(namespace["project"])
        cluster_client.delete(namespace["pv"])
        if DELETE_NFS is True:
            AmazonWebServices().delete_node(nfs_node)
    request.addfinalizer(fin)
