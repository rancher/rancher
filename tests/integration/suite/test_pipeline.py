import pytest
from rancher import ApiError
from .pipeline_common import MockGithub
from .conftest import ProjectContext, rancher, \
    wait_until_available, user_project_client
from .common import random_str


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
    client = user_pc.client
    configs = client.list_source_code_provider_config()
    gh = None
    for c in configs:
        if c.type == "githubPipelineConfig":
            gh = c
    assert gh is not None

    gh.testAndApply(code="test_code",
                    hostname=MOCK_GITHUB_HOST,
                    tls=False,
                    clientId="test_id",
                    clientSecret="test_secret")


def test_pipeline_run_access(admin_mc, admin_pc, user_mc, remove_resource):
    """Tests that a user with read-only access is not
    able to run a pipeline.
    """
    prtb = admin_mc.client.create_project_role_template_binding(
        name="prtb-" + random_str(),
        userId=user_mc.user.id,
        projectId=admin_pc.project.id,
        roleTemplateId="read-only")
    remove_resource(prtb)

    pipeline = admin_pc.client.create_pipeline(
        projectId=admin_pc.project.id,
        repositoryUrl="https://github.com/rancher/pipeline-example-go.git",
        name=random_str(),
    )
    remove_resource(pipeline)
    wait_until_available(admin_pc.client, pipeline)

    # ensure user can get pipeline
    proj_user_client = user_project_client(user_mc, admin_pc.project)
    wait_until_available(proj_user_client, pipeline)
    with pytest.raises(ApiError) as e:
        # Doing run action with pipeline obj from admin_client should fail
        user_mc.client.action(obj=pipeline, action_name="run", branch="master")
    assert e.value.error.status == 404
