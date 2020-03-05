import pytest
import os
import re
import time
from rancher import ApiError

from .common import (
    DEFAULT_TIMEOUT,
    get_user_client,
    random_name,
)
from .test_rke_cluster_provisioning import (
    AZURE_SUBSCRIPTION_ID,
    AZURE_CLIENT_ID,
    AZURE_CLIENT_SECRET,
    validate_rke_dm_host_1,
    engine_install_url
)


# NOTE(aiyengar2): due to constraints from Azure, the images
# must be in the same region (i.e. location) as you are attempting
# to deploy to. However, you should be able to deploy any image
# in the same region, even if that image is owned by a different
# subscription or part of a different resource group.
DEFAULT_AZURE_IMAGE = "canonical:UbuntuServer:16.04.0-LTS:latest"
AZURE_LOCATION = "westus"
AZURE_CUSTOM_IMAGE = os.environ.get("AZURE_CUSTOM_IMAGE")
AZURE_GALLERY_IMAGE_VERSION = os.environ.get("AZURE_GALLERY_IMAGE_VERSION")


def validate_ci(arm_identifier):
    """
    Validates that the provided string has the information necessary for the
    ARM resource identifer for an Azure custom image

    Expected:
    subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s
    """
    if not arm_identifier:
        return False
    subscriptions_re = re.compile(
        r'[sS]ubscriptions/[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}')
    resource_group_re = re.compile(
        r'[rR]esource[gG]roups/[-\w\._\(\)]+')
    image_re = re.compile(
        r'[iI]mages/[A-Za-z][A-Za-z0-9-_]{1,61}[A-Za-z0-9_]')
    return all([
        subscriptions_re.search(arm_identifier),
        resource_group_re.search(arm_identifier),
        image_re.search(arm_identifier)
    ])


def validate_giv(arm_identifier):
    """
    Validates that the provided string has the information necessary for the
    ARM resource identifer for an Azure Shared Image Gallery Version

    Expected:
    subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/versions/%s
    """
    if not arm_identifier:
        return False
    gallery_re = re.compile(
        r'[gG]alleries/[A-Za-z0-9][A-Za-z0-9_.]{1,61}[A-Za-z0-9]')
    version_re = re.compile(
        r'[vV]ersions/[0-9]*\.[0-9]*\.[0-9]*')
    return all([
        validate_ci(arm_identifier),
        gallery_re.search(arm_identifier),
        version_re.search(arm_identifier)
    ])


@pytest.mark.skipif(not validate_ci(AZURE_CUSTOM_IMAGE),
                    reason="AZURE_CUSTOM_IMAGE missing fields")
@pytest.mark.skipif(not AZURE_CUSTOM_IMAGE,
                    reason="AZURE_CUSTOM_IMAGE not provided")
def test_provision_custom_image(client, node_template_az_custom_image):
    """
    Provisions an Azure VM with a custom image by providing the custom
    image ARM resource Identifier
    """
    validate_rke_dm_host_1(node_template_az_custom_image)


@pytest.mark.skipif(not validate_giv(AZURE_GALLERY_IMAGE_VERSION),
                    reason="AZURE_GALLERY_IMAGE_VERSION missing fields")
@pytest.mark.skipif(not AZURE_GALLERY_IMAGE_VERSION,
                    reason="AZURE_GALLERY_IMAGE_VERSION not provided")
def test_provision_gallery_image(client, node_template_az_gallery_image):
    """
    Provisions an Azure VM with a custom image from a Azure Shared Image
    Gallery by providing the custom gallery image's ARM resource Identifier
    """
    validate_rke_dm_host_1(node_template_az_gallery_image)


# TODO: a test that tries to deploy an Azure resource in a
# different resource group (in the same region)


# TODO: a test that tries to deploy an Azure resource in a
# different subscription (in the same region)


@pytest.fixture('module')
def client():
    """
    A user client to be used in tests
    """
    return get_user_client()


