import pytest
import errno
import warnings
from distutils.spawn import find_executable
from .common import * # NOQA

setup = {"p_client": None, "project": None,
         "admin_client": None, "server_url": None,
         "server_host": None}

DISTRIBUTION_HEADER = "docker-distribution-api-version"
REGISTRY_TEMP_ID = "catalog://?catalog=system-library&template=harbor" \
                   "&version=1.7.5-rancher1"


def test_global_registry_enable():
    admin_client = setup["admin_client"]
    p_client = setup["p_client"]
    p = setup["project"]

    setting = admin_client.by_id_setting(id="global-registry-enabled")
    assert setting is not None, \
        "Expected to find global-registry-enabled setting"

    p_client.create_app(
        name="global-registry",
        externalId=REGISTRY_TEMP_ID,
        targetNamespace="cattle-system",
        projectId=p.id,
        answers={
            "expose.ingress.host": setup["server_host"],
            "externalURL": setup["server_url"],
            "persistence.enabled": "false",
            "clair.enabled": "false",
            "notary.enabled": "false"
        },
    )
    admin_client.update_by_id_setting(id="global-registry-enabled",
                                      value="true")
    wait_for_app_to_active(p_client, "global-registry", timeout=360)
    check_global_registry()


@pytest.fixture(scope='module', autouse="True")
def create_clients():
    admin_client = get_admin_client()
    setup["admin_client"] = admin_client

    local_cluster = admin_client.by_id_cluster("local")
    if local_cluster is None:
        pytest.skip("skipping HA-only tests", allow_module_level=True)

    projects = admin_client.list_project(
        clusterId="local")
    sys_p = None
    for project in projects:
        if project['name'] == "System":
            sys_p = project
            break
    assert sys_p is not None
    p_client = get_project_client_for_token(sys_p, ADMIN_TOKEN)
    setup["project"] = sys_p
    setup["p_client"] = p_client

    serverURLSetting = admin_client.by_id_setting(id="server-url")
    assert serverURLSetting is not None
    setup["server_url"] = serverURLSetting.value
    setup["server_host"] = setup["server_url"].strip("https://")


def check_global_registry():
    response = requests.get(setup["server_url"] + "/registry/",
                            verify=False)
    assert response.status_code == 200
    if docker_exist() and prepare_ca_crt():
        server_host = setup["server_host"]
        command = '''
        set -e;
        docker login -u admin -p Harbor12345 %s
        docker pull nginx
        docker tag nginx %s/library/nginx
        docker push %s/library/nginx
        ''' % (server_host, server_host, server_host)
        proc = subprocess.run(command, shell=True, universal_newlines=True)
        assert proc.returncode == 0, proc.stderr
    else:
        response = requests.get(setup["server_url"] + "/v2/",
                                verify=False)
        assert response.status_code == 401
        assert response.headers[DISTRIBUTION_HEADER] == 'registry/2.0'


def docker_exist():
    return find_executable("docker") is not None


def prepare_ca_crt():
    admin_client = setup["admin_client"]
    setting = admin_client.by_id_setting(id="cacerts")

    dir = "/etc/docker/certs.d/%s" % (setup["server_host"])
    certpath = dir + "/ca.crt"
    try:
        os.makedirs(dir)
        f = open(certpath, "w")
        f.write(setting.value)
        f.close()
        os.chmod(certpath, 0o600)
    except IOError as e:
        if (e.errno == errno.EACCES):
            warnings.warn("no permission to prepare cacert")
            return False
    return True


def teardown_module(module):
    admin_client = setup["admin_client"]
    p_client = setup["p_client"]
    app_data = p_client.list_app(name="global-registry").data
    if len(app_data) > 0:
        app = app_data[0]
        p_client.delete(app)
    admin_client.update_by_id_setting(id="global-registry-enabled",
                                      value="false")
