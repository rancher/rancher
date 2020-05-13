import pytest
from rancher import ApiError

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
