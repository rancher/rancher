import pytest
from .common import * # NOQA
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC

setup = {"p_client": None, "project": None,
         "sp_client": None, "system_project": None,
         "admin_client": None, "ns": None,
         "server_url": None, "server_host": None}

DISTRIBUTION_HEADER = "docker-distribution-api-version"
REGISTRY_TEMP_ID = "catalog://?catalog=system-library&template=harbor" \
                   "&version=1.7.5-rancher1"
TEST_USER = "testuser"
TEST_PASSWORD = "TestPassword000"


@pytest.mark.dependency()
def test_global_registry_enable():
    admin_client = setup["admin_client"]
    sp_client = setup["sp_client"]
    sys_p = setup["system_project"]

    setting = admin_client.by_id_setting(id="global-registry-enabled")
    assert setting is not None, \
        "Expected to find global-registry-enabled setting"

    sp_client.create_app(
        name="global-registry",
        externalId=REGISTRY_TEMP_ID,
        targetNamespace="cattle-system",
        projectId=sys_p.id,
        answers={
            "clair.enabled": "true",
            "clair.resources.limits.cpu": "500m",
            "clair.resources.limits.memory": "2048Mi",
            "clair.resources.requests.cpu": "100m",
            "clair.resources.requests.memory": "256Mi",
            "database.internal.resources.limits.cpu": "500m",
            "database.internal.resources.limits.memory": "2048Mi",
            "database.internal.resources.requests.cpu": "100m",
            "database.internal.resources.requests.memory": "256Mi",
            "database.type": "internal",
            "harborAdminPassword": "Harbor12345",
            "imageChartStorage.type": "filesystem",
            "notary.enabled": "true",
            "notary.server.resources.limits.cpu": "500m",
            "notary.server.resources.limits.memory": "2048Mi",
            "notary.server.resources.requests.cpu": "100m",
            "notary.server.resources.requests.memory": "256Mi",
            "notary.signer.resources.limits.cpu": "500m",
            "notary.signer.resources.limits.memory": "2048Mi",
            "notary.signer.resources.requests.cpu": "100m",
            "notary.signer.resources.requests.memory": "256Mi",
            "persistence.persistentVolumeClaim.database.size": "5Gi",
            "persistence.persistentVolumeClaim.redis.size": "5Gi",
            "persistence.persistentVolumeClaim.registry.size": "100Gi",
            "persistence.type": "storageClass",
            "redis.internal.resources.limits.cpu": "500m",
            "redis.internal.resources.limits.memory": "2048Mi",
            "redis.internal.resources.requests.cpu": "100m",
            "redis.internal.resources.requests.memory": "256Mi",
            "redis.type": "internal",
            "registry.registry.resources.limits.cpu": "1000m",
            "registry.registry.resources.limits.memory": "2048Mi",
            "registry.registry.resources.requests.cpu": "100m",
            "registry.registry.resources.requests.memory": "256Mi",
            "secretKey": "add-test-secret0",
            "expose.ingress.host": setup["server_host"],
            "externalURL": setup["server_url"],
        },
    )
    admin_client.update_by_id_setting(id="global-registry-enabled",
                                      value="true")
    time.sleep(10)
    wait_for_app_to_active(sp_client, "global-registry", timeout=360)


@pytest.mark.dependency(depends=['test_global_registry_enable'])
def test_access_harbor_ui():
    chrome_options = webdriver.ChromeOptions()
    chrome_options.add_argument('--headless')
    chrome_options.add_argument('--disable-gpu')
    chrome_options.add_argument('--no-sandbox')
    driver = webdriver.Chrome(options=chrome_options)
    driver.implicitly_wait(15)
    driver.get(setup["server_url"] + "/registry")
    WebDriverWait(driver, 15).until(
        EC.presence_of_element_located((By.ID,"login_username"))
    )
    elem = driver.find_element_by_id("login_username")
    elem.clear()
    elem.send_keys("admin")
    elem = driver.find_element_by_id("login_password")
    elem.send_keys("Harbor12345")
    elem.send_keys(Keys.RETURN)

    # Create a Harbor project
    WebDriverWait(driver, 15).until(
        EC.presence_of_element_located((By.CSS_SELECTOR, 'button.btn-secondary'))
    )
    driver.find_element_by_css_selector('button.btn-secondary').click()
    driver.find_element_by_id("create_project_name").send_keys("test-project")
    driver.find_element_by_css_selector('button.btn-primary').click()
    WebDriverWait(driver, 15).until(
        EC.invisibility_of_element_located(
            (By.CSS_SELECTOR, 'div.modal-content'))
    )
    # Create a Harbor user
    driver.find_element_by_partial_link_text('Users').click()
    driver.find_element_by_css_selector('button.btn-secondary').click()
    driver.find_element_by_id("username").send_keys(TEST_USER)
    driver.find_element_by_id("email").send_keys("test@example.com")
    driver.find_element_by_id("realname").send_keys("Test User")
    driver.find_element_by_id("newPassword").send_keys(TEST_PASSWORD)
    driver.find_element_by_id("confirmPassword").send_keys(TEST_PASSWORD)
    driver.find_element_by_css_selector('button.btn-primary').click()
    WebDriverWait(driver, 15).until(
        EC.invisibility_of_element_located(
            (By.CSS_SELECTOR, 'div.modal-content'))
    )
    # Add the user to the project
    driver.find_element_by_partial_link_text('Projects').click()
    driver.find_element_by_link_text("test-project").click()
    driver.find_element_by_link_text("Members").click()
    driver.find_element_by_css_selector('button.btn-secondary').click()
    driver.find_element_by_id("member_name").send_keys(TEST_USER)
    driver.find_element_by_css_selector('button.btn-primary').click()

    driver.close()


