import pytest
import os
import time
import json
from .common import USER_TOKEN
from .common import get_cluster_client_for_token_v1
from .common import execute_kubectl_cmd
from .common import get_user_client_and_cluster
from .common import wait_until_app_v2_deployed
from .common import check_v2_app_and_uninstall
from .common import check_v2_app_installed
from .common import random_test_name

CIS_CHART_VERSION = os.environ.get('RANCHER_CIS_CHART_VERSION', "1.0.100")
SCAN_PROFILE = os.environ.get('RANCHER_SCAN_PROFILE', "rke-profile-permissive")
DEFAULT_SCAN_RUN_TIMEOUT = 120
cluster_detail = {"cluster": None}
cis_annotations = \
    {
        "catalog.cattle.io/ui-source-repo": "rancher-charts",
        "catalog.cattle.io/ui-source-repo-type": "cluster"
    }
cis_charts = {
    "values":
        { "global": {"cattle":{"clusterId": None, "clusterName": None}}},
    "version": CIS_CHART_VERSION,
    "projectId": None
}
CHART_NAME = "rancher-cis-benchmark"
cis_scan_report_results = \
    {"rke-cis-1.5-permissive":{"fail": 0, "notApplicable": 34, "pass": 58,
                               "skip": 0, "total": 92},
    "rke-cis-1.5-hardened":{"fail": 0, "notApplicable": 20, "pass": 72,
                               "skip": 0, "total": 92},
     "eks-1.0": {"fail": 10, "notApplicable": 0, "pass": 4,
                                "skip": 0, "total": 14},
     "gke-1.0": {"fail": 44, "notApplicable": 0, "pass": 4,
                                "skip": 0, "total": 48},
     "cis-1.5": {"fail": 90, "notApplicable": 0, "pass": 2,
                     "skip": 0, "total": 92}}
cis_scan_profile_benchmark = {
    "rke-profile-permissive": "rke-cis-1.5-permissive",
    "rke-profile-hardened": "rke-cis-1.5-hardened",
    "eks-profile": "eks-1.0",
    "gke-profile": "gke-1.0",
    "cis-1.5-profile": "cis-1.5"
}

def test_v2_install_cis_benchmark():
    """
    List installed apps
    Check if the app is installed
    If installed, delete the app and the CRDs
    Create namespace
    Install App and the CRDs
    :return:
    """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"],USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    rancher_repo = rancherrepo["data"][0]

    # check if CIS is already installed and uninstall the app
    check_v2_app_and_uninstall(client, CHART_NAME)
    check_v2_app_and_uninstall(client, CHART_NAME + "-crd")

    # create namespace
    ns = "cis-operator-system"
    command = "create namespace " + ns
    execute_kubectl_cmd(command, False)

    # install CIS v2
    cis_charts["annotations"] = cis_annotations
    cis_charts["values"]["global"]["cattle"]["clusterId"] = cluster_id
    cis_charts["values"]["global"]["cattle"]["clusterName"] = cluster_name
    cis_charts["chartName"] = CHART_NAME + "-crd"
    cis_charts["releaseName"] = CHART_NAME + "-crd"

    install_v2_app(client, rancher_repo, cis_charts, CHART_NAME + "-crd", ns)


    # install app
    cis_charts["chartName"] = CHART_NAME
    cis_charts["releaseName"] = CHART_NAME
    install_v2_app(client, rancher_repo, cis_charts, CHART_NAME, ns)


def test_v2_create_clusterscan_permissive():
    """
    Create cluster scan with profile: "rke-profile-permissive"
    Validate the scan CR and the scan report generated
    """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    rancher_repo = rancherrepo["data"][0]

    # install CIS if not installed
    if not check_v2_app_installed(client,CHART_NAME):
        install_cis_v2(client, cluster_id, cluster_name, rancher_repo)

    # getting the client again after installing v2 app
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    # create cluster scans
    scan_profile = "rke-profile-permissive"
    scan = create_clusterscan(client, scan_profile)

    # validate scan CR
    benchmark_version = cis_scan_profile_benchmark[scan_profile]
    validate_clusterscan_cr_and_report(
        client, scan, scan_profile, benchmark_version,
        cis_scan_report_results[benchmark_version]["total"],
        cis_scan_report_results[benchmark_version]["pass"],
        cis_scan_report_results[benchmark_version]["fail"],
        cis_scan_report_results[benchmark_version]["notApplicable"],
        cis_scan_report_results[benchmark_version]["skip"])


