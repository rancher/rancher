import pytest
from rancher import ApiError
from kubernetes.client import CoreV1Api
from .conftest import wait_for

systemProjectLabel = "authz.management.cattle.io/system-project"
defaultProjectLabel = "authz.management.cattle.io/default-project"
initial_system_namespaces = set(["kube-node-lease",
                                 "kube-system",
                                 "cattle-system",
                                 "kube-public",
                                 "cattle-global-data",
                                 "cattle-global-nt"])
loggingNamespace = "cattle-logging"


def test_system_project_created(admin_cc):
    projects = admin_cc.management.client.list_project(
        clusterId=admin_cc.cluster.id)
    initial_projects = {}
    initial_projects["Default"] = defaultProjectLabel
    initial_projects["System"] = systemProjectLabel
    required_projects = []

    for project in projects:
        name = project['name']
        if name in initial_projects:
            projectLabel = initial_projects[name]
            assert project['labels'].\
                data_dict()[projectLabel] == 'true'
            required_projects.append(name)

    assert len(required_projects) == len(initial_projects)


def test_system_namespaces_assigned(admin_cc):
    projects = admin_cc.management.client.list_project(
        clusterId=admin_cc.cluster.id)
    systemProject = None
    for project in projects:
        if project['name'] == "System":
            systemProject = project
            break
    assert systemProject is not None

    system_namespaces = admin_cc.client.list_namespace(
        projectId=systemProject.id)
    system_namespaces_names = set(
        [ns['name'] for ns in system_namespaces])

    # If clusterLogging tests run before this, cattle-logging
    # will be present in current system_namespaces, removing it
    if loggingNamespace in system_namespaces_names:
        system_namespaces_names.remove(loggingNamespace)

    assert system_namespaces_names == initial_system_namespaces


def test_system_project_cant_be_deleted(admin_mc, admin_cc):
    """The system project is not allowed to be deleted, test to ensure that is
    true
    """
    projects = admin_cc.management.client.list_project(
        clusterId=admin_cc.cluster.id)
    system_project = None
    for project in projects:
        if project['name'] == "System":
            system_project = project
            break
    assert system_project is not None

    # Attempting to delete the template should raise an ApiError
    with pytest.raises(ApiError) as e:
        admin_mc.client.delete(system_project)
    assert e.value.error.status == 405
    assert e.value.error.message == 'System Project cannot be deleted'


def test_system_namespaces_default_svc_account(admin_mc):
    system_namespaces_setting = admin_mc.client.by_id_setting(
                                "system-namespaces")
    system_namespaces = system_namespaces_setting["value"].split(",")
    k8sclient = CoreV1Api(admin_mc.k8s_client)
    def_saccnts = k8sclient.list_service_account_for_all_namespaces(
        field_selector='metadata.name=default')
    for sa in def_saccnts.items:
        ns = sa.metadata.namespace

        def _check_system_sa_flag():
            if ns in system_namespaces and ns != "kube-system":
                if sa.automount_service_account_token is False:
                    return True
                else:
                    return False
            else:
                return True

        def _sa_update_fail():
            name = sa.metadata.name
            flag = sa.automount_service_account_token
            return 'Service account {} in namespace {} does not have correct \
            automount_service_account_token flag: {}'.format(name, ns, flag)

        wait_for(_check_system_sa_flag, fail_handler=_sa_update_fail)
