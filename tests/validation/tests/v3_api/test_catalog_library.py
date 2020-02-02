"""
This file has tests to deploy apps in a project created in a cluster.
Test requirements:
Env variables - Cattle_url, Admin Token, User Token, Cluster Name
Test on at least 3 worker nodes
App versions are given in 'cataloglib_appversion.json' file
"""


import json
from .common import os
from .common import pytest
from .common import create_ns
from .common import create_catalog_external_id
from .common import get_defaut_question_answers
from .common import validate_catalog_app
from .common import validate_app_deletion
from .common import get_user_client_and_cluster
from .common import create_kubeconfig
from .common import get_cluster_client_for_token
from .common import create_project
from .common import random_test_name
from .common import get_project_client_for_token
from .common import USER_TOKEN


cluster_info = {"cluster": None, "cluster_client": None,
                "project": None, "project_client": None,
                "user_client": None}
catalog_filename = os.path.join(os.path.dirname(os.path.realpath(__file__)),
                                "cataloglib_appversion.json")
with open(catalog_filename, "r") as app_v:
    app_data = json.load(app_v)


@pytest.mark.parametrize('app_name, app_version', app_data.items())
def test_catalog_app_deploy(app_name, app_version):
    """
    Runs for app from 'cataloglib_appversion.json',
    creates relevant namespace and deploy them.
    Validates status of the app, version and answer.
    """
    user_client = cluster_info["user_client"]
    project_client = cluster_info["project_client"]
    cluster_client = cluster_info["cluster_client"]
    cluster = cluster_info["cluster"]
    project = cluster_info["project"]

    ns = create_ns(cluster_client, cluster, project, app_name)
    app_ext_id = create_catalog_external_id('library',
                                            app_name, app_version)
    answer = get_defaut_question_answers(user_client, app_ext_id)
    app = project_client.create_app(
                                name=random_test_name(),
                                externalId=app_ext_id,
                                targetNamespace=ns.name,
                                projectId=ns.projectId,
                                answers=answer)
    validate_catalog_app(project_client, app, app_ext_id, answer)
    project_client.delete(app)
    validate_app_deletion(project_client, app.id)


@pytest.fixture(scope='module', autouse="True")
def create_project_client():
    """
    Creates project in a cluster and collects details of
    user, project and cluster
    """
    user_client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    cluster_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    project = create_project(user_client, cluster,
                             random_test_name("App-deployment"))
    project_client = get_project_client_for_token(project, USER_TOKEN)

    cluster_info["cluster"] = cluster
    cluster_info["cluster_client"] = cluster_client
    cluster_info["project"] = project
    cluster_info["project_client"] = project_client
    cluster_info["user_client"] = user_client