def test_v2_create_clusterscan_default():
    """
        Create cluster scan with profile: ""
        Default profile for the cluster will be picked.
        Currently testing it on an RKE cluster so the default scan profile
        will be "rke-profile-permissive"
        Validate the scan CR and the scan report generated
        """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    rancher_repo = rancherrepo["data"][0]

    # check if CIS is already installed and uninstall the app
    if not check_v2_app_installed(client, CHART_NAME):
        install_cis_v2(client, cluster_id, cluster_name, rancher_repo)

    # getting the client again after installing v2 app
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    # create cluster scans
    scan_profile = ""
    scan = create_clusterscan(client, scan_profile)

    if cluster_detail["cluster"]["provider"] == "rke":
        scan_profile = "rke-profile-permissive"
    elif cluster_detail["cluster"]["provider"] == "eks":
        scan_profile = "eks-profile"
    elif cluster_detail["cluster"]["provider"] == "gke":
        scan_profile = "gke-profile"
    else:
        scan_profile = "cis-1.5-profile"
    benchmark_version = cis_scan_profile_benchmark[scan_profile]
    validate_clusterscan_cr_and_report(
        client, scan, scan_profile, benchmark_version,
        cis_scan_report_results[benchmark_version]["total"],
        cis_scan_report_results[benchmark_version]["pass"],
        cis_scan_report_results[benchmark_version]["fail"],
        cis_scan_report_results[benchmark_version]["notApplicable"],
        cis_scan_report_results[benchmark_version]["skip"])


def test_v2_create_clusterscanprofile():
    """
    Create a custom Scan profile
    Run scan using this profile
    Validate the scan CR and the scan report generated
    """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    rancher_repo = rancherrepo["data"][0]

    # install CIS if not installed
    if not check_v2_app_installed(client,CHART_NAME):
        install_cis_v2(client, cluster_id, cluster_name, rancher_repo)

    # getting the client again after installing v2 app
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    # create clusterscanprofile
    benchmark_version = "rke-cis-1.5-permissive"
    tests_to_skip = ["1.1.11", "1.1.19"]
    # these tests as in "pass" state in an rke permissive scan run
    profile = create_clusterscanprofile(
        client, benchmark_version, tests_to_skip
    )
    print("profile", profile)
    assert profile["metadata"]["state"]["name"] == "active"
    assert profile["spec"]["skipTests"] == tests_to_skip
    assert profile["spec"]["benchmarkVersion"] == benchmark_version

    # create a clusterscan using this profile
    scan = create_clusterscan(client, profile["id"])

    # validate scan CR
    tests_skipped = len(tests_to_skip)
    validate_clusterscan_cr_and_report(
        client, scan, profile["id"], benchmark_version,
        cis_scan_report_results[benchmark_version]["total"],
        cis_scan_report_results[benchmark_version]["pass"] - tests_skipped,
        cis_scan_report_results[benchmark_version]["fail"],
        cis_scan_report_results[benchmark_version]["notApplicable"],
        cis_scan_report_results[benchmark_version]["skip"] + tests_skipped
    )


def test_v2_create_clusterscanbenchmark():
    """
    This test will fail because of issue : 
    https://github.com/rancher/cis-operator/issues/44
    Create a benchmark version similar to "rke-cis-1.5-permissive"
    Create a cluster scan profile using this benchmark
    Create a cluster scan using this profile

    """
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )
    rancherrepo = \
        client.list_catalog_cattle_io_clusterrepo(id="rancher-charts")
    cluster_id = cluster_detail["cluster"]["id"]
    cluster_name = cluster_detail["cluster"]["name"]
    rancher_repo = rancherrepo["data"][0]

    # install CIS if not installed
    if not check_v2_app_installed(client,CHART_NAME):
        install_cis_v2(client, cluster_id, cluster_name, rancher_repo)

    # getting the client again after installing v2 app
    client = \
        get_cluster_client_for_token_v1(
            cluster_detail["cluster"]["id"], USER_TOKEN
        )

    # create clusterscanbenchmark
    benchmark = create_clusterscanbenchmark(client, "rke")
    assert benchmark["metadata"]["state"]["name"] == "active"

    # create clusterscanprofile
    profile = create_clusterscanprofile(
        client, benchmark["id"]
    )

    # create a clusterscan using this profile
    scan = create_clusterscan(client, profile["id"])

    # validate scan CR
    # Report will have similar results as "rke-cis-1.5-permissive" scan
    key = "rke-cis-1.5-permissive"
    validate_clusterscan_cr_and_report(
        client, scan, profile["id"], benchmark["id"],
        cis_scan_report_results[key]["total"],
        cis_scan_report_results[key]["pass"],
        cis_scan_report_results[key]["fail"],
        cis_scan_report_results[key]["notApplicable"],
        cis_scan_report_results[key]["skip"]
    )


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster_detail["cluster"] = get_user_client_and_cluster()

    def fin():
        v1_client = \
            get_cluster_client_for_token_v1(
                cluster_detail["cluster"]["id"], USER_TOKEN
            )
        check_v2_app_and_uninstall(v1_client, CHART_NAME)
        check_v2_app_and_uninstall(v1_client, CHART_NAME + "-crd")

    request.addfinalizer(fin)


