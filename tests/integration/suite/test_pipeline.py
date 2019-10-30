import pytest
import time
from .pipeline_common import MockGithub
from .conftest import ProjectContext, rancher

MOCK_GITHUB_PORT = 4016
MOCK_GITHUB_HOST = "localhost:4016"
MOCK_GITHUB_REPO_URL = 'https://github.com/octocat/Hello-World.git'
MOCK_GITHUB_USER = 'octocat'
GITHUB_TYPE = 'github'


@pytest.fixture(scope="module")
def mock_github():
    server = MockGithub(port=MOCK_GITHUB_PORT)
    server.start()
    yield server
    server.shutdown_server()


@pytest.mark.nonparallel
def test_pipeline_set_up_github(admin_pc, mock_github):
    client = admin_pc.client
    set_up_pipeline_github(admin_pc)

    configs = client.list_source_code_provider_config()
    gh = None
    for c in configs:
        if c.type == "githubPipelineConfig":
            gh = c
    assert gh is not None
    assert gh.enabled is True
    assert gh.disable

    providers = client.list_source_code_provider()
    assert len(providers) == 1
    gh_provider = providers.data[0]
    assert gh_provider.type == 'githubProvider'
    assert gh_provider.login

    creds = client.list_source_code_credential()
    assert len(creds) == 1
    assert creds.data[0].sourceCodeType == GITHUB_TYPE
    assert creds.data[0].loginName == MOCK_GITHUB_USER

    repos = client.list_source_code_repository()
    assert len(repos) == 1
    assert repos.data[0].sourceCodeType == GITHUB_TYPE
    assert repos.data[0].url == MOCK_GITHUB_REPO_URL


@pytest.mark.nonparallel
def test_pipeline_set_up_github_with_custom_role(admin_mc,
                                                 admin_pc,
                                                 mock_github,
                                                 user_factory,
                                                 remove_resource):
    # Create a new user with custom global role
    user = user_factory(globalRoleId="user-base")
    remove_resource(user)

    # Preference creation triggers user ns creation
    user.client.create_preference(name="language", value="\"en-us\"")
    client = admin_mc.client
    project = admin_pc.project

    # Add this user as project-owner
    prtb_owner = client.create_project_role_template_binding(
        projectId=project.id,
        roleTemplateId="project-owner",
        userId=user.user.id)
    remove_resource(prtb_owner)

    url = project.links.self + '/schemas'
    user_pc = ProjectContext(None, project,
                             rancher.Client(url=url,
                                            verify=False,
                                            token=user.client.token))
    set_up_pipeline_github(user_pc)
    user_client = user_pc.client
    creds = user_client.list_source_code_credential()
    assert len(creds) == 1
    assert creds.data[0].sourceCodeType == GITHUB_TYPE
    assert creds.data[0].loginName == MOCK_GITHUB_USER

    repos = user_client.list_source_code_repository()
    assert len(repos) == 1
    assert repos.data[0].sourceCodeType == GITHUB_TYPE
    assert repos.data[0].url == MOCK_GITHUB_REPO_URL


@pytest.mark.nonparallel
def test_pipeline_disable_github(admin_pc, mock_github):
    client = admin_pc.client
    set_up_pipeline_github(admin_pc)

    configs = client.list_source_code_provider_config()
    gh = None
    for c in configs:
        if c.type == "githubPipelineConfig":
            gh = c
    assert gh is not None
    assert gh.enabled is True
    assert gh.disable

    gh.disable()

    providers = client.list_source_code_provider()
    assert len(providers) == 0


@pytest.mark.nonparallel
def test_pipeline_github_log_in_out(admin_pc, mock_github):
    client = admin_pc.client
    set_up_pipeline_github(admin_pc)

    providers = client.list_source_code_provider()
    gh_provider = providers.data[0]

    creds = client.list_source_code_credential()
    creds.data[0].refreshrepos()

    repos = client.list_source_code_repository()
    assert len(repos) == 1

    repos_by_cred = creds.data[0].repos()
    assert len(repos_by_cred) == 1

    creds.data[0].logout_action()
    creds = client.list_source_code_credential()
    assert len(creds) == 0

    gh_provider.login(code='test_code')
    creds = client.list_source_code_credential()
    assert len(creds) == 1


def set_up_pipeline_github(user_pc):
    gh = get_source_code_provider_config(user_pc, "githubPipelineConfig")
    assert gh is not None

    gh.testAndApply(code="test_code",
                    hostname=MOCK_GITHUB_HOST,
                    tls=False,
                    clientId="test_id",
                    clientSecret="test_secret")


def get_source_code_provider_config(user_pc, config_type):
    client = user_pc.client
    start_time = int(time.time())
    while int(time.time()) - start_time < 30:
        configs = client.list_source_code_provider_config()
        for c in configs:
            if c.type == config_type:
                return c
        time.sleep(3)
    raise Exception('Timeout getting {0}'.format(config_type))
