import time
import os

k8s_resurce_dir = os.path.join(os.path.dirname(os.path.realpath(__file__)), 
                          "resources/k8s_ymls/")


def create_and_validate(
    cloud_provider, rke_client, kubectl, rke_template, nodes,
    base_namespace="ns", network_validation=None, dns_validation=None,
        teardown=False, remove_nodes=False, etcd_private_ip=False):

    create_rke_cluster(rke_client, kubectl, nodes, rke_template)
    network_validation, dns_validation = validate_rke_cluster(
        rke_client, kubectl, nodes, base_ns=base_namespace,
        network_validation=network_validation, dns_validation=dns_validation,
        teardown=teardown, etcd_private_ip=etcd_private_ip)

    if remove_nodes:
        delete_nodes(cloud_provider, nodes)

    return network_validation, dns_validation


def delete_nodes(cloud_provider, nodes):
    for node in nodes:
        cloud_provider.delete_node(node)


def create_rke_cluster(
        rke_client, kubectl, nodes, rke_template, **rke_template_kwargs):
    """
    Creates a cluster and returns the rke config as a python dictionary
    """

    # create rke cluster yml
    config_yml, nodes = rke_client.build_rke_template(
        rke_template, nodes, **rke_template_kwargs)

    # run rke up
    result = rke_client.up(config_yml)

    # validate k8s reachable
    kubectl.kube_config_path = rke_client.kube_config_path()

    return rke_client.convert_to_dict(config_yml)


def validate_rke_cluster(rke_client, kubectl, nodes, base_ns='one',
                         network_validation=None, dns_validation=None,
                         teardown=False, etcd_private_ip=False):
    """
    General rke up test validation, runs validations methods for:
    - node roles validation
    - intercommuincation per pod
    - dns service discovery validation
    If teardown is true, removes any resources created for validation
    """

    validation_node_roles(nodes, kubectl.get_nodes(), etcd_private_ip)
    if network_validation is None:
        network_validation = PodIntercommunicationValidation(kubectl, base_ns)
        network_validation.setup()
    if dns_validation is None:
        dns_validation = DNSServiceDiscoveryValidation(kubectl, base_ns)
        dns_validation.setup()

    network_validation.validate()
    dns_validation.validate()

    if teardown:
        network_validation.teardown()
        dns_validation.teardown()

    return network_validation, dns_validation


def match_nodes(nodes, k8s_nodes):
    """
    Builds a list of tuples, where:
    nodes_to_k8s_nodes[0][0] is the node object matched to
    nodes_to_k8s_nodes[0][1] is the k8s info for the same node
    """
    k8s_node_names = []
    for k8s_node in k8s_nodes['items']:
        k8s_node_names.append(
            k8s_node['metadata']['labels']['kubernetes.io/hostname'])

    nodes_to_k8s_nodes = []
    for node in nodes:
        for k8s_node in k8s_nodes['items']:
            hostname = k8s_node['metadata']['labels']['kubernetes.io/hostname']
            if hostname in node.node_name:
                nodes_to_k8s_nodes.append((node, k8s_node))
                break
        else:
            raise Exception(
                "Did not find provisioned node's '{0}' corresponding nodes "
                "resourse in cluster: {1}".format(
                    node.node_name, k8s_node_names))
    return nodes_to_k8s_nodes


def assert_containers_exist_for_roles(roles, containers):
    # All nodes will have these containers:
    expect_containers = ['kubelet', 'kube-proxy']

    # Add extra containers depending on roles present
    if 'controlplane' in roles:
        expect_containers.extend(
            ['kube-scheduler', 'kube-controller-manager', 'kube-apiserver'])
    else:
        expect_containers.extend(['nginx-proxy'])
    if 'etcd' in roles:
        expect_containers.extend(['etcd'])

    missing_containers = expect_containers[:]
    for container in containers:
        if container in expect_containers:
            missing_containers.remove(container)
    assert len(missing_containers) == 0, \
        "Missing expected containers for role '{0}': {1}".format(
            roles, missing_containers)


