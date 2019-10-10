import base64

import pytest

from .common import *  # NOQA

CLUSTER_NAME = os.environ.get("CLUSTER_NAME", "")

namespace = {"p_client": None, "ns": None, "cluster": None, "project": None}


def test_secret_create_all_ns():

    p_client = namespace["p_client"]
    ns = namespace["ns"]

    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}
    cluster = namespace["cluster"]
    project = namespace["project"]
    c_client = namespace["c_client"]

    new_ns = create_ns(c_client, cluster, project)
    namespacelist = [ns, new_ns]
    secret = create_secret(keyvaluepair)

    # Create workloads with secret in existing namespaces
    for ns in namespacelist:

        create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                           ns,
                                                           keyvaluepair)
        create_and_validate_workload_with_secret_as_env_variable(p_client,
                                                                 secret,
                                                                 ns,
                                                                 keyvaluepair)

    # Create a new namespace and workload in the new namespace using the secret

    new_ns1 = create_ns(c_client, cluster, project)
    create_and_validate_workload_with_secret_as_volume(p_client,
                                                       secret,
                                                       new_ns1,
                                                       keyvaluepair)
    create_and_validate_workload_with_secret_as_env_variable(p_client,
                                                             secret,
                                                             new_ns1,
                                                             keyvaluepair)


def test_secret_create_single_ns():

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}

    secret = create_secret(keyvaluepair, singlenamespace=True)

    # Create workloads with secret in existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret, ns,
                                                       keyvaluepair)
    create_and_validate_workload_with_secret_as_env_variable(p_client, secret,
                                                             ns, keyvaluepair)


def test_delete_secret_all_ns():

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valuealldelete")
    keyvaluepair = {"testalldelete": value.decode('utf-8')}
    secret = create_secret(keyvaluepair)
    delete_secret(p_client, secret, ns, keyvaluepair)


def test_delete_secret_single_ns():

    p_client = namespace["p_client"]
    ns = namespace["ns"]
    # Value is base64 encoded
    value = base64.b64encode(b"valuealldelete")
    keyvaluepair = {"testalldelete": value.decode('utf-8')}

    secret = create_secret(keyvaluepair, singlenamespace=True)
    delete_secret(p_client, secret, ns, keyvaluepair)


def test_edit_secret_all_ns():

    p_client = namespace["p_client"]
    name = random_test_name("default")
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}
    cluster = namespace["cluster"]
    project = namespace["project"]
    c_client = namespace["c_client"]

    # Create a namespace
    new_ns = create_ns(c_client, cluster, project)
    secret = create_secret(keyvaluepair)

    # Value is base64 encoded
    value1 = base64.b64encode(b"valueall")
    value2 = base64.b64encode(b"valueallnew")
    updated_dict = {"testall": value1.decode(
        'utf-8'), "testallnew": value2.decode('utf-8')}
    updated_secret = p_client.update(secret, name=name, namespaceId='NULL',
                                     data=updated_dict)

    assert updated_secret['baseType'] == "secret"
    updatedsecretdata = updated_secret['data']

    print("UPDATED SECRET DATA")
    print(updatedsecretdata)

    assert updatedsecretdata.data_dict() == updated_dict

    # Create workloads using updated secret in the existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       new_ns,
                                                       updatedsecretdata)

    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, new_ns, updatedsecretdata)

    # Create a new namespace and workloads in the new namespace using secret
    new_ns1 = create_ns(c_client, cluster, project)

    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       new_ns1,
                                                       updatedsecretdata)
    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, new_ns1, updatedsecretdata)


