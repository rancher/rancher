from .common import *   # NOQA

import requests

AUTH_PROVIDER = os.environ.get('RANCHER_AUTH_PROVIDER', "")

'''
Prerequisite:
Enable AD without TLS, and using testuser1 as admin user.

Description:
In this test, we are testing the customized user and group search filter
functionalities.
1) For customized user search filter:
The filter looks like:
(&(objectClass=person)(|(sAMAccountName=test*)(sn=test*)(givenName=test*))
[user customized filter])
Here, after we add
userSearchFilter = (memberOf=CN=testgroup5,CN=Users,DC=tad,DC=rancher,DC=io)
we will filter out only testuser40 and testuser41, otherwise, all users start
with search keyword "testuser" will be listed out.

2) For customized group search filter:
The filter looks like:
(&(objectClass=group)(sAMAccountName=test)[group customized filter])
Here, after we add groupSearchFilter = (cn=testgroup2)
we will filter out only testgroup2, otherwise, all groups has search
keyword "testgroup" will be listed out.
'''

# Config Fields
HOSTNAME_OR_IP_ADDRESS = os.environ.get("RANCHER_HOSTNAME_OR_IP_ADDRESS")
PORT = os.environ.get("RANCHER_PORT")
CONNECTION_TIMEOUT = os.environ.get("RANCHER_CONNECTION_TIMEOUT")
SERVICE_ACCOUNT_NAME = os.environ.get("RANCHER_SERVICE_ACCOUNT_NAME")
SERVICE_ACCOUNT_PASSWORD = os.environ.get("RANCHER_SERVICE_ACCOUNT_PASSWORD")
DEFAULT_LOGIN_DOMAIN = os.environ.get("RANCHER_DEFAULT_LOGIN_DOMAIN")
USER_SEARCH_BASE = os.environ.get("RANCHER_USER_SEARCH_BASE")
GROUP_SEARCH_BASE = os.environ.get("RANCHER_GROUP_SEARCH_BASE")
PASSWORD = os.environ.get('RANCHER_USER_PASSWORD', "")

CATTLE_AUTH_URL = \
    CATTLE_TEST_URL + \
    "/v3-public/"+AUTH_PROVIDER+"Providers/" + \
    AUTH_PROVIDER.lower()+"?action=login"

CATTLE_AUTH_PROVIDER_URL = \
    CATTLE_TEST_URL + "/v3/"+AUTH_PROVIDER+"Configs/"+AUTH_PROVIDER.lower()

CATTLE_AUTH_PRINCIPAL_URL = CATTLE_TEST_URL + "/v3/principals?action=search"

CATTLE_AUTH_ENABLE_URL = CATTLE_AUTH_PROVIDER_URL + "?action=testAndApply"

CATTLE_AUTH_DISABLE_URL = CATTLE_AUTH_PROVIDER_URL + "?action=disable"


def test_custom_user_and_group_filter_for_AD():
    disable_ad("testuser1", ADMIN_TOKEN)
    enable_ad_with_customized_filter(
        "testuser1",
        "(memberOf=CN=testgroup5,CN=Users,DC=tad,DC=rancher,DC=io)",
        "", ADMIN_TOKEN)
    search_ad_users("testuser", ADMIN_TOKEN)

    disable_ad("testuser1", ADMIN_TOKEN)
    enable_ad_with_customized_filter(
        "testuser1", "", "(cn=testgroup2)", ADMIN_TOKEN)
    search_ad_groups("testgroup", ADMIN_TOKEN)


def disable_ad(username, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_DISABLE_URL, json={
        "enabled": False,
        "username": username,
        "password": PASSWORD
    }, verify=False, headers=headers)
    assert r.status_code == expected_status
    print("Disable ActiveDirectory request for " +
          username + " " + str(expected_status))


def enable_ad_with_customized_filter(username, usersearchfilter,
                                     groupsearchfilter, token,
                                     expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    activeDirectoryConfig = {
        "accessMode": "unrestricted",
        "userSearchFilter": usersearchfilter,
        "groupSearchFilter": groupsearchfilter,
        "connectionTimeout": CONNECTION_TIMEOUT,
        "defaultLoginDomain": DEFAULT_LOGIN_DOMAIN,
        "groupDNAttribute": "distinguishedName",
        "groupMemberMappingAttribute": "member",
        "groupMemberUserAttribute": "distinguishedName",
        "groupNameAttribute": "name",
        "groupObjectClass": "group",
        "groupSearchAttribute": "sAMAccountName",
        "nestedGroupMembershipEnabled": False,
        "port": PORT,
        "servers": [
            HOSTNAME_OR_IP_ADDRESS
        ],
        "serviceAccountUsername": SERVICE_ACCOUNT_NAME,
        "userDisabledBitMask": 2,
        "userEnabledAttribute": "userAccountControl",
        "userLoginAttribute": "sAMAccountName",
        "userNameAttribute": "name",
        "userObjectClass": "person",
        "userSearchAttribute": "sAMAccountName|sn|givenName",
        "userSearchBase": USER_SEARCH_BASE,
        "serviceAccountPassword": SERVICE_ACCOUNT_PASSWORD
    }

    r = requests.post(CATTLE_AUTH_ENABLE_URL, json={
        "activeDirectoryConfig": activeDirectoryConfig,
        "enabled": True,
        "username": username,
        "password": PASSWORD
    }, verify=False, headers=headers)
    assert r.status_code == expected_status
    print("Enable ActiveDirectory request for " +
          username + " " + str(expected_status))


def search_ad_users(searchkey, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_PRINCIPAL_URL,
                      json={'name': searchkey, 'principalType': 'user',
                            'responseType': 'json'},
                      verify=False, headers=headers)
    assert r.status_code == expected_status

    if r.status_code == 200:
        print(r.json())
        data = r.json()['data']
        print(data)
        assert len(data) == 2
        print(data)
        assert \
            data[0].get('id') == \
            "activedirectory_user://CN=test user40," \
            "CN=Users,DC=tad,DC=rancher,DC=io"
        assert \
            data[1].get('id') == \
            "activedirectory_user://CN=test user41," \
            "CN=Users,DC=tad,DC=rancher,DC=io"


def search_ad_groups(searchkey, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_PRINCIPAL_URL,
                      json={'name': searchkey, 'principalType': 'group',
                            'responseType': 'json'},
                      verify=False, headers=headers)
    assert r.status_code == expected_status

    if r.status_code == 200:
        data = r.json()['data']
        assert len(data) == 1
        assert \
            data[0].get('id') == \
            "activedirectory_group://CN=testgroup2," \
            "CN=Users,DC=tad,DC=rancher,DC=io"
