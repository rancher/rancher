import os
import json
import time
import subprocess

DEBUG = os.environ.get('DEBUG', 'false')
CONFORMANCE_YAML = ("tests/kubernetes_conformance/resources/k8s_ymls/"
                    "sonobuoy-conformance.yaml")


class KubectlClient(object):

    def __init__(self):
        self._kube_config_path = None
        self._hide = False if DEBUG.lower() == 'true' else True

    @property
    def kube_config_path(self):
        return self._kube_config_path

    @kube_config_path.setter
    def kube_config_path(self, value):
        self._kube_config_path = value

    @staticmethod
    def _load_json(output):
        if output == '':
            return None
        return json.loads(output)

    def _default_output_json(self, **cli_options):
        """
            Adds --output=json to options
            Does not override if output is passed in!
        """
        if 'output' not in list(cli_options.keys()):
            cli_options['output'] = 'json'
        return cli_options

    def _cli_options(self, **kwargs):
        """
            Pass through any kubectl option
            A couple of exceptions for the keyword args mapping to the
            cli options names:
            1) if option flag has a '-', replace with '_'
            e.i. '--all-namespaces' can be passed in all_namespaces=True
            2) reserved words:
                For cli option: 'as' => 'as_user'
        """
        command_options = ""
        for k, v in kwargs.items():
            # Do not include values that are none
            if v is None:
                continue
            # reserved word
            k = 'as' if k == 'as_user' else k
            # k = 'all' if k == 'all_' else k
            if v is False or v is True:
                value = str(v).lower()
            else:
                value = v
            command_options += " --{}={}".format(k.replace('_', '-'), value)
        return command_options

    def execute_kubectl_cmd(self, cmd, json_out=True):
        command = '/usr/local/bin/kubectl  --kubeconfig {0} {1}'.format(
            self.kube_config_path, cmd)
        if json_out:
            command += ' -o json'
        print("Running kubectl command: {}".format(command))
        start_time = time.time()
        result = self.run_command(command)
        end_time = time.time()
        print('Run time for command {0}: {1} seconds'.format(
            command, end_time - start_time))
        return result

    def execute_kubectl(self, cmd, **cli_options):
        # always add kubeconfig
        cli_options['kubeconfig'] = self.kube_config_path
        command = 'kubectl {0}{1}'.format(
            cmd, self._cli_options(**cli_options))
        print("Running kubectl command: {}".format(command))
        start_time = time.time()
        result = self.run_command_with_stderr(command)
        end_time = time.time()
        print('Run time for command {0}: {1} seconds'.format(
            command, end_time - start_time))
        return result

    def exec_cmd(self, pod, cmd, namespace):
        result = self.execute_kubectl_cmd(
            'exec {0} --namespace={1} -- {2}'.format(pod, namespace, cmd),
            json_out=False)
        return result

    def logs(self, pod='', **cli_options):
        command = 'logs {0}'.format(pod) if pod else "logs"
        result = self.execute_kubectl(command, **cli_options)
        return result

    def cp_from_pod(self, pod, namespace, path_in_pod, local_path):
        command = "cp {}/{}:{} {}".format(
            namespace, pod, path_in_pod, local_path)
        return self.execute_kubectl(command)

    def list_namespaces(self):
        ns = self.get_resource("namespace")
        return [n['metadata']['name'] for n in ns['items']]

    def get_nodes(self):
        nodes = self.get_resource("nodes")
        return nodes

    def create_ns(self, namespace):
        self.create_resource("namespace", namespace)
        # Verify namespace is created
        ns = self.get_resource("namespace", name=namespace)
        assert ns["metadata"]["name"] == namespace
        assert ns["status"]["phase"] == "Active"
        return ns

    def run(self, name, **cli_options):
        command = "run {0}".format(name)
        result = self.execute_kubectl(command, **cli_options)
        return result

    def create_resourse_from_yml(self, file_yml, namespace=None):
        cmd = "create -f {0}".format(file_yml)
        if namespace:
            cmd += ' --namespace={0}'.format(namespace)
        return self.execute_kubectl_cmd(cmd)

    def delete_resourse_from_yml(self, file_yml, namespace=None):
        cmd = "delete -f {0}".format(file_yml)
        if namespace:
            cmd += ' --namespace={0}'.format(namespace)
        return self.execute_kubectl_cmd(cmd, json_out=False)

    def create_resource(self, resource, name=None, **cli_options):
        cli_options = self._default_output_json(**cli_options)
        command = "create {0}".format(resource)
        if name:
            command += ' {0}'.format(name)
        result = self.execute_kubectl(command, **cli_options)
        return self._load_json(result)

    def get_resource(self, resource, name=None, **cli_options):
        cli_options = self._default_output_json(**cli_options)
        command = "get {0}".format(resource)
        if name:
            command += ' {0}'.format(name)
        result = self.execute_kubectl(command, **cli_options)
        return self._load_json(result)

    def delete_resourse(self, resource, name=None, **cli_options):
        command = "delete {0}".format(resource)
        if name:
            command += ' {0}'.format(name)
        return self.execute_kubectl(command, **cli_options)

    def wait_for_pods(self, number_of_pods=1, state='Running', **cli_options):
        start_time = int(time.time())
        while True:
            pods = self.get_resource('pods', **cli_options)
            print("pods:")
            print(pods)
            print (len(pods['items']))
            if len(pods['items']) == number_of_pods:
                running_pods = 0
                for pod in pods['items']:
                    print (pod['status']['phase'])
                    if pod['status']['phase'] != state:
                        print("Pod '{0}' not {1} is {2}!".format(
                            pod['metadata']['name'], state,
                            pod['status']['phase']))
                        break
                    else:
                        running_pods += 1
                if running_pods == number_of_pods:
                    return pods
            if int(time.time()) - start_time > 300:
                pod_states = {}
                for p in pods.get('items', []):
                    pod_states[p['metadata']['name']] = p['status']['phase']
                raise Exception(
                    'Timeout Exception: pods did not start\n'
                    'Expect number of pods {0} vs number of pods found {1}:\n'
                    'Pod states: {2}'.format(
                        number_of_pods, len(pod_states), pod_states))
            time.sleep(5)

    def wait_for_pod(self, name, state='Running', **cli_options):
        """
            If a pod name is known, wait for pod to start
        """
        start_time = int(time.time())
        while True:
            pod = self.get_resource('pod', name=name, **cli_options)
            if pod['status']['phase'] != state:
                print("Pod '{0}' not {1} is {2}!".format(
                    pod['metadata']['name'], state, pod['status']['phase']))
            else:
                time.sleep(15)
                return pod
            if int(time.time()) - start_time > 300:
                raise Exception(
                    'Timeout Exception: pod {} did not start\n'.format(name))
            time.sleep(5)

    def apply_conformance_tests(self):
        command = "apply -f {0}".format(CONFORMANCE_YAML)
        result = self.execute_kubectl_cmd(command)
        assert result.ok, (
            "Failed to apply sonobuoy-conformance.yaml.\nCommand: '{0}'\n"
            "stdout: {1}\nstderr:{2}\n".format(
                command, result.stdout, result.stderr))
        return result

    def run_command(self, command):
        return subprocess.check_output(command, shell=True, text=True)

    def run_command_with_stderr(self, command):
        try:
            return subprocess.check_output(command, shell=True,
                                             stderr=subprocess.PIPE)
        except subprocess.CalledProcessError as e:
            print(e.output)
            print(e.stderr)
            output = e.output
            returncode = e.returncode
        print(returncode)
