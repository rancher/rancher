from .common import random_str
from .conftest import wait_until_available, wait_until, wait_for
from rancher import ApiError
import time
import pytest
import kubernetes


def test_role_template_creation(admin_mc, remove_resource):
    rt_name = random_str()
    rt = admin_mc.client.create_role_template(name=rt_name)
    remove_resource(rt)
    assert rt is not None
    assert rt.name == rt_name


def test_administrative_role_template_creation(admin_mc, remove_resource):
    client = admin_mc.client
    crt_name = random_str()
    crt = client.create_role_template(name=crt_name,
                                      context="cluster",
                                      administrative=True)
    remove_resource(crt)
    assert crt is not None
    assert crt.name == crt_name

    prt_name = random_str()
    try:
        client.create_role_template(name=prt_name,
                                    context="project",
                                    administrative=True)
    except ApiError as e:
        assert e.error.status == 500
        assert e.error.message == "Only cluster roles can be administrative"


def test_edit_builtin_role_template(admin_mc, remove_resource):
    client = admin_mc.client
    # edit non builtin role, any field is updatable
    org_rt_name = random_str()
    rt = client.create_role_template(name=org_rt_name)
    remove_resource(rt)
    wait_for_role_template_creation(admin_mc, org_rt_name)
    new_rt_name = random_str()
    rt = client.update(rt, name=new_rt_name)
    assert rt.name == new_rt_name

    # edit builtin role, only locked,cluster/projectcreatordefault
    # are updatable
    new_rt_name = "Cluster Member-Updated"
    cm_rt = client.by_id_role_template("cluster-member")
    rt = client.update(cm_rt, name=new_rt_name)
    assert rt.name == "Cluster Member"


def test_context_prtb(admin_mc, admin_pc, remove_resource,
                      user_mc):
    """Asserts that a projectroletemplatebinding cannot reference a cluster
    roletemplate
    """
    admin_client = admin_mc.client
    project = admin_pc.project

    with pytest.raises(ApiError) as e:
        prtb = admin_client.create_project_role_template_binding(
            name="prtb-" + random_str(),
            userId=user_mc.user.id,
            projectId=project.id,
            roleTemplateId="cluster-owner"
        )
        remove_resource(prtb)

    assert e.value.error.status == 422
    assert "Cannot reference context [cluster] from [project] context" in \
           e.value.error.message


def test_context_crtb(admin_mc, admin_cc, remove_resource,
                      user_mc):
    """Asserts that a clusterroletemplatebinding cannot reference a project
    roletemplate
    """
    admin_client = admin_mc.client

    with pytest.raises(ApiError) as e:
        crtb = admin_client.create_cluster_role_template_binding(
            userId=user_mc.user.id,
            roleTemplateId="project-owner",
            clusterId=admin_cc.cluster.id,
        )
        remove_resource(crtb)

    assert e.value.error.status == 422
    assert "Cannot reference context [project] from [cluster] context" in \
        e.value.error.message


def test_cloned_role_permissions(admin_mc, remove_resource, user_factory,
                                 admin_pc):
    client = admin_mc.client
    rt_name = random_str()
    rt = client.create_role_template(name=rt_name, context="project",
                                     roleTemplateIds=["project-owner"])
    remove_resource(rt)
    wait_for_role_template_creation(admin_mc, rt_name)

    # user with cloned project owner role should be able to enable monitoring
    cloned_user = user_factory()
    remove_resource(cloned_user)

    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=cloned_user.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId=rt.id
    )
    remove_resource(prtb)
    wait_until_available(cloned_user.client, admin_pc.project)

    project = cloned_user.client.by_id_project(admin_pc.project.id)
    assert project.actions.enableMonitoring


