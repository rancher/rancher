from .common import random_str
import kubernetes

from .conftest import kubernetes_api_client, user_project_client


def test_dep_creation_kubectl(admin_mc, admin_cc, remove_resource):
    name = random_str()
    project = admin_mc.client.create_project(name=random_str(),
                                             clusterId='local')
    remove_resource(project)
    namespace_name = random_str()
    ns = admin_cc.client.create_namespace(name=namespace_name,
                                          projectId=project.id)
    remove_resource(ns)

    k8s_client = kubernetes_api_client(admin_mc.client, 'local')
    d_api = kubernetes.client.AppsV1Api(k8s_client)

    d = kubernetes.client.V1beta2Deployment()
    # Metadata
    d.metadata = kubernetes.client.V1ObjectMeta(
        name=name,
        namespace=namespace_name)
    pod_meta = kubernetes.client.V1ObjectMeta(
        labels={"foo": "bar"})
    port = kubernetes.client.V1ContainerPort(
        container_port=80,
        host_port=8099,
    )
    container = {"name": "nginx", "image": "nginx:1.7.9", "ports": [port]}
    spec = kubernetes.client.V1PodSpec(
        containers=[container])
    template = kubernetes.client.V1PodTemplateSpec(
        metadata=pod_meta,
        spec=spec
    )
    selector = kubernetes.client.V1LabelSelector(
        match_labels={"foo": "bar"}
    )

    d.spec = kubernetes.client.V1beta2DeploymentSpec(
        selector=selector,
        template=template
    )
    dep = d_api.create_namespaced_deployment(namespace=namespace_name,
                                             body=d)
    remove_resource(dep)
    assert dep is not None

    # now get this through rancher api as namespacedCertificate
    p_client = user_project_client(admin_mc, project)
    d = p_client.list_workload(name=name, namespace=namespace_name).data[0]
    assert d is not None
    port = d['containers'][0]['ports'][0]
    assert port['sourcePort'] == 8099
    assert port['kind'] == 'HostPort'


def test_port(admin_pc):
    client = admin_pc.client

    ports = [{
        'sourcePort': 776,
        'containerPort': 80,
        'kind': 'HostPort',
        'protocol': 'TCP', },
        {
        'sourcePort': 777,
        'containerPort': 80,
        'kind': 'NodePort',
        'protocol': 'TCP', },
        {
        'sourcePort': 778,
        'containerPort': 80,
        'kind': 'LoadBalancer',
        'protocol': 'TCP', },
        {
        'sourcePort': 779,
        'containerPort': 80,
        'kind': 'ClusterIP',
        'protocol': 'TCP', },
        ]

    for port in ports:
        ns = admin_pc.cluster.client.create_namespace(name=random_str(),
                                                      projectId=admin_pc.
                                                      project.id)
        name = random_str()

        workload = client.create_workload(
            name=name,
            namespaceId=ns.id,
            scale=1,
            containers=[{
                'name': 'one',
                'image': 'nginx',
                'ports': [port],
            }])
        workload_ports = workload['containers'][0]['ports']
        assert workload_ports is not None
        assert workload_ports[0]['kind'] == port['kind']
        assert workload_ports[0]['containerPort'] == port['containerPort']
        assert workload_ports[0]['sourcePort'] == port['sourcePort']