def wait_for_etcd_cluster_health(node, etcd_private_ip=False):
    result = ''
    endpoints = "127.0.0.1"
    if etcd_private_ip:
        endpoints = node.private_ip_address
    etcd_tls_cmd = (
        'ETCDCTL_API=2 etcdctl --endpoints "https://'+endpoints+':2379" '
        ' --ca-file /etc/kubernetes/ssl/kube-ca.pem --cert-file '
        ' $ETCDCTL_CERT --key-file '
        ' $ETCDCTL_KEY cluster-health')

    print(etcd_tls_cmd)
    start_time = time.time()
    while start_time - time.time() < 120:
        result = node.docker_exec('etcd', "sh -c '" + etcd_tls_cmd + "'")
        print("**RESULT**")
        print(result)
        if 'cluster is healthy' in result:
            break
        time.sleep(5)
    return result


def validation_node_roles(nodes, k8s_nodes, etcd_private_ip=False):
    """
    Validates each node's labels for match its roles
    Validates each node's running containers match its role
    Validates etcd etcdctl cluster-health command
    Validates worker nodes nginx-proxy conf file for controlplane ips
    """

    role_matcher = {
        'worker': 'node-role.kubernetes.io/worker',
        'etcd': 'node-role.kubernetes.io/etcd',
        'controlplane': 'node-role.kubernetes.io/controlplane'}

    controlplane_ips = []
    etcd_members = []
    for node in nodes:
        if 'controlplane' in node.roles:
            controlplane_ips.append(node.node_address)
        if 'etcd' in node.roles:
            etcd_members.append(node.node_address)

    nodes_to_k8s_nodes = match_nodes(nodes, k8s_nodes)
    for node, k8s_node in nodes_to_k8s_nodes:
        containers = list(node.docker_ps().keys())
        assert_containers_exist_for_roles(node.roles, containers)
        k8s_node_labels = list(k8s_node['metadata']['labels'].keys())
        for role in node.roles:
            assert role_matcher[role] in k8s_node_labels, \
                "Expected label '{0}' not in labels: {1}".format(
                    role_matcher[role], k8s_node_labels)

            # nodes with controlplane roles do not have nginx-proxy
            if (role == 'worker' or role == 'etcd') and \
                    ('controlplane' not in node.roles):
                result = node.docker_exec(
                    'nginx-proxy', 'cat /etc/nginx/nginx.conf')
                for ip in controlplane_ips:
                    assert 'server {0}:6443'.format(ip) in result, (
                        "Expected to find all controlplane node addresses {0}"
                        "in nginx.conf: {1}".format(controlplane_ips, result))
            if role == 'etcd':
                if len(node.roles) == 1:
                    for taint in k8s_node['spec']['taints']:
                        if taint['key'] == 'node-role.kubernetes.io/etcd':
                            assert taint['effect'] == 'NoExecute', (
                                "{0} etcd-only node's taint is not 'NoExecute'"
                                ": {1}".format(node.node_name, taint['effect'])
                            )
                            # found, do not complete for loop
                            # or else an assertion will be raised
                            break
                    else:
                        assert False, \
                            "Expected to find taint for etcd-only node"
                # check etcd membership and cluster health
                result = wait_for_etcd_cluster_health(node, etcd_private_ip)
                for member in etcd_members:
                    expect = "got healthy result from https://{}".format(
                        member)
                    assert expect in result, result
                assert 'cluster is healthy' in result, result


