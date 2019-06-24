from .common import random_str, check_subject_in_rb
from rancher import ApiError
from .conftest import wait_for
import pytest
import time
import kubernetes

rb_resource = 'rolebinding'


def test_create_cluster_template_with_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc.client,
                                               remove_resource, [])
    templateId = cluster_template.id
    _ = \
        create_cluster_template_revision(admin_mc.client, templateId)
    _ = \
        create_cluster_template_revision(admin_mc.client, templateId)
    client = admin_mc.client
    template_reloaded = client.by_id_cluster_template(cluster_template.id)
    assert template_reloaded.links.revisions is not None


def test_check_default_revision(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc.client,
                                               remove_resource, [])
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
    cluster_template = create_cluster_template(admin_mc.client,
                                               remove_resource, [])
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
                                    clusterTemplateId=cluster_template.id,
                                    clusterTemplateRevisionId=revId,
                                    description="template from cluster",
                                    answers=answers)
    remove_resource(cluster)
    assert cluster.conditions[0].type == 'Pending'
    assert cluster.conditions[0].status == 'True'

    # delete the cluster template, it should error out
    with pytest.raises(ApiError) as e:
        client.delete(cluster_template)
        assert e.value.error.status == 403

    client.delete(cluster)
    wait_for_cluster_to_be_deleted(client, cluster.id)


def test_create_cluster_validations(admin_mc, remove_resource):
    cluster_template = create_cluster_template(admin_mc.client,
                                               remove_resource, [])
    templateId = cluster_template.id
    template_revision = \
        create_cluster_template_revision(admin_mc.client, templateId)
    # create a cluster with this template
    revId = template_revision.id
    client = admin_mc.client
    rConfig = getRKEConfig()
    with pytest.raises(ApiError) as e:
        client.create_cluster(name=random_str(),
                              clusterTemplateId=cluster_template.id,
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
    cluster_template = create_cluster_template(admin_mc.client,
                                               remove_resource, members)
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
    cluster_template = create_cluster_template(um_client, remove_resource,
                                               [])
    id = cluster_template.id
    rbac = kubernetes.client.RbacAuthorizationV1Api(admin_mc.k8s_client)
    split = cluster_template.id.split(":")
    name = split[1]
    rb_name = name + "-ct-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_member.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    ct = um_client.by_id_cluster_template(id)
    assert ct is not None

    templateId = cluster_template.id
    template_revision = \
        create_cluster_template_revision(um_client, templateId)
    split = template_revision.id.split(":")
    name = split[1]
    rb_name = name + "-ctr-a"
    wait_for(lambda: check_subject_in_rb(rbac, 'cattle-global-data',
                                         user_member.user.id, rb_name),
             timeout=60,
             fail_handler=fail_handler(rb_resource))
    ctr = um_client.by_id_cluster_template_revision(template_revision.id)
    assert ctr is not None


def create_cluster_template(client, remove_resource, members):
    template_name = random_str()

    cluster_template = \
        client.create_cluster_template(
                                         name=template_name,
                                         description="demo template",
                                         members=members)
    remove_resource(cluster_template)
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
        "kubernetesVersion": "v1.13.5-rancher1-3",
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
