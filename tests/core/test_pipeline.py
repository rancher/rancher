import pytest
import time
from .mockserver import MockGithub, MockGitlab
from rancher import ApiError

MOCK_GITHUB_USER = 'octocat'
MOCK_GITLAB_USER = 'john_smith'
MOCK_GITHUB_REPO_URL = 'http://localhost:4016/octocat/Hello-World.git'
MOCK_GITLAB_REPO_URL = 'http://localhost:4017/brightbox/puppet.git'
EXAMPLE_REPO_URL = 'https://github.com/rancher/pipeline-example-php.git'
GITHUB_TYPE = 'github'
GITLAB_TYPE = 'gitlab'


@pytest.fixture(scope="module")
def mock_github():
    server = MockGithub()
    server.start()
    yield server
    server.shutdown_server()


@pytest.fixture(scope="module")
def mock_gitlab():
    server = MockGitlab()
    server.start()
    yield server
    server.shutdown_server()


@pytest.fixture
def remove_pipeline_namespace(admin_cc, request):
    def fin(project_id):
        client = admin_cc.client
        splits = project_id.split(':')
        print(splits)
        ns = client.by_id_namespace(splits[1] + '-pipeline')
        client.delete(ns)

    def _cleanup(project_id):
        request.addfinalizer(lambda: fin(project_id))

    return _cleanup


@pytest.mark.nonparallel
def test_pipeline_source_code_provider_configs(admin_pc):
    client = admin_pc.client
    wait_for_config_create(client)

    configs = client.list_source_code_provider_config()
    assert configs.pagination.total == 2

    gh = None
    gl = None

    for c in configs:
        if c.type == "githubPipelineConfig":
            gh = c
        elif c.type == "gitlabPipelineConfig":
            gl = c

    for x in [gh, gl]:
        assert x is not None
        config = client.by_id_source_code_provider_config(x.id)
        with pytest.raises(ApiError) as e:
            client.delete(config)
        assert e.value.error.status == 405

    assert gh.actions.testAndApply
    assert gl.actions.testAndApply


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
def test_pipeline_set_up_gitlab(admin_pc, mock_gitlab):
    client = admin_pc.client
    set_up_pipeline_gitlab(admin_pc)

    configs = client.list_source_code_provider_config()
    gl = None
    for c in configs:
        if c.type == "gitlabPipelineConfig":
            gl = c
    assert gl is not None
    assert gl.enabled is True
    assert gl.disable

    providers = client.list_source_code_provider()
    assert len(providers) == 1
    gl_provider = providers.data[0]
    assert gl_provider.type == 'gitlabProvider'
    assert gl_provider.login

    creds = client.list_source_code_credential()
    assert len(creds) == 1
    assert creds.data[0].sourceCodeType == GITLAB_TYPE
    assert creds.data[0].loginName == MOCK_GITLAB_USER

    repos = client.list_source_code_repository()
    assert len(repos) == 1
    assert repos.data[0].sourceCodeType == GITLAB_TYPE
    assert repos.data[0].url == MOCK_GITLAB_REPO_URL


@pytest.mark.nonparallel
def test_pipeline_disable_gitlab(admin_pc, mock_gitlab):
    client = admin_pc.client
    set_up_pipeline_gitlab(admin_pc)

    configs = client.list_source_code_provider_config()
    gl = None
    for c in configs:
        if c.type == "gitlabPipelineConfig":
            gl = c
    assert gl is not None
    assert gl.enabled is True
    assert gl.disable

    gl.disable()

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


@pytest.mark.nonparallel
def test_pipeline_gitlab_log_in_out(admin_pc, mock_gitlab):
    client = admin_pc.client
    set_up_pipeline_gitlab(admin_pc)

    providers = client.list_source_code_provider()
    gl_provider = providers.data[0]

    creds = client.list_source_code_credential()
    creds.data[0].refreshrepos()

    repos = client.list_source_code_repository()
    assert len(repos) == 1

    repos_by_cred = creds.data[0].repos()
    assert len(repos_by_cred) == 1

    creds.data[0].logout_action()
    creds = client.list_source_code_credential()
    assert len(creds) == 0

    gl_provider.login(code='test_code')
    creds = client.list_source_code_credential()
    assert len(creds) == 1