class PodIntercommunicationValidation(object):
    def __init__(self, kubectl, base_namespace):
        self.kubectl = kubectl
        self.yml_file = (
            k8s_resurce_dir + 'daemonset_pods_per_node.yml')
        self.ns_out = 'daemonset-out-{}'.format(base_namespace)
        self.ns_in = 'daemonset-in-{}'.format(base_namespace)
        self.selector = 'name=daemonset-test1'

    def setup(self):
        self.kubectl.create_ns(self.ns_out)
        result = self.kubectl.create_resourse_from_yml(
            self.yml_file, namespace=self.ns_out)

        self.kubectl.create_ns(self.ns_in)
        result = self.kubectl.create_resourse_from_yml(
            self.yml_file, namespace=self.ns_in)

    def validate(self):
        """
        Gets pod name, pod ip, host ip, and containers
        For each pod, use kubectl exec to ping all other pod ips
        Asserts that each ping is successful
        Tears down daemonset
        """
        # get number of expected pods
        worker_nodes = self.kubectl.get_resource(
            'nodes', selector='node-role.kubernetes.io/worker=true')
        master_nodes = self.kubectl.get_resource(
            'nodes', selector='node-role.kubernetes.io/controlplane=true')
        node_names = [n['metadata']['name'] for n in worker_nodes['items']]
        expected_number_pods = len(worker_nodes['items'])
        """
        for master_node in master_nodes['items']:
            if master_node['metadata']['name'] not in node_names:
                expected_number_pods += 1
        """
        # get pods on each node/namespaces to test intercommunication
        # with pods on different nodes
        pods_to_ping = self.kubectl.wait_for_pods(
            selector=self.selector, namespace=self.ns_in,
            number_of_pods=expected_number_pods)
        pods_from_which_ping = self.kubectl.wait_for_pods(
            selector=self.selector, namespace=self.ns_out,
            number_of_pods=expected_number_pods)

        # verify daemonset pods are on all worker nodes
        assert len(pods_to_ping['items']) == expected_number_pods, (
            "DaemonSet number of pods '{0}', does not match number of worker "
            "nodes '{1}'".format(
                len(pods_to_ping['items']), expected_number_pods))
        assert len(pods_from_which_ping['items']) == expected_number_pods, (
            "DaemonSet number of pods '{0}', does not match number of worker "
            "nodes '{1}'".format(
                len(pods_from_which_ping['items']), expected_number_pods))

        pod_ips_to_ping = []
        for pod in pods_to_ping['items']:
            pod_ips_to_ping.append(pod['status']['podIP'])

        pod_names_to_ping_from = []
        for pod in pods_from_which_ping['items']:
            pod_names_to_ping_from.append(pod['metadata']['name'])

        # From each pod of daemonset in namespace ns_out, ping all pods
        # in from second daemonset in ns_in
        expect_result = \
            '1 packets transmitted, 1 received, 0% packet loss'
        for pod_name in pod_names_to_ping_from:
            for pod_ip in pod_ips_to_ping:
                cmd = 'ping -c 1 {0}'.format(pod_ip)
                for _ in range(10):
                    result = self.kubectl.exec_cmd(pod_name, cmd, self.ns_out)
                    assert expect_result in result, (
                        "Could not ping pod with ip {0} from pod {1}:\n"
                        "stdout: {2}\n".format(
                            pod_ip, pod_name, result))

    def teardown(self):
        """
        Deletes a daemonset of pods and namespace
        """
        result = self.kubectl.delete_resourse_from_yml(
            self.yml_file, namespace=self.ns_out)
        result = self.kubectl.delete_resourse_from_yml(
            self.yml_file, namespace=self.ns_in)
        self.kubectl.delete_resourse('namespace', self.ns_out)
        self.kubectl.delete_resourse('namespace', self.ns_in)


class DNSServiceDiscoveryValidation(object):
    def __init__(self, kubectl, base_namespace):
        namespace_one = 'nsone-{}'.format(base_namespace)
        namespace_two = 'nstwo-{}'.format(base_namespace)
        self.namespace = namespace_one
        self.services = {
            'k8test1': {
                'namespace': namespace_one,
                'selector': 'k8s-app=k8test1-service',
                'yml_file': k8s_resurce_dir + 'service_k8test1.yml',
            },
            'k8test2': {
                'namespace': namespace_two,
                'selector': 'k8s-app=k8test2-service',
                'yml_file': k8s_resurce_dir + 'service_k8test2.yml',
            }
        }
        self.pod_selector = 'k8s-app=pod-test-util'
        self.kubectl = kubectl

    def setup(self):

        for service_name, service_info in self.services.items():
            # create service
            result = self.kubectl.create_ns(service_info['namespace'])

            result = self.kubectl.create_resourse_from_yml(
                service_info['yml_file'], namespace=service_info['namespace'])

        result = self.kubectl.create_resourse_from_yml(
            k8s_resurce_dir + 'single_pod.yml',
            namespace=self.namespace)

    def validate(self):
        # wait for exec pod to be ready before validating
        pods = self.kubectl.wait_for_pods(
            selector=self.pod_selector, namespace=self.namespace)
        exec_pod_name = pods['items'][0]['metadata']['name']

        # Get Cluster IP and pod names per service
        dns_records = {}
        for service_name, service_info in self.services.items():
            # map expected IP to dns service name
            dns = "{0}.{1}.svc.cluster.local".format(
                service_name, service_info['namespace'])
            svc = self.kubectl.get_resource(
                'svc', name=service_name, namespace=service_info['namespace'])
            service_pods = self.kubectl.wait_for_pods(
                selector=service_info['selector'],
                namespace=service_info['namespace'], number_of_pods=2)
            svc_cluster_ip = svc["spec"]["clusterIP"]
            dns_records[dns] = {
                'ip': svc_cluster_ip,
                'pods': [p['metadata']['name'] for p in service_pods['items']]
            }

        for dns_record, dns_info in dns_records.items():
            # Check dns resolution
            expected_ip = dns_info['ip']
            cmd = 'dig {0} +short'.format(dns_record)
            result = self.kubectl.exec_cmd(exec_pod_name, cmd, self.namespace)
            assert expected_ip in result, (
                "Unable to test DNS resolution for service {0}: {1}".format(
                    dns_record, result.stderr))

            # Check Cluster IP reaches pods in service
            pods_names = dns_info['pods']
            cmd = 'curl "http://{0}/name.html"'.format(dns_record)
            result = self.kubectl.exec_cmd(exec_pod_name, cmd, self.namespace)
            print(result)
            print(pods_names)
            assert result.rstrip() in pods_names, (
                "Service ClusterIP does not reach pods {0}".format(
                    dns_record))

    def teardown(self):
        self.kubectl.delete_resourse(
            'pod', 'pod-test-util', namespace=self.namespace)

        for service_name, service_info in self.services.items():
            self.kubectl.delete_resourse_from_yml(
                service_info['yml_file'], namespace=service_info['namespace'])
            self.kubectl.delete_resourse(
                'namespace', service_info['namespace'])


