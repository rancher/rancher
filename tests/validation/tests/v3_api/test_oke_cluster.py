import pytest
from .common import *  # NOQA

OCI_TENANCY_OCID = os.environ.get('RANCHER_OCI_TENANCY_OCID', "")
OCI_COMPARTMENT_OCID = os.environ.get('RANCHER_OCI_COMPARTMENT_OCID', "")
OCI_USER_OCID = os.environ.get('RANCHER_OCI_USER_OCID', "")
OCI_FINGERPRINT = os.environ.get('RANCHER_OCI_FINGERPRINT', "")
OCI_PRIVATE_KEY_PATH = os.environ.get('RANCHER_OCI_PRIVATE_KEY_PATH', "")
OCI_PRIVATE_KEY_PASSPHRASE = os.environ.get('RANCHER_OCI_PRIVATE_KEY_PASSPHRASE', "")
OCI_REGION = os.environ.get('RANCHER_OCI_REGION', None)
OKE_VERSION = os.environ.get('RANCHER_OKE_VERSION', None)
OKE_NODE_SHAPE = os.environ.get('RANCHER_OKE_NODE_SHAPE', None)
OKE_NODE_IMAGE = os.environ.get('RANCHER_OKE_NODE_IMAGE', None)

okecredential = pytest.mark.skipif(not (
        OCI_TENANCY_OCID and OCI_COMPARTMENT_OCID and OCI_USER_OCID and OCI_FINGERPRINT and OCI_PRIVATE_KEY_PATH),
                                   reason='OKE Credentials not provided, '
                                          'cannot create cluster')


def test_error_get_oke_latest_versions_missing_key():
    oci_cred_body = {
        "tenancyOCID": "ocid1.tenancy.oc1..aaaaaa",
        "userOCID": "ocid1.user.oc1..aaaaaaaa",
        "region": "us-ashburn-1",
        "fingerprint": "xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx",
    }
    response = get_oci_meta_response("/meta/oci/okeVersions", oci_cred_body)
    assert response.content is not None
    json_response = json.loads(response.content)
    print(json_response)
    assert response.status_code == 422
    assert json_response['message'] == "OCI API private key is required"


def test_error_get_oke_images_missing_tenancy():
    oci_cred_body = {
        "userOCID": "ocid1.user.oc1..aaaaaaaa",
        "region": "us-phoenix-1",
        "fingerprint": "xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx",
        "privateKey": "-----BEGIN RSA PRIVATE KEY-----\nMIIE...ewBQ==\n-----END RSA PRIVATE KEY-----"
    }

    response = get_oci_meta_response("/meta/oci/nodeOkeImages", oci_cred_body)
    assert response.content is not None
    json_response = json.loads(response.content)
    print(json_response)
    assert response.status_code == 422
    assert json_response['message'] == "OCI tenancy is required"


def test_error_get_invalid_endpoint():
    oci_cred_body = {
        "tenancyOCID": "ocid1.tenancy.oc1..aaaaaa",
        "userOCID": "ocid1.user.oc1..aaaaaaaa",
        "region": "ap-tokyo-1",
        "fingerprint": "xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx",
        "privateKey": "-----BEGIN RSA PRIVATE KEY-----\nMIIE...ewBQ==\n-----END RSA PRIVATE KEY-----"
    }
    response = get_oci_meta_response("/meta/oci/dne", oci_cred_body)
    assert response.status_code == 404
    assert response.content is not None
    json_response = json.loads(response.content)
    print(json_response)
    assert "invalid endpoint" in json_response['message']


@okecredential
def test_get_oke_latest_versions():
    oci_cred_body = {
        "tenancyOCID": OCI_TENANCY_OCID,
        "userOCID": OCI_USER_OCID,
        "region": OCI_REGION,
        "fingerprint": OCI_FINGERPRINT,
        "privateKey": get_ssh_key_contents(OCI_PRIVATE_KEY_PATH)
    }
    response = get_oci_meta_response("/meta/oci/okeVersions", oci_cred_body)

    assert response.status_code == 200
    assert response.content is not None
    json_response = json.loads(response.content)
    latest_oke_version = json_response[-1]

    print(latest_oke_version)