def test_update_role_template_permissions(admin_mc, remove_resource,
                                          user_factory, admin_cc):
    client = admin_mc.client
    cc_rt_name = random_str()
    view_cc_rule = [{'apiGroups': ['management.cattle.io'],
                     'resources': ['clustercatalogs'],
                     'type': '/v3/schemas/policyRule',
                     'verbs': ['get', 'list', 'watch']},
                    {'apiGroups': ['management.cattle.io'],
                     'resources': ['clusterevents'],
                     'type': '/v3/schemas/policyRule',
                     'verbs': ['get', 'list', 'watch']}]
    rt = client.create_role_template(name=cc_rt_name, context="cluster",
                                     rules=view_cc_rule)
    # remove_resource(rt)
    role_template_id = rt['id']
    wait_for_role_template_creation(admin_mc, cc_rt_name)

    user_view_cc = user_factory()
    user_client = user_view_cc.client
    crtb = client.create_cluster_role_template_binding(
        userId=user_view_cc.user.id,
        roleTemplateId=role_template_id,
        clusterId=admin_cc.cluster.id,
    )
    remove_resource(crtb)
    wait_until_available(user_client, admin_cc.cluster)

    # add clustercatalog as admin
    url = "https://github.com/rancher/integration-test-charts.git"
    name = random_str()
    cluster_catalog = \
        client.create_cluster_catalog(name=name,
                                      branch="master",
                                      url=url,
                                      clusterId="local",
                                      )
    remove_resource(cluster_catalog)
    wait_until_available(client, cluster_catalog)

    # list clustercatalog as the cluster-member
    cc = user_client.list_cluster_catalog(name=name)
    assert len(cc) == 1

    # update role to remove view clustercatalogs permission
    view_cc_role_template = client.by_id_role_template(role_template_id)
    new_rules = [{'apiGroups': ['management.cattle.io'],
                  'resources': ['clusterevents'],
                  'type': '/v3/schemas/policyRule',
                  'verbs': ['get', 'list', 'watch']}]
    client.update(view_cc_role_template, rules=new_rules)
    wait_until(lambda: client.reload(view_cc_role_template)['rules'] is None)

    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)

    def check_role_rules(rbac, namespace, role_name, rules):
        role = rbac.read_namespaced_role(role_name, namespace)
        if len(role.rules) == len(rules) and \
           role.rules[0].resources == ["clusterevents"]:
            return True

    wait_for(lambda: check_role_rules(rbac, 'local', role_template_id,
                                      new_rules),
             timeout=60, fail_handler=lambda:
             'failed to check updated role')
    # user should not be able to list cluster catalog now
    cc = user_client.list_cluster_catalog(name=name)
    assert len(cc) == 0


def test_role_template_update_inherited_role(admin_mc, remove_resource,
                                             user_factory, admin_pc):
    client = admin_mc.client
    name = random_str()
    # clone project-member role
    pm = client.by_id_role_template("project-member")
    cloned_pm = client.create_role_template(name=name, context="project",
                                            rules=pm.rules,
                                            roleTemplateIds=["edit"])
    remove_resource(cloned_pm)
    role_template_id = cloned_pm['id']
    wait_for_role_template_creation(admin_mc, name)

    # create a namespace in this project
    ns_name = random_str()
    ns = admin_pc.cluster.client.create_namespace(name=ns_name,
                                                  projectId=admin_pc.
                                                  project.id)
    remove_resource(ns)

    # add user to a project with this role
    user_cloned_pm = user_factory()
    prtb = client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_cloned_pm.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId=role_template_id
    )
    remove_resource(prtb)
    wait_until_available(user_cloned_pm.client, admin_pc.project)

    # As the user, assert that the two expected role bindings exist in the
    # namespace for the user. There should be one for the rancher role
    # 'cloned_pm' and one for the k8s built-in role 'edit'
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)

    def _refresh_user_template():
        rbs = rbac.list_namespaced_role_binding(ns_name)
        rb_dict = {}
        for rb in rbs.items:
            if rb.subjects[0].name == user_cloned_pm.user.id:
                rb_dict[rb.role_ref.name] = rb
        return role_template_id in rb_dict and 'edit' in rb_dict

    wait_for(_refresh_user_template,
             fail_handler=lambda: 'role bindings do not exist')

    # now edit the roleTemplate to remove "edit" from inherited roles,
    # and add "view" to inherited roles
    client.update(cloned_pm, roleTemplateIds=["view"])
    wait_until(lambda: client.reload(cloned_pm)['roleTemplateIds'] is ["view"])

    def check_rb(rbac):
        rbs = rbac.list_namespaced_role_binding(ns_name)
        for rb in rbs.items:
            if rb.subjects[0].name == user_cloned_pm.user.id \
                    and rb.role_ref.name == "view":
                return True

    wait_for(lambda: check_rb(rbac), timeout=60,
             fail_handler=lambda: 'failed to check updated rolebinding')

    # Now there should be one rolebinding for the rancher role
    # 'cloned_pm' and one for the k8s built-in role 'view'
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    rbs = rbac.list_namespaced_role_binding(ns_name)
    rb_dict = {}
    for rb in rbs.items:
        if rb.subjects[0].name == user_cloned_pm.user.id:
            rb_dict[rb.role_ref.name] = rb
    assert role_template_id in rb_dict
    assert 'view' in rb_dict
    assert 'edit' not in rb_dict


