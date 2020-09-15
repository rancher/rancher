from .common import random_str, check_subject_in_rb
from rancher import ApiError
from .conftest import wait_until, wait_for, DEFAULT_TIMEOUT
import pytest
import time
import kubernetes

rb_resource = 'rolebinding'


def test_create_cluster_template_with_revision(admin_mc, remove_resource):

    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_resource(cluster_template)
    templateId = cluster_template.id
    _ = \
        create_cluster_template_revision(admin_mc.client, templateId)
    _ = \
        create_cluster_template_revision(admin_mc.client, templateId)
    client = admin_mc.client
    template_reloaded = client.by_id_cluster_template(cluster_template.id)
    assert template_reloaded.links.revisions is not None


def test_create_template_revision_k8s_translation(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               [], admin_mc)
    remove_resource(cluster_template)
    tId = cluster_template.id
    client = admin_mc.client

    cconfig = {
        "rancherKubernetesEngineConfig": {
            "kubernetesVersion": "1.15"
        }
    }
    with pytest.raises(ApiError) as e:
        client.create_cluster_template_revision(clusterConfig=cconfig,
                                                clusterTemplateId=tId,
                                                enabled="true")
    assert e.value.error.status == 422

    # template k8s question needed if using generic version
    cconfig = {
        "rancherKubernetesEngineConfig": {
            "kubernetesVersion": "1.15.x"
        }
    }
    questions = [{
                  "variable": "dockerRootDir",
                  "required": "false",
                  "type": "string",
                  "default": "/var/lib/docker"
                 }]
    with pytest.raises(ApiError) as e:
        client.create_cluster_template_revision(name=random_str(),
                                                clusterConfig=cconfig,
                                                clusterTemplateId=tId,
                                                questions=questions,
                                                enabled="true")
    assert e.value.error.status == 422


def test_default_pod_sec(admin_mc, list_remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)

    tId = cluster_template.id
    client = admin_mc.client
    cconfig = {
        "rancherKubernetesEngineConfig": {
            "services": {
                "type": "rkeConfigServices",
                "kubeApi": {
                    "alwaysPullImages": "false",
                    "podSecurityPolicy": "false",
                    "serviceNodePortRange": "30000-32767",
                    "type": "kubeAPIService"
                }
            }
        },
        "defaultPodSecurityPolicyTemplateId": "restricted",
    }
    rev = client.create_cluster_template_revision(name=random_str(),
                                                  clusterConfig=cconfig,
                                                  clusterTemplateId=tId,
                                                  enabled="true")

    time.sleep(2)
    cluster = wait_for_cluster_create(client, name=random_str(),
                                      clusterTemplateRevisionId=rev.id)
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    assert cluster.defaultPodSecurityPolicyTemplateId == "restricted"
    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_check_default_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               [], admin_mc)
    remove_resource(cluster_template)
    templateId = cluster_template.id
    first_revision = \
        create_cluster_template_revision(admin_mc.client, templateId)
    client = admin_mc.client
    wait_for_default_revision(client, templateId, first_revision.id)
    # delete the cluster template revision, it should error out
    with pytest.raises(ApiError) as e:
        client.delete(first_revision)
    assert e.value.error.status == 403