@okecredential
def test_create_oke_cluster():
    client, cluster = create_and_validate_oke_cluster("test-auto-oke")
    cluster_cleanup(client, cluster)


def create_and_validate_oke_cluster(name):
    oci_cred_body = {
        "tenancyOCID": OCI_TENANCY_OCID,
        "userOCID": OCI_USER_OCID,
        "region": OCI_REGION,
        "fingerprint": OCI_FINGERPRINT,
        "privateKey": get_ssh_key_contents(OCI_PRIVATE_KEY_PATH)
    }
    client = get_user_client()
    print("Cluster creation")
    # Get the region
    if OCI_REGION is None:
        region = "us-phoenix-1"
    else:
        region = OCI_REGION
    # Get the node shape
    if OKE_NODE_SHAPE is None:
        response = get_oci_meta_response("/meta/oci/nodeShapes", oci_cred_body)
        print(response.content)
        assert response.status_code == 200
        assert response.content is not None
        json_response = json.loads(response.content)
        print(json_response)
        shape = json_response[-1]
    else:
        shape = OKE_NODE_SHAPE

    # Get the node image
    if OKE_NODE_IMAGE is None:
        response = get_oci_meta_response("/meta/oci/nodeOkeImages", oci_cred_body)
        assert response.status_code == 200
        assert response.content is not None
        json_response = json.loads(response.content)
        print(json_response)
        latest_oke_image = json_response[-1]
    else:
        latest_oke_image = OKE_NODE_IMAGE

    # Get the OKE version
    if OKE_VERSION is None:
        response = get_oci_meta_response("/meta/oci/okeVersions", oci_cred_body)
        assert response.status_code == 200
        assert response.content is not None
        json_response = json.loads(response.content)
        print(json_response)
        latest_oke_version = json_response[-1]
    else:
        latest_oke_version = OKE_VERSION

    cluster = client.create_cluster(
        name=name,
        okeEngineConfig={
            "availabilityDomain": "",
            "compartmentId": OCI_COMPARTMENT_OCID,
            "displayName": name,
            "driverName": "oraclecontainerengine",
            "enableKubernetesDashboard": False,
            "enablePrivateNodes": False,
            "enableTiller": False,
            "fingerprint": OCI_FINGERPRINT,
            "kubernetesVersion": latest_oke_version,
            "loadBalancerSubnetName1": "",
            "loadBalancerSubnetName2": "",
            "name": name,
            "nodeImage": latest_oke_image,
            "nodePoolDnsDomainName": "nodedns",
            "nodePoolSecurityListName": "Node Security List",
            "nodePoolSubnetName": "nodedns",
            "nodePublicKeyContents": "",
            "nodePublicKeyPath": "",
            "nodeShape": shape,
            "privateKeyContents": get_ssh_key_contents(OCI_PRIVATE_KEY_PATH),
            "privateKeyPassphrase": OCI_PRIVATE_KEY_PASSPHRASE,
            "privateKeyPath": "",
            "quantityOfNodeSubnets": 1,
            "quantityPerSubnet": 1,
            "region": region,
            "serviceDnsDomainName": "svcdns",
            "serviceSecurityListName": "Service Security List",
            "skipVcnDelete": False,
            "tenancyId": OCI_TENANCY_OCID,
            "tenancyName": "",
            "type": "okeEngineConfig",
            "userOcid": OCI_USER_OCID,
            "vcnName": "",
            "waitNodesActive": 0,
            "workerNodeIngressCidr": ""
        })
    print(cluster)
    cluster = validate_cluster(client, cluster, check_intermediate_state=True,
                               skipIngresscheck=True)
    return client, cluster


def get_oci_meta_response(endpoint, oci_cred_body):
    headers = {"Content-Type": "application/json",
               "Accept": "application/json",
               "Authorization": "Bearer " + USER_TOKEN}

    oke_version_url = CATTLE_TEST_URL + endpoint

    response = requests.post(oke_version_url, json=oci_cred_body,
                             verify=False, headers=headers)

    return response


def get_ssh_key_contents(path):
    if os.path.exists(path):
        with open(path, 'r') as f:
            ssh_key = f.read()
        return ssh_key