def test_edit_secret_single_ns():
    p_client = namespace["p_client"]
    ns = namespace["ns"]
    name = random_test_name("default")
    # Value is base64 encoded
    value = base64.b64encode(b"valueall")
    keyvaluepair = {"testall": value.decode('utf-8')}

    secret = create_secret(keyvaluepair, singlenamespace=True)

    value1 = base64.b64encode(b"valueall")
    value2 = base64.b64encode(b"valueallnew")
    updated_dict = {"testall": value1.decode(
        'utf-8'), "testallnew": value2.decode('utf-8')}
    updated_secret = p_client.update(secret, name=name,
                                     namespaceId=ns['name'],
                                     data=updated_dict)
    assert updated_secret['baseType'] == "namespacedSecret"
    updatedsecretdata = updated_secret['data']

    print("UPDATED SECRET DATA")
    print(updatedsecretdata)

    assert updatedsecretdata.data_dict() == updated_dict

    # Create a workload with the updated secret in the existing namespace
    create_and_validate_workload_with_secret_as_volume(p_client, secret,
                                                       ns,
                                                       updatedsecretdata)
    create_and_validate_workload_with_secret_as_env_variable(
        p_client, secret, ns, updatedsecretdata)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    p, ns = create_project_and_ns(USER_TOKEN, cluster, "testsecret")
    p_client = get_project_client_for_token(p, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    namespace["p_client"] = p_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = p
    namespace["c_client"] = c_client

    def fin():
        client = get_user_client()
        client.delete(namespace["project"])
    request.addfinalizer(fin)


def validate_workload_with_secret(p_client, workload,
                                  type, ns_name, keyvaluepair,
                                  workloadwithsecretasVolume=False,
                                  workloadwithsecretasenvvar=False,
                                  podcount=1):

    validate_workload(p_client, workload, type, ns_name, pod_count=podcount)

    pod_list = p_client.list_pod(workloadId=workload.id).data
    mountpath = "/test"
    for i in range(0, len(keyvaluepair)):
        key = list(keyvaluepair.keys())[i]
        if workloadwithsecretasVolume:
            key_file_in_pod = mountpath + "/" + key
            print(key_file_in_pod)
            command = "cat " + key_file_in_pod + ''
            print(" Command to display secret value from container is: ")
            print(command)
            result = kubectl_pod_exec(pod_list[0], command)
            assert result == base64.b64decode(list(keyvaluepair.values())[i])
        elif workloadwithsecretasenvvar:
            command = 'env'
            result = kubectl_pod_exec(pod_list[0], command)
            if base64.b64decode(list(keyvaluepair.values())[i]) in result:
                assert True


def delete_secret(client, secret, ns, keyvaluepair):

    key = list(keyvaluepair.keys())[0]

    print("Delete Secret")
    client.delete(secret)

    # Sleep to allow for the secret to be deleted
    time.sleep(5)
    print("Secret list after deleting secret")
    secretdict = client.list_secret()
    print(secretdict)
    print(secretdict.get('data'))
    if len(secretdict.get('data')) > 0:
        testdata = secretdict.get('data')
        print("TESTDATA")
        print(testdata[0]['data'])
        if key in testdata[0]['data']:
            assert False
        else:
            assert True
    if len(secretdict.get('data')) == 0:
        assert True

    # Verify secret is deleted by "kubectl get secret" command
    command = " get secret " + secret['name'] + " --namespace=" + ns.name
    print("Command to obtain the secret")
    print(command)
    result = execute_kubectl_cmd(command, json_out=False, stderr=True)
    print(result)

    print("Verify that the secret does not exist "
          "and the error code returned is non zero ")
    if result != 0:
        assert True


def create_and_validate_workload_with_secret_as_volume(p_client, secret, ns,
                                                       keyvaluepair,
                                                       name=None):
    if name is None:
        name = random_test_name("test")
    # Create Workload with secret as volume
    mountpath = "/test"
    volumeMounts = [{"readOnly": False, "type": "volumeMount",
                     "mountPath": mountpath, "name": "vol1"}]
    con = [{"name": "test1",
            "image": TEST_IMAGE,
            "volumeMounts": volumeMounts}]

    secretName = secret['name']

    volumes = [{"type": "volume", "name": "vol1",
                "secret": {"type": "secretVolumeSource", "defaultMode": 256,
                           "secretName": secretName,
                           "optional": False, "items": "NULL"}}]

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id, volumes=volumes)
    validate_workload_with_secret(p_client, workload, "deployment",
                                  ns.name, keyvaluepair,
                                  workloadwithsecretasVolume=True)

    return workload


def create_and_validate_workload_with_secret_as_env_variable(p_client, secret,
                                                             ns, keyvaluepair,
                                                             name=None):
    if name is None:
        name = random_test_name("test")

    # Create Workload with secret as env variable
    secretName = secret['name']

    environmentdata = [{
        "source": "secret",
        "sourceKey": None,
        "sourceName": secretName
    }]
    con = [{"name": "test",
            "image": TEST_IMAGE,
            "environmentFrom": environmentdata}]

    workload = p_client.create_workload(name=name,
                                        containers=con,
                                        namespaceId=ns.id)
    validate_workload_with_secret(p_client, workload, "deployment",
                                  ns.name, keyvaluepair,
                                  workloadwithsecretasenvvar=True)
    return workload


def create_secret(keyvaluepair, singlenamespace=False,
                  p_client=None, ns=None, name=None):

    if p_client is None:
        p_client = namespace["p_client"]

    if name is None:
        name = random_test_name("default")
    if ns is None:
        ns = namespace["ns"]

    if not singlenamespace:
        secret = p_client.create_secret(name=name, data=keyvaluepair)
        assert secret['baseType'] == "secret"
    else:
        secret = p_client.create_namespaced_secret(name=name,
                                                   namespaceId=ns['name'],
                                                   data=keyvaluepair)
        assert secret['baseType'] == "namespacedSecret"

    print(secret)
    secretdata = secret['data']
    print("SECRET DATA")
    print(secretdata)
    assert secretdata.data_dict() == keyvaluepair

    return secret
