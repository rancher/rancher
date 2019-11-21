import urllib3
from .common import *

# This stops ssl warnings for insecure certs
urllib3.disable_warnings()


def pytest_configure(config):
    if TEST_RBAC is False:
        return
    rbac_prepare()


def pytest_unconfigure(config):
    if TEST_RBAC is False:
        return
    rbac_cleanup()


@pytest.fixture
def remove_resource(request):
    """Remove a resource after a test finishes even if the test fails."""
    client = get_admin_client()

    def _cleanup(resource):
        def clean():
            try:
                client.delete(resource)
            except ApiError as e:
                code = e.error.status
                if code == 409 and "namespace will automatically be purged " \
                        in e.error.message:
                    pass
                elif code != 404:
                    raise e
        request.addfinalizer(clean)
    return _cleanup