@pytest.fixture(scope='module')
def node_template_az_custom_image(client):
    """
    A node template for Azure based on a custom image
    """
    node_template, az_cloud_credential = setup_node_template_and_cloud_creds(
        client, AZURE_CUSTOM_IMAGE)
    yield node_template
    tear_down_node_template_and_cloud_creds(
        client, node_template, az_cloud_credential)


@pytest.fixture(scope='module')
def node_template_az_gallery_image(client):
    """
    A node template for Azure based on an Azure Shared Gallery image
    """
    node_template, az_cloud_credential = setup_node_template_and_cloud_creds(
        client, AZURE_GALLERY_IMAGE_VERSION)
    yield node_template
    tear_down_node_template_and_cloud_creds(
        client, node_template, az_cloud_credential)


def setup_node_template_and_cloud_creds(client, image=DEFAULT_AZURE_IMAGE):
    """
    Sets up an Azure node template based on the image provided
    """
    az_cloud_credential_config = {"clientId": AZURE_CLIENT_ID,
                                  "subscriptionId": AZURE_SUBSCRIPTION_ID,
                                  "clientSecret": AZURE_CLIENT_SECRET}
    az_cloud_credential = client.create_cloud_credential(
        azurecredentialConfig=az_cloud_credential_config
    )
    # NOTE(aiyengar2): Azure constraints on custom images
    # - custom images must use managedDisks
    # - availability set must also be a managed
    azureConfig = {
        "availabilitySet": "custom-images-avset",
        "customData": "",
        "dns": "",
        "dockerPort": "2376",
        "environment": "AzurePublicCloud",
        "image": image,
        "location": AZURE_LOCATION,
        "managedDisks": True,
        "noPublicIp": False,
        "openPort": [
            "6443/tcp",
            "2379/tcp",
            "2380/tcp",
            "8472/udp",
            "4789/udp",
            "10256/tcp",
            "10250/tcp",
            "10251/tcp",
            "10252/tcp",
            "80/tcp",
            "443/tcp",
            "9999/tcp",
            "8888/tcp",
            "30456/tcp",
            "30457/tcp",
            "30458/tcp",
            "30459/tcp",
            "9001/tcp"
        ],
        "privateIpAddress": "",
        "resourceGroup": "docker-machine",
        "size": "Standard_A2",
        "sshUser": "docker-user",
        "staticPublicIp": False,
        "storageType": "Standard_LRS",
        "subnet": "docker-machine",
        "subnetPrefix": "192.168.0.0/16",
        "usePrivateIp": False,
        "vnet": "docker-machine-vnet"
    }

    node_template = client.create_node_template(
        azureConfig=azureConfig,
        name=random_name(),
        useInternalIpAddress=False,
        driver="azure",
        engineInstallURL=engine_install_url,
        cloudCredentialId=az_cloud_credential.id
    )
    node_template = client.wait_success(node_template)
    return node_template, az_cloud_credential


def tear_down_node_template_and_cloud_creds(client, node_template, creds):
    """
    Tear down logic for a created node template and cloud credential,
    to be called after waiting for the cluster to finish deleting
    """
    def _attempt_delete_node_template(client, node_template,
                                      timeout=DEFAULT_TIMEOUT,
                                      sleep_time=.5):
        start = time.time()
        while node_template:
            if time.time() - start > timeout:
                raise AssertionError(
                    "Timed out waiting for node template %s to get deleted"
                    % node_template["name"])
            time.sleep(sleep_time)
            client.reload(node_template)
            try:
                client.delete(node_template)
                break
            except ApiError:
                pass
            except Exception as e:
                raise e

    def _attempt_delete_cloud_credential(client, creds,
                                         timeout=DEFAULT_TIMEOUT,
                                         sleep_time=.5):
        start = time.time()
        while creds:
            if time.time() - start > timeout:
                raise AssertionError(
                    "Timed out waiting for cloud credential %s to get deleted"
                    % creds["name"])
            time.sleep(sleep_time)
            client.reload(creds)
            try:
                client.delete(creds)
                break
            except ApiError:
                pass
            except Exception as e:
                raise e
    _attempt_delete_node_template(client, node_template)
    _attempt_delete_cloud_credential(client, creds)