@pytest.mark.dependency(depends=['test_access_harbor_ui'])
def test_push_to_global_registry():
    response = requests.get(setup["server_url"] + "/registry/",
                            verify=False)
    assert response.status_code == 200

    prepare_ca_crt()

    server_host = setup["server_host"]
    command = '''
    set -e;
    docker login -u %s -p %s %s
    docker pull nginx
    docker tag nginx %s/test-project/nginx:test
    docker push %s/test-project/nginx:test
    ''' % (TEST_USER, TEST_PASSWORD, server_host, server_host, server_host)
    proc = subprocess.run(command, shell=True, universal_newlines=True)
    assert proc.returncode == 0, proc.stderr


@pytest.mark.dependency(depends=['test_push_to_global_registry'])
def test_use_global_registry_in_project():
    p_client = setup["p_client"]
    server_host = setup["server_host"]
    ns = setup["ns"]
    name = random_test_name("registry")
    registries = {setup["server_host"]: {"username": TEST_USER,
                             "password": TEST_PASSWORD}}
    p_client.create_dockerCredential(
            registries=registries, name=name)

    workload_name = random_test_name("testwk")
    con = [{"name": "test",
            "image": "%s/test-project/nginx:test" % (server_host),
            "runAsNonRoot": False,
            "stdin": True,
            "imagePullPolicy": "Always"
            }]
    workload = p_client.create_workload(name=workload_name,
                                        containers=con,
                                        namespaceId=ns.id)
    time.sleep(5)
    validate_workload(p_client, workload, "deployment", ns.name)


def prepare_ca_crt():
    admin_client = setup["admin_client"]
    setting = admin_client.by_id_setting(id="cacerts")

    dir = "/etc/docker/certs.d/%s" % (setup["server_host"])
    certpath = dir + "/ca.crt"
    os.makedirs(dir)
    f = open(certpath, "w")
    f.write(setting.value)
    f.close()
    os.chmod(certpath, 0o600)


@pytest.fixture(scope='module', autouse="True")
def create_clients(request):
    admin_client, cluster = get_admin_client_and_cluster()
    setup["admin_client"] = admin_client
    create_kubeconfig(cluster)

    local_cluster = admin_client.by_id_cluster("local")
    if local_cluster is None:
        pytest.skip("Global Registry tests only run in HA setup",
                    allow_module_level=True)
    c_client = get_cluster_client_for_token(cluster, ADMIN_TOKEN)
    scs = c_client.list_storage_class()
    if len(scs) == 0:
        pytest.skip("Global Registry tests require at least "
                    "a default storage class",
                    allow_module_level=True)
    projects = admin_client.list_project(
        clusterId="local")
    sys_p = None
    for project in projects:
        if project['name'] == "System":
            sys_p = project
            break
    assert sys_p is not None
    sp_client = get_project_client_for_token(sys_p, ADMIN_TOKEN)
    setup["system_project"] = sys_p
    setup["sp_client"] = sp_client

    serverURLSetting = admin_client.by_id_setting(id="server-url")
    assert serverURLSetting is not None
    setup["server_url"] = serverURLSetting.value
    setup["server_host"] = setup["server_url"].strip("https://")

    p, ns = create_project_and_ns(
        ADMIN_TOKEN, cluster, random_test_name("testglobalregistry"))
    p_client = get_project_client_for_token(p, ADMIN_TOKEN)
    setup["p_client"] = p_client
    setup["ns"] = ns
    setup["project"] = p

    def fin():
        p_client.delete(ns)
        admin_client.delete(setup["project"])

    request.addfinalizer(fin)


def teardown_module():
    admin_client = setup["admin_client"]
    sp_client = setup["sp_client"]
    app_data = sp_client.list_app(name="global-registry").data
    if len(app_data) > 0:
        app = app_data[0]
        sp_client.delete(app)
    admin_client.update_by_id_setting(id="global-registry-enabled",
                                      value="false")
    pvcs = sp_client.list_persistent_volume_claim().data
    for pvc in pvcs:
        if "global-registry" in pvc['name']:
            sp_client.delete(pvc)
