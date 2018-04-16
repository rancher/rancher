systemProjectLabel = "authz.management.cattle.io/system-project"
defaultProjectLabel = "authz.management.cattle.io/default-project"
initial_system_namespaces = set(["kube-system",
                                "cattle-system",
                                 "kube-public"])


def test_system_project_created(admin_cc):
    projects = admin_cc.management.client.list_project(
               clusterId=admin_cc.cluster.id)
    initial_projects = {}
    initial_projects["Default"] = defaultProjectLabel
    initial_projects["System"] = systemProjectLabel
    required_projects = []

    for project in projects:
        name = project.data_dict()['name']
        if name in initial_projects:
            projectLabel = initial_projects[name]
            assert project.data_dict()['labels'].\
                data_dict()[projectLabel] == 'true'
            required_projects.append(name)

    assert len(required_projects) == len(initial_projects)


def test_system_namespaces_assigned(admin_cc, admin_pc):
    projects = admin_cc.management.client.list_project(
               clusterId=admin_cc.cluster.id)
    systemProject = None
    for project in projects:
        if project.data_dict()['name'] == "System":
            systemProject = project
            break
    assert systemProject is not None

    system_namespaces = admin_cc.client.list_namespace(
                        projectId=systemProject.id)
    system_namespaces_names = set(
        [ns.data_dict()['name'] for ns in system_namespaces])
    assert system_namespaces_names == initial_system_namespaces