@pytest.mark.nonparallel
def test_pipeline_crd(admin_pc, mock_github):
    client = admin_pc.client

    set_up_pipeline_github(admin_pc)

    repos = client.list_source_code_repository()
    creds = client.list_source_code_credential()

    client.create_pipeline(repositoryUrl=EXAMPLE_REPO_URL,
                           projectId=admin_pc.project.id)
    client.create_pipeline(repositoryUrl=repos.data[0].url,
                           projectId=admin_pc.project.id,
                           sourceCodeCredentialId=creds.data[0].id)

    pipelines = client.list_pipeline()

    assert len(pipelines) == 2

    for p in pipelines:
        client.delete(p)


@pytest.mark.nonparallel
def test_pipeline_links(admin_pc, mock_github):
    client = admin_pc.client

    set_up_pipeline_github(admin_pc)

    repos = client.list_source_code_repository()
    creds = client.list_source_code_credential()

    client.create_pipeline(repositoryUrl=repos.data[0].url,
                           projectId=admin_pc.project.id,
                           sourceCodeCredentialId=creds.data[0].id)

    pipelines = client.list_pipeline()
    pipeline = pipelines.data[0]

    result = pipeline.yaml()
    assert result.master

    result = pipeline.configs()
    assert result.master

    result = pipeline.branches()
    assert 'master' in result


@pytest.mark.nonparallel
def test_pipeline_links_gitlab(admin_pc, mock_gitlab):
    client = admin_pc.client

    set_up_pipeline_gitlab(admin_pc)

    repos = client.list_source_code_repository()
    creds = client.list_source_code_credential()

    client.create_pipeline(repositoryUrl=repos.data[0].url,
                           projectId=admin_pc.project.id,
                           sourceCodeCredentialId=creds.data[0].id)

    pipelines = client.list_pipeline()
    pipeline = pipelines.data[0]

    result = pipeline.yaml()
    assert result.master

    result = pipeline.configs()
    assert result.master

    result = pipeline.branches()
    assert 'master' in result


@pytest.mark.nonparallel
def test_pipeline_run(admin_pc, mock_github, remove_pipeline_namespace):
    client = admin_pc.client

    set_up_pipeline_github(admin_pc)

    repos = client.list_source_code_repository()
    creds = client.list_source_code_credential()
    pipeline = client.create_pipeline(repositoryUrl=repos.data[0].url,
                                      projectId=admin_pc.project.id,
                                      sourceCodeCredentialId=creds.data[0].id)

    execution = pipeline.run(branch='master')
    remove_pipeline_namespace(admin_pc.project.id)
    executions = client.list_pipeline_execution()

    assert len(executions) == 1
    assert execution.run == 1


def set_up_pipeline_github(admin_pc):
    client = admin_pc.client
    wait_for_config_create(client)
    configs = client.list_source_code_provider_config()
    gh = None
    for c in configs:
        if c.type == "githubPipelineConfig":
            gh = c
    assert gh is not None

    config = {}
    config["hostname"] = "localhost:4016"
    config["tls"] = False
    config["clientId"] = "test_id"
    config["clientSecret"] = "test_secret"
    gh.testAndApply(code="test_code", githubConfig=config)


def set_up_pipeline_gitlab(admin_pc):
    client = admin_pc.client
    wait_for_config_create(client)
    configs = client.list_source_code_provider_config()
    gl = None
    for c in configs:
        if c.type == "gitlabPipelineConfig":
            gl = c
    assert gl is not None

    config = {}
    config["hostname"] = "localhost:4017"
    config["tls"] = False
    config["clientId"] = "test_id"
    config["clientSecret"] = "test_secret"
    gl.testAndApply(code="test_code", gitlabConfig=config)


def wait_for_config_create(client, timeout=5):
    start = time.time()
    configs = client.list_source_code_provider_config()
    while len(configs) == 0:
        time.sleep(.5)
        configs = client.list_source_code_provider_config()
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for provider config creation')


def wait_for_pipeline_execution_state(client, execution_id, timeout=600,
                                      state='Success'):
    start = time.time()
    execution = client.by_id_pipeline_execution(execution_id)
    while not hasattr(execution, 'ended') or execution.ended == "":
        time.sleep(10)
        execution = client.by_id_pipeline_execution(execution_id)
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for execution finish')

    assert execution.executionState == state
