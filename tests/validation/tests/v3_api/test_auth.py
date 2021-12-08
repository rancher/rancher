import json
import os
import pytest
import requests

from .common import ADMIN_TOKEN
from .common import AUTH_PROVIDER
from .common import AUTH_USER_PASSWORD
from .common import CATTLE_TEST_URL
from .common import cluster_cleanup
from .common import create_project_and_ns
from .common import get_admin_client
from .common import get_client_for_token
from .test_rke_cluster_provisioning import create_and_validate_custom_host

'''
Prerequisite:
1. testautoadmin as your admin user, if the fixture detects the auth
   is disabled it will be enabled automatically.
2. Two clusters in your setup, if none or one are detected by the fixture
   will create clusters to match two
'''


# Config Fields
HOSTNAME_OR_IP_ADDRESS = os.environ.get("RANCHER_HOSTNAME_OR_IP_ADDRESS")
PORT = os.environ.get("RANCHER_PORT", "")
CA_CERTIFICATE = os.environ.get("RANCHER_CA_CERTIFICATE", "")
OPENLDAP_CA_CERTIFICATE = os.environ.get("RANCHER_OPENLDAP_CA_CERTIFICATE", "")
FREEIPA_CA_CERTIFICATE = os.environ.get("RANCHER_FREEIPA_CA_CERTIFICATE", "")
CONNECTION_TIMEOUT = os.environ.get("RANCHER_CONNECTION_TIMEOUT", 5000)
SERVICE_ACCOUNT_NAME = os.environ.get("RANCHER_SERVICE_ACCOUNT_NAME")
SERVICE_ACCOUNT_PASSWORD = os.environ.get("RANCHER_SERVICE_ACCOUNT_PASSWORD")
DEFAULT_LOGIN_DOMAIN = os.environ.get("RANCHER_DEFAULT_LOGIN_DOMAIN")
USER_SEARCH_BASE = os.environ.get("RANCHER_USER_SEARCH_BASE")
GROUP_SEARCH_BASE = os.environ.get("RANCHER_GROUP_SEARCH_BASE")
AD_SPECIAL_CHAR_PASSWORD = os.environ.get("RANCHER_AD_SPECIAL_CHAR_PASSWORD")
OPENLDAP_SPECIAL_CHAR_PASSWORD = \
    os.environ.get("RANCHER_OPENLDAP_SPECIAL_CHAR_PASSWORD")
FREEIPA_SPECIAL_CHAR_PASSWORD = \
    os.environ.get("RANCHER_FREEIPA_SPECIAL_CHAR_PASSWORD")
OPENLDAP_HOSTNAME_OR_IP_ADDRESS = \
    os.environ.get("RANCHER_OPENLDAP_HOSTNAME_OR_IP_ADDRESS")
OPENLDAP_SERVICE_ACCOUNT_NAME = \
    os.environ.get("RANCHER_OPENLDAP_SERVICE_ACCOUNT_NAME")
OPENLDAP_SERVICE_ACCOUNT_PASSWORD = \
    os.environ.get("RANCHER_OPENLDAP_SERVICE_ACCOUNT_PASSWORD")
OPENLDAP_USER_SEARCH_BASE = os.environ.get("RANCHER_OPENLDAP_USER_SEARCH_BASE")
OPENLDAP_AUTH_USER_PASSWORD = \
    os.environ.get("RANCHER_OPENLDAP_AUTH_USER_PASSWORD")

FREEIPA_HOSTNAME_OR_IP_ADDRESS = \
    os.environ.get("RANCHER_FREEIPA_HOSTNAME_OR_IP_ADDRESS")
FREEIPA_SERVICE_ACCOUNT_NAME = \
    os.environ.get("RANCHER_FREEIPA_SERVICE_ACCOUNT_NAME")
FREEIPA_SERVICE_ACCOUNT_PASSWORD = \
    os.environ.get("RANCHER_FREEIPA_SERVICE_ACCOUNT_PASSWORD")
FREEIPA_USER_SEARCH_BASE = os.environ.get("RANCHER_FREEIPA_USER_SEARCH_BASE")
FREEIPA_GROUP_SEARCH_BASE = os.environ.get("RANCHER_FREEIPA_GROUP_SEARCH_BASE")