def test_create_cluster_with_template(admin_mc, list_remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)
    templateId = cluster_template.id

    template_revision = \
        create_cluster_template_revision(admin_mc.client, templateId)

    # create a cluster with this template
    answers = {
                "values": {
                    "dockerRootDir": "/var/lib/docker123",
                    "rancherKubernetesEngineConfig.ignoreDockerVersion":
                    "false"
                }
              }

    revId = template_revision.id
    client = admin_mc.client

    cluster = wait_for_cluster_create(client, name=random_str(),
                                      clusterTemplateRevisionId=revId,
                                      description="template from cluster",
                                      answers=answers)
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    assert cluster.questions is not None
    k8s_version = cluster.rancherKubernetesEngineConfig.kubernetesVersion
    assert k8s_version != "v1.15.x"

    # edit cluster should not fail
    client.update(cluster, name=random_str(), clusterTemplateRevisionId=revId)

    # edit cluster to remove template must fail
    with pytest.raises(ApiError) as e:
        client.update(cluster, name=random_str(), clusterTemplateId=None,
                      clusterTemplateRevisionId=None)
    assert e.value.error.status == 422

    # delete the cluster template, it should error out
    with pytest.raises(ApiError) as e:
        client.delete(cluster_template)
    assert e.value.error.status == 422

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_create_cluster_validations(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               [], admin_mc)
    remove_resource(cluster_template)
    templateId = cluster_template.id
    template_revision = \
        create_cluster_template_revision(admin_mc.client, templateId)
    # create a cluster with this template
    revId = template_revision.id
    client = admin_mc.client
    rConfig = getRKEConfig()
    try:
        wait_for_cluster_create(client, name=random_str(),
                                clusterTemplateRevisionId=revId,
                                description="template from cluster",
                                rancherKubernetesEngineConfig=rConfig)

    except ApiError as e:
        assert e.error.status == 500


@pytest.mark.nonparallel
def test_create_cluster_template_with_members(admin_mc, remove_resource,
                                              user_factory):
    client = admin_mc.client
    user_member = user_factory()
    remove_resource(user_member)
    user_not_member = user_factory()
    remove_resource(user_not_member)
    members = [{"userPrincipalId": "local://" + user_member.user.id,
                "accessType": "read-only"}]
    cluster_template = create_cluster_template(admin_mc, members, admin_mc)
    remove_resource(cluster_template)
    time.sleep(30)
    # check who has access to the cluster template
    # admin and user_member should be able to list it
    id = cluster_template.id
    ct = client.by_id_cluster_template(id)
    assert ct is not None
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_member.user.id, rb_name),
             timeout=60,
             fail_handler=lambda: "failed to check rolebinding")
    um_client = user_member.client
    ct = um_client.by_id_cluster_template(id)
    assert ct is not None

    # user not added as member shouldn't be able to access
    unm_client = user_not_member.client
    try:
        unm_client.by_id_cluster_template(id)
    except ApiError as e:
        assert e.error.status == 403

    # add * as member to share with all
    new_members = [{"userPrincipalId": "local://" + user_member.user.id,
                    "accessType": "read-only"}, {"groupPrincipalId": "*",
                                                 "accessType": "read-only"}]
    client.update(ct, members=new_members)

    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         'system:authenticated', rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    time.sleep(30)
    ct = user_not_member.client.by_id_cluster_template(id)
    assert ct is not None


def test_creation_standard_user(admin_mc, remove_resource, user_factory):
    user_member = user_factory()
    remove_resource(user_member)
    um_client = user_member.client
    with pytest.raises(ApiError) as e:
        um_client.create_cluster_template(name="user template",
                                          description="user template")
    assert e.value.error.status == 403


@pytest.mark.nonparallel
def test_check_enforcement(admin_mc, remove_resource,
                           list_remove_resource, user_factory):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)
    templateId = cluster_template.id
    rev = \
        create_cluster_template_revision(admin_mc.client, templateId)

    client = admin_mc.client

    # turn on the enforcement
    client.update_by_id_setting(id='cluster-template-enforcement',
                                value="true")

    # a globaladmin can create a rke cluster without a template
    cluster = client.create_cluster(
            name=random_str(), rancherKubernetesEngineConfig={
                "accessKey": "asdfsd"})
    remove_list.insert(0, cluster)

    # a user cannot create an rke cluster without template
    user = user_factory()
    remove_resource(user)
    crtb_owner = client.create_cluster_role_template_binding(
          clusterId="local",
          roleTemplateId="cluster-owner",
          userId=user.user.id)
    remove_resource(crtb_owner)
    wait_until(rtb_cb(client, crtb_owner))

    user_client = user.client
    with pytest.raises(ApiError) as e:
        user_client.create_cluster(name=random_str(),
                                   rancherKubernetesEngineConfig={
                                        "accessKey": "asdfsd"})
    assert e.value.error.status == 422

    # a user can create a non-rke cluster without template
    cluster2 = user_client.create_cluster(
            name=random_str(), amazonElasticContainerServiceConfig={
                "accessKey": "asdfsd"})
    remove_list.insert(0, cluster2)

    # a user can create an rke cluster with a public template
    template_reloaded = client.by_id_cluster_template(templateId)
    new_members = [{"groupPrincipalId": "*", "accessType": "read-only"}]
    client.update(template_reloaded, members=new_members)

    cluster3 = wait_for_cluster_create(user_client, name=random_str(),
                                       clusterTemplateRevisionId=rev.id,
                                       description="template from cluster")
    remove_list.insert(0, cluster3)
    client.update_by_id_setting(id='cluster-template-enforcement',
                                value="false")


