from ..common import *  # NOQA
import pprint
import json
import yaml
import rancher

TEST_IMAGE_V1 = os.environ.get('RANCHER_TEST_IMAGE_V1', "ranchertest/mytestcontainer")


def get_admin_client_v1():
    url = CATTLE_TEST_URL + "/v1"
    # in fact, we get the cluster client for the local cluster
    return rancher.Client(url=url, token=ADMIN_TOKEN, verify=False)


def get_cluster_client_for_token_v1(cluster_id=None, token=None):
    if cluster_id is None:
        cluster = get_cluster_by_name(get_admin_client_v1(), CLUSTER_NAME)
        cluster_id = cluster["id"]
    if token is None:
        token = USER_TOKEN

    url = CATTLE_TEST_URL + "/k8s/clusters/" + cluster_id + "/v1/schemas"
    return rancher.Client(url=url, token=token, verify=False)


def get_cluster_by_name(client, cluster_name):
    res = client.list_management_cattle_io_cluster()
    assert "data" in res.keys(), "failed to find any cluster in the setup"
    for cluster in res["data"]:
        if cluster["spec"]["displayName"] == cluster_name:
            return cluster
    assert False, "failed to find the cluster {}".format(cluster_name)


def display(res):
    if res is None:
        print("None object is returned")
        return
    if isinstance(res, dict) and "data" in res.keys():
        print("count of data {}".format(len(res.data)))
        for item in res.get("data"):
            print("-------")
            pprint.pprint(item)
        return
    else:
        print("This is a instance of {}".format(type(res)))
        pprint.pprint(res)


def read_json_from_resource_dir(filename):
    dir_path = os.path.dirname(os.path.realpath(__file__))
    try:
        with open('{}/resource/{}'.format(dir_path, filename)) as f:
            data = json.load(f)
        return data
    except FileNotFoundError as e:
        assert False, e


def read_yaml_from_resource_dir(filename):
    dir_path = os.path.dirname(os.path.realpath(__file__))
    try:
        with open('{}/resource/{}'.format(dir_path, filename)) as f:
            data = yaml.safe_load(f)
        return data
    except FileNotFoundError as e:
        assert False, e


def create_project(cluster, project_name):
    admin_client = get_admin_client_v1()
    project = read_yaml_from_resource_dir("project.yaml")
    project["metadata"]["name"] = project_name
    project["metadata"]["namespace"] = cluster["id"]
    project["spec"]["clusterName"] = cluster["id"]
    project["spec"]["displayName"] = project_name
    res = admin_client.create_management_cattle_io_project(project)
    return res