FREEIPA_AUTH_USER_PASSWORD = \
    os.environ.get("RANCHER_FREEIPA_AUTH_USER_PASSWORD")

PASSWORD = ""

if AUTH_PROVIDER == "activeDirectory":
    PASSWORD = AUTH_USER_PASSWORD
elif AUTH_PROVIDER == "openLdap":
    PASSWORD = OPENLDAP_AUTH_USER_PASSWORD
elif AUTH_PROVIDER == "freeIpa":
    PASSWORD = FREEIPA_AUTH_USER_PASSWORD

CATTLE_AUTH_URL = \
    CATTLE_TEST_URL + \
    "/v3-public/" + AUTH_PROVIDER + "Providers/" + \
    AUTH_PROVIDER.lower() + "?action=login"

CATTLE_AUTH_PROVIDER_URL = \
    CATTLE_TEST_URL + "/v3/" + AUTH_PROVIDER + "Configs/" +\
    AUTH_PROVIDER.lower()

CATTLE_AUTH_PRINCIPAL_URL = CATTLE_TEST_URL + "/v3/principals?action=search"

CATTLE_AUTH_ENABLE_URL = CATTLE_AUTH_PROVIDER_URL + "?action=testAndApply"

CATTLE_AUTH_DISABLE_URL = CATTLE_AUTH_PROVIDER_URL + "?action=disable"

setup = {"cluster1": None,
         "project1": None,
         "ns1": None,
         "cluster2": None,
         "project2": None,
         "ns2": None,
         "auth_setup_data": {},
         "permission_denied_code": 403}

auth_setup_fname = \
    os.path.join(os.path.dirname(os.path.realpath(__file__)) + "/resource",
                 AUTH_PROVIDER.lower() + ".json")


def test_access_control_required_set_access_mode_required():
    access_mode = "required"
    validate_access_control_set_access_mode(access_mode)


def test_access_control_restricted_set_access_mode_required():
    access_mode = "restricted"
    validate_access_control_set_access_mode(access_mode)


def test_access_control_required_add_users_and_groups_to_cluster():
    access_mode = "required"
    validate_add_users_and_groups_to_cluster_or_project(
        access_mode, add_users_to_cluster=True)


def test_access_control_restricted_add_users_and_groups_to_cluster():
    access_mode = "restricted"
    validate_add_users_and_groups_to_cluster_or_project(
        access_mode, add_users_to_cluster=True)


def test_access_control_required_add_users_and_groups_to_project():
    access_mode = "required"
    validate_add_users_and_groups_to_cluster_or_project(
        access_mode, add_users_to_cluster=False)


def test_access_control_restricted_add_users_and_groups_to_project():
    access_mode = "restricted"
    validate_add_users_and_groups_to_cluster_or_project(
        access_mode, add_users_to_cluster=False)


def test_disable_and_enable_auth_set_access_control_required():
    access_mode = "required"
    validate_access_control_disable_and_enable_auth(access_mode)


def test_disable_and_enable_auth_set_access_control_restricted():
    access_mode = "restricted"
    validate_access_control_disable_and_enable_auth(access_mode)


# By default nestedgroup is disabled for ad and openldap, enabled for freeipa
def test_disable_and_enable_nestedgroups_set_access_control_required():
    access_mode = "required"
    validate_access_control_disable_and_enable_nestedgroups(access_mode)


def test_disable_and_enable_nestedgroup_set_access_control_restricted():
    access_mode = "restricted"
    validate_access_control_disable_and_enable_nestedgroups(access_mode)


def test_ad_service_account_login():
    delete_project_users()
    delete_cluster_users()
    auth_setup_data = setup["auth_setup_data"]
    admin_user = auth_setup_data["admin_user"]
    # admin_user here is the AD admin user
    if AUTH_PROVIDER == "activeDirectory":
        admin_token = login(admin_user, AUTH_USER_PASSWORD)
        disable_ad(admin_user, admin_token)
        enable_ad(admin_user, admin_token)
        login(SERVICE_ACCOUNT_NAME, SERVICE_ACCOUNT_PASSWORD)