def validate_k8s_service_images(nodes, expectedimagesdict):
    """
    expected_images should be sent from the tests
    This verifies that the nodes have the correct image version
    This does not validate containers per role,
    assert_containers_exist_for_roles method does that
    """

    for node in nodes:
        containers = node.docker_ps()
        allcontainers = node.docker_ps(includeall=True)
        print("Container Dictionary ")
        print(containers)
        print("All containers dictionary")
        print(allcontainers)
        sidekickservice = "service-sidekick"
        for key in expectedimagesdict.keys():
            servicename = key
            if servicename in containers:
                print("Service name")
                print(servicename)
                print(expectedimagesdict[servicename])
                print(containers[servicename])
                assert expectedimagesdict[servicename] == \
                   containers[servicename], (
                   "K8s service '{0}' does not match config version "
                   "{1}, found {2} on node {3}".format(
                   servicename, expectedimagesdict[servicename],
                   containers[servicename], node.node_name))
        if sidekickservice in expectedimagesdict.keys():
            if sidekickservice in allcontainers:
                print("sidekick-service in allcontainers")
                print(sidekickservice)
                print(expectedimagesdict[sidekickservice])
                print(allcontainers[sidekickservice])
                assert expectedimagesdict[sidekickservice] == \
                    allcontainers[sidekickservice], (
                    "K8s service '{0}' does not match config version "
                    "{1}, found {2} on node {3}".format(
                    sidekickservice, expectedimagesdict[sidekickservice],
                    allcontainers[sidekickservice], node.node_name))


def validate_remove_cluster(nodes):
    """
    Removes all k8s services containers on each node:
    ['kubelet', 'kube-proxy', 'kube-scheduler', 'kube-controller-manager',
     'kube-apiserver', 'nginx-proxy']
    Removes files from these directories:
    ['/etc/kubernetes/ssl', '/var/lib/etcd'
     '/etc/cni', '/opt/cni', '/var/run/calico']
    """
    k8s_services = [
        'kubelet', 'kube-proxy', 'kube-scheduler', 'kube-controller-manager',
        'kube-apiserver', 'nginx-proxy'
    ]
    rke_cleaned_directories = [
        '/etc/kubernetes/ssl', '/var/lib/etcd' '/etc/cni', '/opt/cni',
        '/var/run/calico'
    ]
    for node in nodes:
        containers = node.docker_ps()
        for service in k8s_services:
            assert service not in list(containers.keys()), (
                "Found kubernetes service '{0}' still running on node '{1}'"
                .format(service, node.node_name))

        for directory in rke_cleaned_directories:
            result = node.execute_command('ls ' + directory)
            assert result[0] == '', (
                "Found a non-empty directory '{0}' after remove on node '{1}'"
                .format(directory, node.node_name))


def validate_dashboard(kubectl):
    # Start dashboard
    # Validated it is reachable
    pass
