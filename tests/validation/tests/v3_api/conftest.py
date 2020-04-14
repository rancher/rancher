import urllib3
from .common import *  # NOQA

# This stops ssl warnings for insecure certs
urllib3.disable_warnings()


def pytest_configure(config):
    if TEST_RBAC and CATTLE_TEST_URL:
        rbac_prepare()
    if AUTH_PROVIDER != "":
        prepare_auth_data()


def pytest_unconfigure(config):
    if TEST_RBAC and CATTLE_TEST_URL:
        rbac_cleanup()


@pytest.fixture
def remove_resource(request):
    """Remove a resource after a test finishes even if the test fails.

    How to use:
      pass this function as an argument of your testing function,
      then call this function with the new resource as argument after
      creating any new resource

    """

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