def test_special_character_users_login_access_mode_required():
    access_mode = "required"
    special_character_users_login(access_mode)


def test_special_character_users_login_access_mode_restricted():
    access_mode = "restricted"
    special_character_users_login(access_mode)


def special_character_users_login(access_mode):
    delete_project_users()
    delete_cluster_users()
    auth_setup_data = setup["auth_setup_data"]
    admin_user = auth_setup_data["admin_user"]
    admin_token = login(admin_user, PASSWORD)
    allowed_principal_ids = []
    if AUTH_PROVIDER == "activeDirectory":
        disable_ad(admin_user, admin_token)
        enable_ad(admin_user, admin_token)
    if AUTH_PROVIDER == "openLdap":
        disable_openldap(admin_user, admin_token)
        enable_openldap(admin_user, admin_token)
    if AUTH_PROVIDER == "freeIpa":
        disable_freeipa(admin_user, admin_token)
        enable_freeipa(admin_user, admin_token)

    if AUTH_PROVIDER == "activeDirectory":
        for user in auth_setup_data["specialchar_in_username"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for user in auth_setup_data["specialchar_in_password"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for user in auth_setup_data["specialchar_in_userdn"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for group in auth_setup_data["specialchar_in_groupname"]:
            allowed_principal_ids.append(principal_lookup(group, admin_token))

        allowed_principal_ids.append(
            principal_lookup(admin_user, admin_token))
        add_users_to_site_access(
            admin_token, access_mode, allowed_principal_ids)

        for user in auth_setup_data["specialchar_in_username"]:
            login(user, PASSWORD)
        for user in auth_setup_data["specialchar_in_password"]:
            login(user, AD_SPECIAL_CHAR_PASSWORD)
        for user in auth_setup_data["specialchar_in_userdn"]:
            login(user, PASSWORD)
        for group in auth_setup_data["specialchar_in_groupname"]:
            for user in auth_setup_data[group]:
                login(user, PASSWORD)

    if AUTH_PROVIDER == "openLdap":
        for user in auth_setup_data["specialchar_in_user_cn_sn"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for user in auth_setup_data["specialchar_in_uid"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for user in auth_setup_data["specialchar_in_password"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for group in auth_setup_data["specialchar_in_groupname"]:
            allowed_principal_ids.append(principal_lookup(group, admin_token))

        allowed_principal_ids.append(principal_lookup(admin_user, admin_token))
        add_users_to_site_access(
            admin_token, access_mode, allowed_principal_ids)

        for user in auth_setup_data["specialchar_in_user_cn_sn"]:
            login(user, PASSWORD)
        for user in auth_setup_data["specialchar_in_uid"]:
            login(user, PASSWORD)
        for user in auth_setup_data["specialchar_in_password"]:
            login(user, OPENLDAP_SPECIAL_CHAR_PASSWORD)
        for group in auth_setup_data["specialchar_in_groupname"]:
            for user in auth_setup_data[group]:
                login(user, PASSWORD)

    if AUTH_PROVIDER == "freeIpa":
        for user in auth_setup_data["specialchar_in_users"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for user in auth_setup_data["specialchar_in_password"]:
            allowed_principal_ids.append(principal_lookup(user, admin_token))
        for group in auth_setup_data["specialchar_in_groupname"]:
            allowed_principal_ids.append(principal_lookup(group, admin_token))

        allowed_principal_ids.append(
            principal_lookup(admin_user, admin_token))
        add_users_to_site_access(
            admin_token, access_mode, allowed_principal_ids)

        for user in auth_setup_data["specialchar_in_users"]:
            login(user, PASSWORD)
        for user in auth_setup_data["specialchar_in_password"]:
            login(user, FREEIPA_SPECIAL_CHAR_PASSWORD)
        for group in auth_setup_data["specialchar_in_groupname"]:
            for user in auth_setup_data[group]:
                login(user, PASSWORD)


def validate_access_control_set_access_mode(access_mode):

    delete_cluster_users()
    auth_setup_data = setup["auth_setup_data"]
    admin_user = auth_setup_data["admin_user"]
    token = login(admin_user, PASSWORD)
    allowed_principal_ids = []
    for user in auth_setup_data["allowed_users"]:
        allowed_principal_ids.append(principal_lookup(user, token))
    for group in auth_setup_data["allowed_groups"]:
        allowed_principal_ids.append(principal_lookup(group, token))
    allowed_principal_ids.append(principal_lookup(admin_user, token))

    # Add users and groups in allowed list to access rancher-server
    add_users_to_site_access(token, access_mode, allowed_principal_ids)

    for user in auth_setup_data["allowed_users"]:
        login(user, PASSWORD)

    for group in auth_setup_data["allowed_groups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD)

    for user in auth_setup_data["dis_allowed_users"]:
        login(user, PASSWORD,
              expected_status=setup["permission_denied_code"])

    for group in auth_setup_data["dis_allowed_groups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD,
                  expected_status=setup["permission_denied_code"])

    # Add users and groups from dis allowed list to access rancher-server

    for user in auth_setup_data["dis_allowed_users"]:
        allowed_principal_ids.append(principal_lookup(user, token))

    for group in auth_setup_data["dis_allowed_groups"]:
        for user in auth_setup_data[group]:
            allowed_principal_ids.append(principal_lookup(user, token))

    add_users_to_site_access(token, access_mode, allowed_principal_ids)

    for user in auth_setup_data["allowed_users"]:
        login(user, PASSWORD)

    for group in auth_setup_data["allowed_groups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD)

    for user in auth_setup_data["dis_allowed_users"]:
        login(user, PASSWORD)

    for group in auth_setup_data["dis_allowed_groups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD)

    # Remove users and groups from allowed list to access rancher-server
    allowed_principal_ids = [principal_lookup(admin_user, token)]

    for user in auth_setup_data["dis_allowed_users"]:
        allowed_principal_ids.append(principal_lookup(user, token))
    for group in auth_setup_data["dis_allowed_groups"]:
        for user in auth_setup_data[group]:
            allowed_principal_ids.append(principal_lookup(user, token))

    add_users_to_site_access(token, access_mode, allowed_principal_ids)

    for user in auth_setup_data["allowed_users"]:
        login(user, PASSWORD,
              expected_status=setup["permission_denied_code"])

    for group in auth_setup_data["allowed_groups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD,
                  expected_status=setup["permission_denied_code"])

    for user in auth_setup_data["dis_allowed_users"]:
        login(user, PASSWORD)

    for group in auth_setup_data["dis_allowed_groups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD)


def validate_add_users_and_groups_to_cluster_or_project(
        access_mode, add_users_to_cluster=True):
    delete_cluster_users()
    client = get_admin_client()
    for project in client.list_project():
        delete_existing_users_in_project(client, project)
    auth_setup_data = setup["auth_setup_data"]
    admin_user = auth_setup_data["admin_user"]
    token = login(admin_user, PASSWORD)
    allowed_principal_ids = [principal_lookup(admin_user, token)]

    # Add users and groups in allowed list to access rancher-server
    add_users_to_site_access(token, access_mode, allowed_principal_ids)

    if add_users_to_cluster:
        groups_to_check = auth_setup_data["groups_added_to_cluster"]
        users_to_check = auth_setup_data["users_added_to_cluster"]
    else:
        groups_to_check = auth_setup_data["groups_added_to_project"]
        users_to_check = auth_setup_data["users_added_to_project"]
    for group in groups_to_check:
        for user in auth_setup_data[group]:
            login(user, PASSWORD,
                  expected_status=setup["permission_denied_code"])

    for user in users_to_check:
        login(user, PASSWORD,
              expected_status=setup["permission_denied_code"])

    client = get_client_for_token(token)
    for group in groups_to_check:
        if add_users_to_cluster:
            assign_user_to_cluster(client, principal_lookup(group, token),
                                   setup["cluster1"], "cluster-owner")
        else:
            assign_user_to_project(client, principal_lookup(group, token),
                                   setup["project2"], "project-owner")
    for user in users_to_check:
        if add_users_to_cluster:
            assign_user_to_cluster(client, principal_lookup(user, token),
                                   setup["cluster1"], "cluster-owner")
        else:
            assign_user_to_project(client, principal_lookup(user, token),
                                   setup["project2"], "project-owner")
    expected_status = setup["permission_denied_code"]

    if access_mode == "required":
        expected_status = setup["permission_denied_code"]

    if access_mode == "restricted":
        expected_status = 201

    for group in groups_to_check:
        for user in auth_setup_data[group]:
            login(user, PASSWORD, expected_status)

    for user in users_to_check:
        login(user, PASSWORD, expected_status)


def validate_access_control_disable_and_enable_auth(access_mode):

    delete_cluster_users()
    delete_project_users()
    auth_setup_data = setup["auth_setup_data"]

    # Login as admin user to disable auth, should be success, then enable it.
    admin_user = auth_setup_data["admin_user"]
    admin_token = login(admin_user, PASSWORD)
    if AUTH_PROVIDER == "activeDirectory":
        disable_ad(admin_user, admin_token)
        enable_ad(admin_user, admin_token)
    if AUTH_PROVIDER == "openLdap":
        disable_openldap(admin_user, admin_token)
        enable_openldap(admin_user, admin_token)
    if AUTH_PROVIDER == "freeIpa":
        disable_freeipa(admin_user, admin_token)
        enable_freeipa(admin_user, admin_token)

    # Login as users within allowed principal id list, which cannot perform
    # disable action.
    allowed_principal_ids = []
    for user in auth_setup_data["allowed_users"]:
        allowed_principal_ids.append(principal_lookup(user, admin_token))
    allowed_principal_ids.append(principal_lookup(admin_user, admin_token))

    # Add users in allowed list to access rancher-server
    add_users_to_site_access(admin_token, access_mode, allowed_principal_ids)

    for user in auth_setup_data["allowed_users"]:
        token = login(user, PASSWORD)
        if AUTH_PROVIDER == "activeDirectory":
            disable_ad(user, token,
                       expected_status=setup["permission_denied_code"])
            enable_ad(user, token,
                      expected_status=setup["permission_denied_code"])
        if AUTH_PROVIDER == "openLdap":
            disable_openldap(user, token,
                             expected_status=setup["permission_denied_code"])
            enable_openldap(user, token,
                            expected_status=setup["permission_denied_code"])
        if AUTH_PROVIDER == "freeIpa":
            disable_freeipa(user, token,
                            expected_status=setup["permission_denied_code"])
            enable_freeipa(user, token,
                           expected_status=setup["permission_denied_code"])


def validate_access_control_disable_and_enable_nestedgroups(access_mode):
    delete_project_users()
    delete_cluster_users()

    auth_setup_data = setup["auth_setup_data"]
    admin_user = auth_setup_data["admin_user"]
    token = login(admin_user, PASSWORD)
    if AUTH_PROVIDER == "activeDirectory":
        enable_ad(admin_user, token)
    if AUTH_PROVIDER == "openLdap":
        enable_openldap(admin_user, token)
    if AUTH_PROVIDER == "freeIpa":
        enable_freeipa(admin_user, token)

    allowed_principal_ids = []
    for group in auth_setup_data["allowed_nestedgroups"]:
        allowed_principal_ids.append(principal_lookup(group, token))

    allowed_principal_ids.append(principal_lookup(admin_user, token))

    # Add users in allowed list to access rancher-server
    add_users_to_site_access(token, access_mode, allowed_principal_ids)

    for group in auth_setup_data["allowed_nestedgroups"]:
        for user in auth_setup_data[group]:
            login(user, PASSWORD)

    if AUTH_PROVIDER == "freeIpa":
        for user in auth_setup_data["users_under_nestedgroups"]:
            login(user, PASSWORD)

    if AUTH_PROVIDER == "activeDirectory" or AUTH_PROVIDER == "openLdap":
        for user in auth_setup_data["users_under_nestedgroups"]:
            login(user, PASSWORD,
                  expected_status=setup["permission_denied_code"])

        # Enable nestedgroup feature, so users under nestedgroups can login
        # successfully
        if AUTH_PROVIDER == "activeDirectory":
            enable_ad(admin_user, token, nested=True)
        if AUTH_PROVIDER == "openLdap":
            enable_openldap(admin_user, token, nested=True)

        allowed_principal_ids = []
        for group in auth_setup_data["allowed_nestedgroups"]:
            allowed_principal_ids.append(principal_lookup(group, token))
        allowed_principal_ids.append(principal_lookup(admin_user, token))

        # Add users in allowed list to access rancher-server
        add_users_to_site_access(token, access_mode, allowed_principal_ids)

        for group in auth_setup_data["allowed_nestedgroups"]:
            for user in auth_setup_data[group]:
                login(user, PASSWORD)

        for user in auth_setup_data["users_under_nestedgroups"]:
            login(user, PASSWORD)


def login(username, password, expected_status=201):
    token = ""
    r = requests.post(CATTLE_AUTH_URL, json={
        'username': username,
        'password': password,
        'responseType': 'json',
    }, verify=False)
    assert r.status_code == expected_status
    print("Login request for " + username + " " + str(expected_status))
    if expected_status == 201:
        token = r.json()['token']
    return token


def get_tls(certificate):
    if len(certificate) != 0:
        tls = True
    else:
        tls = False
    return tls


def enable_openldap(username, token, enable_url=CATTLE_AUTH_ENABLE_URL,
                    password=PASSWORD, nested=False,
                    expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    ldap_config = {
        "accessMode": "unrestricted",
        "connectionTimeout": CONNECTION_TIMEOUT,
        "certificate": OPENLDAP_CA_CERTIFICATE,
        "groupDNAttribute": "entryDN",
        "groupMemberMappingAttribute": "member",
        "groupMemberUserAttribute": "entryDN",
        "groupNameAttribute": "cn",
        "groupObjectClass": "groupOfNames",
        "groupSearchAttribute": "cn",
        "nestedGroupMembershipEnabled": nested,
        "enabled": True,
        "port": PORT,
        "servers": [OPENLDAP_HOSTNAME_OR_IP_ADDRESS],
        "serviceAccountDistinguishedName": OPENLDAP_SERVICE_ACCOUNT_NAME,
        "tls": get_tls(OPENLDAP_CA_CERTIFICATE),
        "userDisabledBitMask": 0,
        "userLoginAttribute": "uid",
        "userMemberAttribute": "memberOf",
        "userNameAttribute": "cn",
        "userObjectClass": "inetOrgPerson",
        "userSearchAttribute": "uid|sn|givenName",
        "userSearchBase": OPENLDAP_USER_SEARCH_BASE,
        "serviceAccountPassword": OPENLDAP_SERVICE_ACCOUNT_PASSWORD
    }
    ca_cert = ldap_config["certificate"]
    ldap_config["certificate"] = ca_cert.replace('\\n', '\n')

    r = requests.post(enable_url,
                      json={
                          "ldapConfig": ldap_config,
                          "username": username,
                          "password": password},
                      verify=False, headers=headers)
    nested_msg = " nested group " if nested else " "
    print(f"Enable OpenLDAP{nested_msg}request for {username} "
          f"{expected_status}")
    assert r.status_code == expected_status


def disable_openldap(username, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_DISABLE_URL, json={
        'username': username,
        'password': PASSWORD
    }, verify=False, headers=headers)
    assert r.status_code == expected_status
    print("Disable openLdap request for " +
          username + " " + str(expected_status))


def enable_ad(username, token, enable_url=CATTLE_AUTH_ENABLE_URL,
              password=AUTH_USER_PASSWORD, nested=False, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    active_directory_config = {
        "accessMode": "unrestricted",
        "certificate": CA_CERTIFICATE,
        "connectionTimeout": CONNECTION_TIMEOUT,
        "defaultLoginDomain": DEFAULT_LOGIN_DOMAIN,
        "groupDNAttribute": "distinguishedName",
        "groupMemberMappingAttribute": "member",
        "groupMemberUserAttribute": "distinguishedName",
        "groupNameAttribute": "name",
        "groupObjectClass": "group",
        "groupSearchAttribute": "sAMAccountName",
        "nestedGroupMembershipEnabled": nested,
        "port": PORT,
        "servers": [HOSTNAME_OR_IP_ADDRESS],
        "serviceAccountUsername": SERVICE_ACCOUNT_NAME,
        "tls": get_tls(CA_CERTIFICATE),
        "userDisabledBitMask": 2,
        "userEnabledAttribute": "userAccountControl",
        "userLoginAttribute": "sAMAccountName",
        "userNameAttribute": "name",
        "userObjectClass": "person",
        "userSearchAttribute": "sAMAccountName|sn|givenName",
        "userSearchBase": USER_SEARCH_BASE,
        "serviceAccountPassword": SERVICE_ACCOUNT_PASSWORD
    }

    ca_cert = active_directory_config["certificate"]
    active_directory_config["certificate"] = ca_cert.replace('\\n', '\n')

    r = requests.post(enable_url,
                      json={"activeDirectoryConfig": active_directory_config,
                            "enabled": True,
                            "username": username,
                            "password": password},
                      verify=False, headers=headers)
    nested_msg = " nested group " if nested else " "
    print(f"Enable ActiveDirectory{nested_msg}request for {username} "
          f"{expected_status}")
    assert r.status_code == expected_status


def disable_ad(username, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_DISABLE_URL,
                      json={"enabled": False,
                            "username": username,
                            "password": AUTH_USER_PASSWORD
                            },
                      verify=False, headers=headers)
    print("Disable ActiveDirectory request for " +
          username + " " + str(expected_status))
    assert r.status_code == expected_status


def enable_freeipa(username, token, enable_url=CATTLE_AUTH_ENABLE_URL,
                   password=PASSWORD, nested=False,
                   expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(enable_url, json={
        "ldapConfig": {
            "accessMode": "unrestricted",
            "certificate": FREEIPA_CA_CERTIFICATE,
            "connectionTimeout": CONNECTION_TIMEOUT,
            "groupDNAttribute": "entrydn",
            "groupMemberMappingAttribute": "member",
            "groupMemberUserAttribute": "entrydn",
            "groupNameAttribute": "cn",
            "groupObjectClass": "groupofnames",
            "groupSearchAttribute": "cn",
            "groupSearchBase": FREEIPA_GROUP_SEARCH_BASE,
            "enabled": True,
            "nestedGroupMembershipEnabled": nested,
            "port": PORT,
            "servers": [FREEIPA_HOSTNAME_OR_IP_ADDRESS],
            "serviceAccountDistinguishedName": FREEIPA_SERVICE_ACCOUNT_NAME,
            "tls": get_tls(FREEIPA_CA_CERTIFICATE),
            "userDisabledBitMask": 0,
            "userLoginAttribute": "uid",
            "userMemberAttribute": "memberOf",
            "userNameAttribute": "givenName",
            "userObjectClass": "inetorgperson",
            "userSearchAttribute": "uid|sn|givenName",
            "userSearchBase": FREEIPA_USER_SEARCH_BASE,
            "serviceAccountPassword": FREEIPA_SERVICE_ACCOUNT_PASSWORD
        },
        "username": username,
        "password": password
    }, verify=False, headers=headers)

    print("Enable freeIpa request for " +
          username + " " + str(expected_status))
    assert r.status_code == expected_status


def disable_freeipa(username, token, expected_status=200):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_DISABLE_URL, json={
        "enabled": False,
        "username": username,
        "password": AUTH_USER_PASSWORD
    }, verify=False, headers=headers)
    assert r.status_code == expected_status
    print("Disable freeIpa request for " +
          username + " " + str(expected_status))


def principal_lookup(name, token):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.post(CATTLE_AUTH_PRINCIPAL_URL,
                      json={'name': name, 'responseType': 'json'},
                      verify=False, headers=headers)
    assert r.status_code == 200
    principals = r.json()['data']
    for principal in principals:
        if principal['principalType'] == "user":
            if principal['loginName'] == name:
                return principal["id"]
        if principal['principalType'] == "group":
            if principal['name'] == name:
                return principal["id"]
    assert False


def add_users_to_site_access(token, access_mode, allowed_principal_ids):
    headers = {'Authorization': 'Bearer ' + token}
    r = requests.put(CATTLE_AUTH_PROVIDER_URL, json={
        'allowedPrincipalIds': allowed_principal_ids,
        'accessMode': access_mode,
        'responseType': 'json',
    }, verify=False, headers=headers)
    print(r.json())


def assign_user_to_cluster(client, principal_id, cluster, role_template_id):
    crtb = client.create_cluster_role_template_binding(
        clusterId=cluster.id,
        roleTemplateId=role_template_id,
        userPrincipalId=principal_id)
    return crtb


def assign_user_to_project(client, principal_id, project, role_template_id):
    prtb = client.create_project_role_template_binding(
        projectId=project.id,
        roleTemplateId=role_template_id,
        userPrincipalId=principal_id)
    return prtb


def delete_existing_users_in_cluster(client, cluster):
    crtbs = client.list_cluster_role_template_binding(clusterId=cluster.id)
    for crtb in crtbs:
        client.delete(crtb)


def delete_existing_users_in_project(client, project):
    prtbs = client.list_project_role_template_binding(projectId=project.id)
    for prtb in prtbs:
        client.delete(prtb)


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):

    '''
    This method enables auth on the Rancher server setup. Two clusters are
    required in the setup. If two clusters are not present, it will create
    the clusters.
    '''

    if AUTH_PROVIDER not in ("activeDirectory", "openLdap", "freeIpa"):
        assert False, "Auth Provider set is not supported"
    setup["auth_setup_data"] = load_setup_data()
    client = get_admin_client()
    configs = client.list_auth_config(name=AUTH_PROVIDER.lower())
    if not configs.data[0].enabled:
        auth_setup_data = setup["auth_setup_data"]
        admin_user = auth_setup_data["admin_user"]
        if AUTH_PROVIDER == 'activeDirectory':
            enable_ad(admin_user, ADMIN_TOKEN)
        elif AUTH_PROVIDER == 'openLdap':
            enable_openldap(admin_user, ADMIN_TOKEN)
        elif AUTH_PROVIDER == 'freeIpa':
            enable_freeipa(admin_user, ADMIN_TOKEN)

    cluster_total = len(client.list_cluster().data)
    node_roles = [["controlplane", "etcd", "worker"]]
    kargs = {'node_roles': node_roles,
             'random_cluster_name': True,
             'validate': False}
    aws_nodes1 = cluster1 = None
    aws_nodes2 = cluster2 = None
    if cluster_total == 0:
        cluster1, aws_nodes1 = create_and_validate_custom_host(**kargs)
        cluster2, aws_nodes2 = create_and_validate_custom_host(**kargs)
    if cluster_total == 1:
        cluster2, aws_nodes2 = create_and_validate_custom_host(**kargs)

    clusters = client.list_cluster().data
    assert len(clusters) >= 2
    cluster1 = cluster1 if cluster1 else clusters[0]
    for project in client.list_project():
        delete_existing_users_in_project(client, project)
    p1, ns1 = create_project_and_ns(ADMIN_TOKEN, cluster1)
    cluster2 = cluster2 if cluster2 else clusters[1]
    p2, ns2 = create_project_and_ns(ADMIN_TOKEN, cluster2)
    setup["cluster1"] = cluster1
    setup["project1"] = p1
    setup["ns1"] = ns1
    setup["cluster2"] = cluster2
    setup["project2"] = p2
    setup["ns2"] = ns2

    def fin():
        client.delete(setup["project1"])
        client.delete(setup["project2"])
        delete_cluster_users()
        if cluster_total == 0:
            if aws_nodes1 and aws_nodes2:
                cluster_cleanup(client, cluster1, aws_nodes1)
                cluster_cleanup(client, cluster2, aws_nodes2)
        if cluster_total == 1:
            if aws_nodes2:
                cluster_cleanup(client, cluster2, aws_nodes2)
    request.addfinalizer(fin)


def delete_cluster_users():
    delete_existing_users_in_cluster(get_admin_client(), setup["cluster1"])
    delete_existing_users_in_cluster(get_admin_client(), setup["cluster2"])


def delete_project_users():
    delete_existing_users_in_project(get_admin_client(), setup["project1"])
    delete_existing_users_in_project(get_admin_client(), setup["project2"])


def load_setup_data():
    auth_setup_file = open(auth_setup_fname)
    auth_setup_str = auth_setup_file.read()
    auth_setup_data = json.loads(auth_setup_str)
    return auth_setup_data
