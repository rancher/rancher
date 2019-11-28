from .common import *   # NOQA
from rancher import ApiError

import pytest
import rancher
import requests

'''
Prerequisite:
Enable AD with TLS, and using testuser1 as admin user.

Description:
This test asserts that a user who is not assigned to an admin globarole,
but is in a group that is assigned to an admin global role, has admin has
admin privileges.

Steps:
1. login to user A who is in group B
2. attempt to create a resources that requires admin privileges
    a. assert Forbidden error
3. create a globalrolebinding that assigns group B to the admin
globalrole.
    a. assert user A can now create a resource that requires admin
    privileges
4. delete globalrolebinding created in step 3.
5. attempt to create a resource that requires admin privileges
    a. assert Forbidden error
'''

AUTH_PROVIDER = os.environ.get('RANCHER_AUTH_PROVIDER', "")

# username of auth user that only has user privileges
RANCHER_AUTH_USERNAME = os.environ.get('RANCHER_AUTH_USERNAME', "")
# password for auth user
RANCHER_AUTH_PASSWORD = os.environ.get('RANCHER_AUTH_PASSWORD', "")
# name of auth group that auth user is a member of
RANCHER_AUTH_GROUP = os.environ.get('RANCHER_AUTH_GROUP', "")

CATTLE_AUTH_URL = \
    CATTLE_TEST_URL + \
    "/v3-public/"+AUTH_PROVIDER+"Providers/" + \
    AUTH_PROVIDER.lower()+"?action=login"

CATTLE_AUTH_PROVIDER_URL = \
    CATTLE_TEST_URL + "/v3/"+AUTH_PROVIDER+"Configs/"+AUTH_PROVIDER.lower()

CATTLE_AUTH_PRINCIPAL_URL = CATTLE_TEST_URL + "/v3/principals?action=search"


def test_group_grbs():
    groups = search_ad_groups(RANCHER_AUTH_GROUP, ADMIN_TOKEN)
    admin_client = get_admin_client()

    r = requests.post(CATTLE_AUTH_URL, json={
        'username': RANCHER_AUTH_USERNAME,
        'password': RANCHER_AUTH_PASSWORD,
        'responseType': 'json',
    }, verify=False)

    token = r.json()["token"]

    testuser3_client = rancher.Client(
        url=CATTLE_TEST_URL + "/v3",
        token=token,
        verify=False)

    with pytest.raises(ApiError) as e:
        rt = testuser3_client.create_role_template(name="rt-"+random_str())

    assert e.value.error.status == 403
    assert e.value.error.code == 'Forbidden'

    gr = admin_client.create_global_role_binding(
        globalRoleId="admin",
        groupPrincipalId=groups[0]["id"])

    def try_create_role_template():
        try:
            return testuser3_client.create_role_template(name="rt-" + random_str())
        except ApiError as e:
            assert e.error.status == 403
            assert e.value.error.code == 'Forbidden'
            return False

    rt = wait_for(try_create_role_template)
    admin_client.delete(gr)

    # once user is no longer admin, they will be unable to see local cluster
    # this is less wasteful than attemptingg to create roletemplates until
    # unauthorized error is encountered
    wait_for(lambda: len(testuser3_client.list_cluster()) == 0)

    try:
        testuser3_client.create_role_template(name="rt-" + random_str())
    except ApiError as e:
        assert e.error.status == 403
        assert e.error.code == 'Forbidden'


def search_ad_groups(searchkey, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_PRINCIPAL_URL,
                      json={'name': searchkey, 'principalType': 'group',
                            'responseType': 'json'},
                      verify=False, headers=headers)
    assert r.status_code == expected_status

    if r.status_code == 200:
        data = r.json()['data']
        return data
