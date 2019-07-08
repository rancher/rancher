from .common import random_str, check_subject_in_rb
from rancher import ApiError
from .conftest import wait_until, wait_for
import pytest
import time
import kubernetes

rb_resource = 'rolebinding'


def test_create_cluster_template_with_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               remove_resource, [], admin_mc)
    templateId = cluster_template.id
    _ = \
        create_cluster_template_revision(admin_mc.client, templateId)
    _ = \
        create_cluster_template_revision(admin_mc.client, templateId)
    client = admin_mc.client
    template_reloaded = client.by_id_cluster_template(cluster_template.id)
    assert template_reloaded.links.revisions is not None


def test_create_template_revision_invalid_k8s(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               remove_resource, [], admin_mc)
    tId = cluster_template.id
    client = admin_mc.client

    cconfig = {
        "rancherKubernetesEngineConfig": {
            "kubernetesVersion": "1.13"
        }
    }
    with pytest.raises(ApiError) as e:
        client.create_cluster_template_revision(clusterConfig=cconfig,
                                                clusterTemplateId=tId,
                                                enabled="true")
        assert e.value.error.status == 422


def test_default_pod_sec(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc,
                                               remove_resource, [], admin_mc)
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
    rev = client.create_cluster_template_revision(clusterConfig=cconfig,
                                                  clusterTemplateId=tId,
                                                  enabled="true")

    cluster = client.create_cluster(name=random_str(),
                                    clusterTemplateRevisionId=rev.id)

    remove_resource(cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    assert cluster.defaultPodSecurityPolicyTemplateId == "restricted"
    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_check_default_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               [], admin_mc)
    templateId = cluster_template.id
    first_revision = \
        create_cluster_template_revision(admin_mc.client, templateId)
    client = admin_mc.client
    wait_for_default_revision(client, templateId, first_revision.id)
    # delete the cluster template revision, it should error out
    with pytest.raises(ApiError) as e:
        client.delete(first_revision)
        assert e.value.error.status == 403


def test_create_cluster_with_template(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               [], admin_mc)
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
    cluster = client.create_cluster(name=random_str(),
                                    clusterTemplateRevisionId=revId,
                                    description="template from cluster",
                                    answers=answers)
    remove_resource(cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'
    k8s_version = cluster.rancherKubernetesEngineConfig.kubernetesVersion
    assert k8s_version != "v1.13.x"

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
        assert e.value.error.status == 403

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_create_cluster_validations(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               [], admin_mc)
    templateId = cluster_template.id
    template_revision = \
        create_cluster_template_revision(admin_mc.client, templateId)
    # create a cluster with this template
    revId = template_revision.id
    client = admin_mc.client
    rConfig = getRKEConfig()
    with pytest.raises(ApiError) as e:
        client.create_cluster(name=random_str(),
                              clusterTemplateRevisionId=revId,
                              description="template from cluster",
                              rancherKubernetesEngineConfig=rConfig)
        assert e.value.error.status == 500


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
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               members, admin_mc)
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
                    "accessType": "read-only"}, {"groupPrincipalId": "*"}]
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
    cluster_template = create_cluster_template(user_member, remove_resource,
                                               [], admin_mc)
    id = cluster_template.id
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    ct = um_client.by_id_cluster_template(id)
    assert ct is not None

    templateId = cluster_template.id
    template_revision = \
        create_cluster_template_revision(um_client, templateId)
    rb_name = template_revision.id.split(":")[1] + "-ctr-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_member.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    ctr = um_client.by_id_cluster_template_revision(template_revision.id)
    assert ctr is not None


def test_check_enforcement(admin_mc, remove_resource, user_factory):
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               [], admin_mc)
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
    remove_resource(cluster)

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
    cluster = user_client.create_cluster(
            name=random_str(), amazonElasticContainerServiceConfig={
                "accessKey": "asdfsd"})
    remove_resource(cluster)

    # a user cannot create an rke cluster without a public
    # and enforced template
    with pytest.raises(ApiError) as e:
        user_client.create_cluster(name=random_str(),
                                   clusterTemplateRevisionId=rev.id,
                                   description="cluster from temp")
        assert e.value.error.status == 403

    # a user can create an rke cluster with a public and enforced template
    client.update(cluster_template, enforced="true")
    template_reloaded = client.by_id_cluster_template(templateId)
    new_members = [{"groupPrincipalId": "*"}]
    client.update(template_reloaded, members=new_members)

    cluster2 = user_client.create_cluster(name=random_str(),
                                          clusterTemplateRevisionId=rev.id,
                                          description="cluster from temp")
    remove_resource(cluster2)
    client.update_by_id_setting(id='cluster-template-enforcement',
                                value="false")

    # check that user cannot update the enforced flag
    user_template = create_cluster_template(user, remove_resource,
                                            [], admin_mc)
    with pytest.raises(ApiError) as e:
        user_client.update(user_template, enforced="true")
        assert e.value.error.status == 403


def test_revision_creation_permission(admin_mc, remove_resource,
                                      user_factory):
    user_readonly = user_factory()
    user_member = user_factory()
    members = [{"userPrincipalId": "local://" + user_readonly.user.id,
                "accessType": "read-only"},
               {"userPrincipalId": "local://" + user_member.user.id,
                "accessType": "member"}]
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               members, admin_mc)
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-r"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_readonly.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    rb_name = name + "-ct-u"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_member.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    templateId = cluster_template.id
    # user with accessType=member should be able to create revision
    rev = create_cluster_template_revision(user_member.client, templateId)
    assert rev is not None
    # user with read-only accessType should get Forbidden error
    try:
        create_cluster_template_revision(user_readonly.client, templateId)
    except ApiError as e:
        assert e.error.status == 403
        assert "read-only member of clustertemplate cannot " \
               "create revisions for it" in e.error.message


def test_updated_members_revision_access(admin_mc, remove_resource,
                                         user_factory):
    # create cluster template without members and a revision
    cluster_template = create_cluster_template(admin_mc, remove_resource,
                                               [], admin_mc)
    templateId = cluster_template.id
    rev = \
        create_cluster_template_revision(admin_mc.client, templateId)

    # update template to add a user as member
    user_member = user_factory()
    members = [{"userPrincipalId": "local://" + user_member.user.id,
                "accessType": "member"}]
    admin_mc.client.update(cluster_template, members=members)

    # this member should get access to existing revision "rev"
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = rev.id.split(":")
    name = split[1]
    rb_name = name + "-ctr-u"
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


def rtb_cb(client, rtb):
    """Wait for the prtb to have the userId populated"""
    def cb():
        rt = client.reload(rtb)
        return rt.userPrincipalId is not None
    return cb


def create_cluster_template(creator, remove_resource, members, admin_mc):
    template_name = random_str()

    cluster_template = \
        creator.client.create_cluster_template(
                                         name=template_name,
                                         description="demo template",
                                         members=members)
    remove_resource(cluster_template)
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
                 }]

    cluster_template_revision = \
        client.create_cluster_template_revision(
                                        clusterConfig=cluster_config,
                                        clusterTemplateId=clusterTemplateId,
                                        disabled="false",
                                        kubernetesVersion="v1.13.5-rancher1-3",
                                        questions=questions
                                        )

    return cluster_template_revision


def getRKEConfig():
    rke_config = {
        "addonJobTimeout": 30,
        "ignoreDockerVersion": "true",
        "sshAgentAuth": "false",
        "type": "rancherKubernetesEngineConfig",
        "kubernetesVersion": "1.13.x",
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