def install_v2_app(client, rancher_repo, chart_values, chart_name, ns):
    # install CRD
    response = client.action(obj=rancher_repo, action_name="install",
                             charts=[chart_values],
                             namespace=ns,
                             disableOpenAPIValidation=False,
                             noHooks=False,
                             projectId=None,
                             skipCRDs=False,
                             timeout="600s",
                             wait=True)
    print("response", response)
    app_list = wait_until_app_v2_deployed(client, chart_name)
    assert chart_name in app_list


def wait_for_scan_run(client, scan_id):
    start = time.time()
    scan = client.list_cis_cattle_io_clusterscan(id=scan_id)
    state = scan["data"][0]["status"]["display"]["state"]
    while state not in ["pass","fail","error"]:
        if time.time() - start > DEFAULT_SCAN_RUN_TIMEOUT:
            raise AssertionError(
                "Timed out waiting for scan to finish running")
        time.sleep(.5)
        scan = client.list_cis_cattle_io_clusterscan(id=scan_id)
        state = scan["data"][0]["status"]["display"]["state"]
    return scan


def validate_clusterscan_cr_and_report(
        client, scan, scan_profile, benchmark_version, total,
        passed, failed, notapplicable, skipped):
    if scan_profile == "rke-profile-permissive":
        assert scan["data"][0]["status"]["display"]["state"] == "pass"
    else:
        assert scan["data"][0]["status"]["display"]["state"] == "fail"
    assert scan["data"][0]["status"]["lastRunScanProfileName"] == \
           scan_profile, "Scan profile is not as expected"
    assert scan["data"][0]["status"]["summary"]["fail"] == failed
    assert scan["data"][0]["status"]["summary"]["notApplicable"] == \
           notapplicable
    assert scan["data"][0]["status"]["summary"]["pass"] == passed
    assert scan["data"][0]["status"]["summary"]["skip"] == skipped
    assert scan["data"][0]["status"]["summary"]["total"] == total

    # validate scan report
    scan_report = client.list_cis_cattle_io_clusterscanreport(
        id="scan-report-" + scan["data"][0]["id"]
    )

    # validate scan report
    report = json.loads(scan_report["data"][0]["spec"]["reportJSON"])
    assert report["version"] == benchmark_version
    assert report["total"] == total
    assert report["pass"] == passed
    assert report["fail"] == failed
    assert report["skip"] == skipped
    assert report["notApplicable"] == notapplicable


def install_cis_v2(client, cluster_id, cluster_name, rancher_repo):
    ns = "cis-operator-system"
    command = "create namespace " + ns
    execute_kubectl_cmd(command, False)

    # install CIS v2
    cis_charts["annotations"] = cis_annotations
    cis_charts["values"]["global"]["cattle"]["clusterId"] = cluster_id
    cis_charts["values"]["global"]["cattle"]["clusterName"] = cluster_name
    cis_charts["chartName"] = CHART_NAME + "-crd"
    cis_charts["releaseName"] = CHART_NAME + "-crd"

    install_v2_app(
        client, rancher_repo, cis_charts, CHART_NAME + "-crd", ns
    )

    # install app
    cis_charts["chartName"] = CHART_NAME
    cis_charts["releaseName"] = CHART_NAME
    install_v2_app(client, rancher_repo, cis_charts, CHART_NAME, ns)


def create_clusterscanprofile(client, benchmark_version, tests_to_skip=[]):
    profile_name = random_test_name("profile")
    profile_spec = {
        "benchmarkVersion": benchmark_version,
        "skipTests": tests_to_skip
    }
    profile = client.create_cis_cattle_io_clusterscanprofile(
        kind="ClusterScanProfile",
        spec=profile_spec,
        metadata={"name": profile_name}
    )
    return profile


def create_clusterscan(client, scan_profile):
    spec = {"scanProfileName": scan_profile}
    metadata = {"generateName": "scan-"}
    scan = client.create_cis_cattle_io_clusterscan(
        kind="ClusterScan",
        spec=spec,
        metadata=metadata,
        apiVersion="cis.cattle.io/v1"
    )
    # wait for scan to finish running
    scan_id = scan["id"]
    scan = wait_for_scan_run(client, scan_id)

    return scan


def create_clusterscanbenchmark(client, clusterprovider):
    benchmark_name = random_test_name("benchmark")
    spec = {"clusterProvider": clusterprovider,
            "minKubernetesVersion": "1.15.0"}
    benchmark = client.create_cis_cattle_io_clusterscanbenchmark(
        kind="ClusterScanBenchmark",
        metadata={"name":benchmark_name},
        spec=spec
    )
    return benchmark