def test_revision_creation_permission(admin_mc, remove_resource,
                                      user_factory):
    user_readonly = user_factory()
    user_owner = user_factory()
    members = [{"userPrincipalId": "local://" + user_readonly.user.id,
                "accessType": "read-only"},
               {"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "owner"}]
    cluster_template = create_cluster_template(admin_mc, members, admin_mc)
    remove_resource(cluster_template)
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_readonly.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    rb_name = name + "-ct-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_owner.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    templateId = cluster_template.id
    # user with accessType=owner should be able to create revision
    # since a standard user can add revisions to template shared
    # with owner access

    create_cluster_template_revision(user_owner.client, templateId)

    # user with read-only accessType should get Forbidden error
    with pytest.raises(ApiError) as e:
        create_cluster_template_revision(user_readonly.client, templateId)
    assert e.value.error.status == 403


def test_updated_members_revision_access(admin_mc, remove_resource,
                                         user_factory):
    # create cluster template without members and a revision
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_resource(cluster_template)
    templateId = cluster_template.id
    rev = \
        create_cluster_template_revision(admin_mc.client, templateId)

    # update template to add a user as member
    user_member = user_factory()
    members = [{"userPrincipalId": "local://" + user_member.user.id,
                "accessType": "read-only"}]
    admin_mc.client.update(cluster_template, members=members)

    # this member should get access to existing revision "rev"
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = rev.id.split(":")
    name = split[1]
    rb_name = name + "-ctr-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_member.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    revision = user_member.client.by_id_cluster_template_revision(rev.id)
    assert revision is not None

    # remove this user from cluster_template members list
    admin_mc.client.update(cluster_template, members=[])

    # now this user should not be able to see that revision
    try:
        user_member.client.by_id_cluster_template_revision(rev.id)
    except ApiError as e:
        assert e.error.status == 403


def test_permissions_removed_on_downgrading_access(admin_mc, remove_resource,
                                                   user_factory):
    user_owner = user_factory()
    remove_resource(user_owner)
    members = [{"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "owner"}]
    # create cluster template with one member having "member" accessType
    cluster_template = create_cluster_template(admin_mc, members, admin_mc)
    remove_resource(cluster_template)

    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_owner.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))

    # user with accessType=owner should be able to update template
    # so adding new member by the user_member should be allowed
    new_member = user_factory()
    remove_resource(new_member)
    members = [{"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "owner"},
               {"userPrincipalId": "local://" + new_member.user.id,
                "accessType": "read-only"}]
    user_owner.client.update(cluster_template, members=members)

    # now change user_owner's accessType to read-only
    members = [{"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "read-only"},
               {"userPrincipalId": "local://" + new_member.user.id,
                "accessType": "read-only"}]
    admin_mc.client.update(cluster_template, members=members)
    rb_name = name + "-ct-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_owner.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))

    # user_owner should not be allowed to update cluster template now
    # test updating members field by removing new_member
    members = [{"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "read-only"}]
    try:
        user_owner.client.update(cluster_template, members=members)
    except ApiError as e:
        assert e.error.status == 403


def test_required_template_question(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_resource(cluster_template)
    tId = cluster_template.id
    client = admin_mc.client

    cconfig = {
        "rancherKubernetesEngineConfig": {
            "services": {
                "type": "rkeConfigServices",
                "kubeApi": {
                    "alwaysPullImages": "false",
                    "podSecurityPolicy": "false",
                    "serviceNodePortRange": "30000-32767",
                    "type": "kubeAPIService"
                }
            }
        },
        "defaultPodSecurityPolicyTemplateId": "restricted",
    }
    questions = [{
                  "variable": "dockerRootDir",
                  "required": "true",
                  "type": "string",
                  "default": ""
                 },
                 {
                  "variable":
                  "rancherKubernetesEngineConfig.ignoreDockerVersion",
                  "required": "false",
                  "type": "boolean",
                  "default": "true"
                 }]

    rev = client.create_cluster_template_revision(name=random_str(),
                                                  clusterConfig=cconfig,
                                                  clusterTemplateId=tId,
                                                  questions=questions,
                                                  enabled="true")

    # creating a cluster with this template with no answer should fail
    answers = {
                "values": {
                    "rancherKubernetesEngineConfig.ignoreDockerVersion":
                    "false"
                }
              }

    try:
        wait_for_cluster_create(client, name=random_str(),
                                clusterTemplateRevisionId=rev.id,
                                description="template from cluster",
                                answers=answers)
    except ApiError as e:
        assert e.error.status == 422


def test_secret_template_answers(admin_mc, remove_resource,
                                 list_remove_resource):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)
    tId = cluster_template.id
    client = admin_mc.client

    cconfig = {
        "rancherKubernetesEngineConfig": {
            "services": {
                "type": "rkeConfigServices",
                "kubeApi": {
                    "alwaysPullImages": "false",
                    "podSecurityPolicy": "false",
                    "serviceNodePortRange": "30000-32767",
                    "type": "kubeAPIService"
                }
            }
        },
        "defaultPodSecurityPolicyTemplateId": "restricted",
    }
    azureClientId = "rancherKubernetesEngineConfig.cloudProvider.\
azureCloudProvider.aadClientId"
    azureClientSecret = "rancherKubernetesEngineConfig.cloudProvider.\
azureCloudProvider.aadClientSecret"

    questions = [{
                  "variable": "dockerRootDir",
                  "required": "true",
                  "type": "string",
                  "default": ""
                 },
                 {
                  "variable": azureClientId,
                  "required": "true",
                  "type": "string",
                  "default": "abcdClientId"
                 },
                 {
                  "variable": azureClientSecret,
                  "required": "true",
                  "type": "string",
                  "default": ""
                 }]

    rev = client.create_cluster_template_revision(name=random_str(),
                                                  clusterConfig=cconfig,
                                                  clusterTemplateId=tId,
                                                  questions=questions,
                                                  enabled="true")

    # creating a cluster with this template
    answers = {
                "values": {
                    "dockerRootDir": "/var/lib/docker123",
                    azureClientId: "abcdClientId",
                    azureClientSecret: "abcdClientSecret"
                }
              }

    cluster = wait_for_cluster_create(client, name=random_str(),
                                      clusterTemplateRevisionId=rev.id,
                                      description="template from cluster",
                                      answers=answers)

    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    assert cluster.answers.values[azureClientId] is not None
    assert azureClientSecret not in cluster.answers.values
    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_member_accesstype_check(admin_mc, user_factory, remove_resource):
    client = admin_mc.client
    user_readonly = user_factory()
    user_owner = user_factory()
    members = [{"userPrincipalId": "local://" + user_readonly.user.id,
                "accessType": "read-only"},
               {"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "member"}]
    # creation with a member with accessType "member" shouldn't be allowed
    try:
        cluster_template = create_cluster_template(admin_mc, members, admin_mc)
        remove_resource(cluster_template)
    except ApiError as e:
        assert e.error.status == 422

    members = [{"userPrincipalId": "local://" + user_readonly.user.id,
                "accessType": "read-only"},
               {"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "owner"}]
    cluster_template = create_cluster_template(admin_mc, members, admin_mc)
    remove_resource(cluster_template)

    updated_members = \
        [{"userPrincipalId": "local://" + user_readonly.user.id,
         "accessType": "read-only"},
         {"userPrincipalId": "local://" + user_owner.user.id,
         "accessType": "member"}]
    # updating a cluster template to add user with access type "member"
    # shouldn't be allowed
    try:
        client.update(cluster_template, members=updated_members)
    except ApiError as e:
        assert e.error.status == 422


def test_create_cluster_with_invalid_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_resource(cluster_template)
    tId = cluster_template.id
    client = admin_mc.client

    # templaterevision with question with invalid format
    cconfig = {
        "rancherKubernetesEngineConfig": {
            "services": {
                "type": "rkeConfigServices",
                "kubeApi": {
                    "alwaysPullImages": "false",
                    "podSecurityPolicy": "false",
                    "serviceNodePortRange": "30000-32767",
                    "type": "kubeAPIService"
                }
            }
        },
        "defaultPodSecurityPolicyTemplateId": "restricted",
    }
    questions = [{
                  "variable": "dockerRootDir",
                  "required": "true",
                  "type": "string",
                  "default": ""
                 },
                 {
                 "default": "map[enabled:true type:localClusterAuthEndpoint]",
                 "required": "false",
                 "type": "string",
                 "variable": "localClusterAuthEndpoint"
                 }]

    rev = client.create_cluster_template_revision(name=random_str(),
                                                  clusterConfig=cconfig,
                                                  clusterTemplateId=tId,
                                                  questions=questions,
                                                  enabled="true")

    # creating a cluster with this template
    try:
        wait_for_cluster_create(client, name=random_str(),
                                clusterTemplateRevisionId=rev.id,
                                description="template from cluster")
    except ApiError as e:
        assert e.error.status == 422


def test_disable_template_revision(admin_mc, list_remove_resource):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)

    tId = cluster_template.id
    client = admin_mc.client
    rev = \
        create_cluster_template_revision(admin_mc.client, tId)

    # creating a cluster with this template
    cluster = wait_for_cluster_create(client, name=random_str(),
                                      clusterTemplateRevisionId=rev.id,
                                      description="template from cluster")
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    # disable the revision
    client.action(obj=rev, action_name="disable")

    try:
        wait_for_cluster_create(client, name=random_str(),
                                clusterTemplateRevisionId=rev.id)
    except ApiError as e:
        assert e.error.status == 500

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_template_delete_by_members(admin_mc, remove_resource,
                                    list_remove_resource, user_factory):
    user_owner = user_factory()
    members = [{"userPrincipalId": "local://" + user_owner.user.id,
                "accessType": "owner"}]
    cluster_template = create_cluster_template(admin_mc, members, admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)

    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_owner.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    templateId = cluster_template.id
    rev = create_cluster_template_revision(user_owner.client, templateId)

    cluster = wait_for_cluster_create(admin_mc.client, name=random_str(),
                                      clusterTemplateRevisionId=rev.id,
                                      description="template from cluster")
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    # user with accessType=owner should not be able to delete this
    # template since a cluster exists
    wait_for_clusterTemplate_update_failure(admin_mc.client, rev)
    with pytest.raises(ApiError) as e:
        user_owner.client.delete(cluster_template)
    assert e.value.error.status == 422

    admin_mc.client.delete(cluster)
    wait_for_cluster_to_be_deleted(admin_mc.client, cluster.id)


def test_template_access(admin_mc, remove_resource, user_factory):
    user = user_factory()
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_resource(cluster_template)
    templateId = cluster_template.id
    rev = create_cluster_template_revision(admin_mc.client, templateId)

    wait_for_clusterTemplate_list_failure(user.client, rev)

    with pytest.raises(ApiError) as e:
        user.client.create_cluster(name=random_str(),
                                   clusterTemplateRevisionId=rev.id,
                                   description="template from cluster")
    assert e.value.error.status == 404


def test_save_as_template_action(admin_mc, list_remove_resource):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)

    templateId = cluster_template.id
    rev = create_cluster_template_revision(admin_mc.client, templateId)

    cluster = wait_for_cluster_create(admin_mc.client, name=random_str(),
                                      clusterTemplateRevisionId=rev.id,
                                      description="template from cluster")
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    try:
        admin_mc.client.action(obj=cluster, action_name="saveAsTemplate", )
    except AttributeError as e:
        assert e is not None


def test_cluster_desc_update(admin_mc, list_remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               [], admin_mc)
    remove_list = [cluster_template]
    list_remove_resource(remove_list)
    templateId = cluster_template.id

    rev = \
        create_cluster_template_revision(admin_mc.client, templateId)

    # create a cluster with this template
    client = admin_mc.client

    cname = random_str()
    cluster = wait_for_cluster_create(admin_mc.client, name=cname,
                                      clusterTemplateRevisionId=rev.id,
                                      description="template from cluster")
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    assert cluster.description == 'template from cluster'

    # edit cluster description
    updatedC = client.update(cluster, name=cname,
                             clusterTemplateRevisionId=rev.id,
                             description="updated desc")
    assert updatedC.description == 'updated desc'

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_update_cluster_monitoring(admin_mc, list_remove_resource):
    cluster_template = create_cluster_template(admin_mc, [], admin_mc)

    remove_list = [cluster_template]
    list_remove_resource(remove_list)

    tId = cluster_template.id
    client = admin_mc.client
    cconfig = {
        "rancherKubernetesEngineConfig": {
            "services": {
                "type": "rkeConfigServices",
                "kubeApi": {
                    "alwaysPullImages": "false",
                    "podSecurityPolicy": "false",
                    "serviceNodePortRange": "30000-32767",
                    "type": "kubeAPIService"
                }
            }
        },
        "enableClusterMonitoring": "true",
        "defaultPodSecurityPolicyTemplateId": "restricted",
    }
    rev1 = client.create_cluster_template_revision(clusterConfig=cconfig,
                                                   name="v1",
                                                   clusterTemplateId=tId,
                                                   enabled="true")
    cconfig2 = {
        "rancherKubernetesEngineConfig": {
            "services": {
                "type": "rkeConfigServices",
                "kubeApi": {
                    "alwaysPullImages": "false",
                    "podSecurityPolicy": "false",
                    "serviceNodePortRange": "30000-32767",
                    "type": "kubeAPIService"
                }
            }
        },
        "enableClusterMonitoring": "false",
        "defaultPodSecurityPolicyTemplateId": "restricted",
    }
    rev2 = client.create_cluster_template_revision(clusterConfig=cconfig2,
                                                   name="v2",
                                                   clusterTemplateId=tId,
                                                   enabled="true")

    cluster_name = random_str()
    cluster = wait_for_cluster_create(client, name=cluster_name,
                                      clusterTemplateRevisionId=rev1.id,
                                      description="template from cluster")
    remove_list.insert(0, cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    # update cluster to use rev2 that turns off monitoring
    # expect no change to monitoring

    client.update(cluster,
                  name=cluster_name, clusterTemplateRevisionId=rev2.id)

    reloaded_cluster = client.by_id_cluster(cluster.id)
    assert reloaded_cluster.enableClusterMonitoring is True

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def rtb_cb(client, rtb):
    """Wait for the prtb to have the userId populated"""
    def cb():
        rt = client.reload(rtb)
        return rt.userPrincipalId is not None
    return cb


def grb_cb(client, grb):
    """Wait for the grb to have the userId populated"""
    def cb():
        rt = client.reload(grb)
        return rt.userId is not None
    return cb


# When calling this function you _must_ remove the cluster_template manually
# If a cluster is created also it must be removed after the template
def create_cluster_template(creator, members, admin_mc):
    template_name = random_str()

    cluster_template = \
        creator.client.create_cluster_template(
                                         name=template_name,
                                         description="demo template",
                                         members=members)
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    rb_name = cluster_template.id.split(":")[1] + "-ct-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         creator.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))

    return cluster_template


def create_cluster_template_revision(client, clusterTemplateId):
    rke_config = getRKEConfig()

    cluster_config = {
        "dockerRootDir": "/var/lib/docker",
        "enableClusterAlerting": "false",
        "enableClusterMonitoring": "false",
        "enableNetworkPolicy": "false",
        "type": "clusterSpecBase",
        "localClusterAuthEndpoint": {
            "enabled": "true",
            "type": "localClusterAuthEndpoint"
        },
        "rancherKubernetesEngineConfig": rke_config
    }

    questions = [{
                  "variable": "dockerRootDir",
                  "required": "false",
                  "type": "string",
                  "default": "/var/lib/docker"
                 },
                 {
                  "variable":
                  "rancherKubernetesEngineConfig.ignoreDockerVersion",
                  "required": "false",
                  "type": "boolean",
                  "default": "true"
                 },
                 {
                  "variable":
                  "rancherKubernetesEngineConfig.kubernetesVersion",
                  "required": "false",
                  "type": "string",
                  "default": "1.18.x"
                 }]

    revision_name = random_str()

    cluster_template_revision = \
        client.create_cluster_template_revision(
                                        name=revision_name,
                                        clusterConfig=cluster_config,
                                        clusterTemplateId=clusterTemplateId,
                                        disabled="false",
                                        questions=questions
                                        )

    return cluster_template_revision


def getRKEConfig():
    rke_config = {
        "addonJobTimeout": 30,
        "ignoreDockerVersion": "true",
        "sshAgentAuth": "false",
        "type": "rancherKubernetesEngineConfig",
        "kubernetesVersion": "1.15.x",
        "authentication": {
            "strategy": "x509",
            "type": "authnConfig"
        },
        "network": {
            "plugin": "canal",
            "type": "networkConfig",
            "options": {
                "flannel_backend_type": "vxlan"
            }
        },
        "ingress": {
            "provider": "nginx",
            "type": "ingressConfig"
        },
        "monitoring": {
            "provider": "metrics-server",
            "type": "monitoringConfig"
        },
        "services": {
            "type": "rkeConfigServices",
            "kubeApi": {
                "alwaysPullImages": "false",
                "podSecurityPolicy": "false",
                "serviceNodePortRange": "30000-32767",
                "type": "kubeAPIService"
            },
            "etcd": {
                "creation": "12h",
                "extraArgs": {
                    "heartbeat-interval": 500,
                    "election-timeout": 5000
                },
                "retention": "72h",
                "snapshot": "false",
                "type": "etcdService",
                "backupConfig": {
                    "enabled": "true",
                    "intervalHours": 12,
                    "retention": 6,
                    "type": "backupConfig"
                }
            }
        }
    }
    return rke_config


def wait_for_cluster_to_be_deleted(client, clusterId, timeout=45):
    deleted = False
    start = time.time()
    interval = 0.5
    while not deleted:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for clusters")
        cluster = client.by_id_cluster(clusterId)
        if cluster is None:
            deleted = True
        time.sleep(interval)
        interval *= 2


def wait_for_default_revision(client, templateId, revisionId, timeout=60):
    updated = False
    interval = 0.5
    start = time.time()
    while not updated:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for clustertemplate to update')
        template_reloaded = client.by_id_cluster_template(templateId)
        if template_reloaded.defaultRevisionId is not None:
            updated = True
        time.sleep(interval)
        interval *= 2


def fail_handler(resource):
    return "failed waiting for clustertemplate" + resource + " to get updated"


def wait_for_cluster_create(client, **kwargs):
    timeout = DEFAULT_TIMEOUT
    interval = 0.5
    start = time.time()
    while True:
        try:
            return client.create_cluster(kwargs)
        except ApiError as e:
            if e.error.status != 404:
                raise e
            if time.time() - start > timeout:
                exception_msg = 'Timeout waiting for condition.'
                raise Exception(exception_msg)
            time.sleep(interval)
            interval *= 2


def wait_for_clusterTemplate_update_failure(client, revision, timeout=45):
    updateWorks = True
    start = time.time()
    interval = 0.5
    cconfig = {
        "rancherKubernetesEngineConfig": {
        }
    }
    while updateWorks:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for clustertemplate update failure")
        try:
            client.update(revision, name=random_str(), clusterConfig=cconfig)
        except ApiError as e:
            if e.error.status == 422:
                updateWorks = False

        time.sleep(interval)
        interval *= 2


def wait_for_clusterTemplate_list_failure(client, revision, timeout=45):
    listWorks = True
    start = time.time()
    interval = 0.5
    while listWorks:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for clustertemplate list failure")
        try:
            client.by_id_cluster_template_revision(revision.id)
        except ApiError as e:
            if e.error.status == 403:
                listWorks = False
        time.sleep(interval)
        interval *= 2