def test_kubernetes_admin_permissions(admin_mc, remove_resource, user_factory,
                                      admin_pc):
    client = admin_mc.client
    name = random_str()
    # clone Kubernetes-admin role
    cloned_admin = client.create_role_template(name=name, context="project",
                                               roleTemplateIds=["admin"])
    remove_resource(cloned_admin)
    wait_for_role_template_creation(admin_mc, name)

    # add user with cloned kubernetes-admin role to a project
    cloned_user = user_factory()
    remove_resource(cloned_user)

    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=cloned_user.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId=cloned_admin.id
    )
    remove_resource(prtb)
    wait_until_available(cloned_user.client, admin_pc.project)

    # cloned kubernetes-admin role should not give user project-owner
    # privileges, for instance, user should not be able to create enable
    # monitoring

    project = cloned_user.client.by_id_project(admin_pc.project.id)
    assert 'enableMonitoring' not in project.actions


def test_role_template_changes_revoke_permissions(admin_mc, remove_resource,
                                                  user_factory, admin_pc):
    client = admin_mc.client
    name = random_str()
    # clone project-owner role
    po = client.by_id_role_template("project-owner")
    cloned_po = client.create_role_template(name=name, context="project",
                                            rules=po.rules,
                                            roleTemplateIds=["admin"])
    remove_resource(cloned_po)
    wait_for_role_template_creation(admin_mc, name)
    role_template_id = cloned_po['id']

    user = user_factory()
    # add a user with this cloned project-owner role to a project
    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId=role_template_id
    )
    remove_resource(prtb)
    wait_until_available(user.client, admin_pc.project)

    # this user should be able to add users to the project
    user1 = user_factory()
    prtb1 = user.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user1.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId="project-member"
    )
    remove_resource(prtb1)
    wait_until_available(user1.client, admin_pc.project)

    # now edit the cloned roletemplate to remove permission
    # to create projectroletemplatebindings
    rules = cloned_po['rules']
    for ind, rule in enumerate(rules):
        if 'projectroletemplatebindings' in rule['resources']:
            setattr(rule, 'verbs', ['get', 'list', 'watch'])

    client.update(cloned_po, rules=rules)

    def role_template_update_check():
        rt = client.by_id_role_template(role_template_id)
        for rule in rt['rules']:
            if 'projectroletemplatebindings' in rule['resources']:
                return rule['verbs'] == ['get', 'list', 'watch']

    def fail_handler():
        return "failed waiting for cloned roletemplate to be updated"

    wait_for(role_template_update_check, fail_handler=fail_handler(),
             timeout=120)

    # now as the same user again try adding another user to a project, this
    # should not work since the user with cloned_po role can no longer create
    # projectroletemplatebindings
    user2 = user_factory()
    with pytest.raises(ApiError) as e:
        user.client.create_project_role_template_binding(
            name="prtb-" + random_str(),
            userId=user2.user.id,
            projectId=admin_pc.project.id,
            roleTemplateId="project-member"
        )
    assert e.value.error.status == 403


def wait_for_role_template_creation(admin_mc, rt_name, timeout=60):
    start = time.time()
    interval = 0.5
    client = admin_mc.client
    found = False
    while not found:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for roletemplate creation')
        rt = client.list_role_template(name=rt_name)
        if len(rt) > 0:
            found = True
        time.sleep(interval)
        interval *= 2
