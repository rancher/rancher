"""
This test suite contains tests to validate certificate create/edit/delete with
different possible way and with different roles of users.
Test requirement:
Below Env variables need to be set
CATTLE_TEST_URL - url to rancher server
ADMIN_TOKEN - Admin token from rancher
USER_TOKEN - User token from rancher
RANCHER_CLUSTER_NAME - Cluster name to run test on
RANCHER_VALID_TLS_KEY - takes authentic certificate key base64 encoded
RANCHER_VALID_TLS_CERT - takes authentic certificate base64 encoded
RANCHER_BYO_TLS_KEY - takes self signed certificate key base64 encoded
RANCHER_BYO_TLS_CERT - takes self signed certificate base64 encoded
AWS_HOSTED_ZONE_ID - Zone Id in AWS route53 where route53 will be created.
RANCHER_TEST_RBAC - To enable rbac tests
"""

from .common import (ApiError, CLUSTER_MEMBER, CLUSTER_OWNER, create_kubeconfig,
                     create_ns, create_project_and_ns,
                     get_cluster_client_for_token, get_project_client_for_token,
                     get_user_client, get_user_client_and_cluster, if_test_rbac,
                     PROJECT_OWNER, PROJECT_MEMBER, PROJECT_READ_ONLY,
                     random_test_name, rbac_get_namespace, rbac_get_project,
                     rbac_get_user_token_by_role, TEST_IMAGE, USER_TOKEN,
                     validate_ingress_using_endpoint, validate_workload,
                     wait_for_ingress_to_active, base64, TEST_IMAGE_PORT)
from lib.aws import AmazonWebServices
from pathlib import Path
import pytest
import os
import time


namespace = {"p_client": None, "ns": None, "cluster": None, "project": None,
             "c_client": None, "cert_valid": None, "cert_ssc": None,
             "cert_allns_valid": None, "cert_allns_ssc": None, "node_id": None}

route_entry_53_1 = random_test_name('auto-valid') + '.qa.rancher.space'
route_entry_53_2 = random_test_name('auto-ssc') + '.qa.rancher.space'


def get_ssh_key(ssh_key_name):
    home = str(Path.home())
    path = '{}/.ssh/{}'.format(home, ssh_key_name)
    if os.path.exists(path):
        with open(path, 'r') as f:
            ssh_key = f.read()
        return ssh_key


def get_private_key(env_var, key_name):
    key = os.environ.get(env_var)
    if key is not None:
        return base64.b64decode(key).decode("utf-8")
    else:
        return get_ssh_key(key_name)


rancher_private_key = get_private_key('RANCHER_VALID_TLS_KEY',
                                      'privkey.pem')
rancher_cert = get_private_key('RANCHER_VALID_TLS_CERT', 'fullchain.pem')
rancher_ssc_private_key = get_private_key('RANCHER_BYO_TLS_KEY',
                                          'key.pem')
rancher_ssc_cert = get_private_key('RANCHER_BYO_TLS_CERT', 'cert.pem')

rbac_role_list = [
                  CLUSTER_OWNER,
                  CLUSTER_MEMBER,
                  PROJECT_OWNER,
                  PROJECT_MEMBER,
                  PROJECT_READ_ONLY
                 ]


@pytest.mark.usefixtures("create_project_client")
class TestCertificate:

    @pytest.fixture(autouse="True")
    def certificate_test_setup(self):
        """
        Test set up which runs before and after all the tests in the class
        Creates Workload_2 if required and delete all the workload and ingres
        created after test execution.
        """
        self.p_client = namespace["p_client"]
        self.ns = namespace["ns"]
        self.c_client = namespace["c_client"]
        self.cluster = namespace["cluster"]
        self.project = namespace["project"]
        self.certificate_valid = namespace["cert_valid"]
        self.certificate_ssc = namespace["cert_ssc"]
        self.certificate_all_ns_valid = namespace["cert_allns_valid"]
        self.certificate_all_ns_ssc = namespace["cert_allns_ssc"]
        self.node_id = namespace["node_id"]
        wl_name = random_test_name("workload-test")
        wl_con = [{"name": "wk1-test",
                   "image": TEST_IMAGE}]
        scheduling = {"node": {"nodeId": self.node_id}}
        self.workload = self.p_client.create_workload(
            name=wl_name, containers=wl_con, namespaceId=self.ns.id,
            scheduling=scheduling
        )
        self.ingress = None
        self.workload_2 = None
        yield
        self.p_client.delete(self.workload)
        if self.workload_2 is not None:
            self.p_client.delete(self.workload_2)
        if self.ingress is not None:
            self.p_client.delete(self.ingress)

    def test_certificate_create_validcert_for_single_ns(self):
        """
        Test steps:
        1. Validate the workload available in ns-certificate namespace
        2. Create an ingress including trusted certificate scoped for current
        namespace and route53 host.
        3. validate the ingress using endpoint
        """
        ingress_name = random_test_name("ingress-test")
        host = route_entry_53_1
        path = "/name.html"
        rule = {"host": host,
                "paths": [{"path": path, "workloadIds": [self.workload.id],
                           "targetPort": TEST_IMAGE_PORT}]}
        tls = {"certificateId": self.certificate_valid.id, "hosts": [host]}
        validate_workload(self.p_client, self.workload, "deployment",
                          self.ns.name)
        self.ingress = self.p_client.create_ingress(
            name=ingress_name, namespaceId=self.ns.id, rules=[rule], tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload], certcheck=True)

    def test_certificate_create_validcert_for_all_ns(self):
        """
        Test steps:
        1. Validate the workload available in ns-certificate namespace
        2. Create an ingress including trusted certificate scoped for all
        namespace and route53 host.
        3. validate the ingress using endpoint
        """
        ingress_name = random_test_name("ingress-test")
        host = route_entry_53_1
        path = "/name.html"
        rule = {"host": host,
                "paths": [{"path": path, "workloadIds": [self.workload.id],
                           "targetPort": TEST_IMAGE_PORT}]
                }
        tls = {"certificateId": self.certificate_all_ns_valid.id,
               "hosts": [host]
               }
        validate_workload(self.p_client, self.workload, "deployment",
                          self.ns.name)
        self.ingress = self.p_client.create_ingress(
            name=ingress_name, namespaceId=self.ns.id, rules=[rule], tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload], certcheck=True)

    def test_certificate_create_validcert_for_all_ns_2(self):
        """
        Test steps:
        1. Create a namespace
        2. Create a workload in namespace created above.
        3. Validate the workload.
        4. Create an ingress including trusted certificate scoped for all
        namespace and route53 host.
        5. validate the ingress using endpoint
        """
        wl_name = random_test_name("workload-test")
        wl_con = [{"name": "wk2-test",
                   "image": TEST_IMAGE}]
        scheduling = {"node": {"nodeId": self.node_id}}
        ns_2 = create_ns(self.c_client, self.cluster, self.project)
        self.workload_2 = self.p_client.create_workload(
            name=wl_name, containers=wl_con, namespaceId=ns_2.id,
            scheduling=scheduling
            )
        validate_workload(self.p_client, self.workload_2, "deployment",
                          ns_2.name)
        ingress_name = random_test_name("ingress-test")
        host = route_entry_53_1
        path = "/name.html"
        rule = {"host": host,
                "paths": [{"path": path, "workloadIds": [self.workload_2.id],
                           "targetPort": TEST_IMAGE_PORT}]
                }
        tls = {"certificateId": self.certificate_all_ns_valid.id,
               "hosts": [host]
               }
        self.ingress = self.p_client.create_ingress(
            name="{}-2".format(ingress_name), namespaceId=ns_2.id,
            rules=[rule], tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload_2], certcheck=True)

    def test_certificate_create_ssc_for_single_ns(self):
        """
        Test steps:
        1. Validate the workload available in ns-certificate namespace
        2. Create an ingress including self signed certificate scoped for
        current namespace and route53 host.
        3. validate the ingress using endpoint
        """
        validate_workload(self.p_client, self.workload, "deployment",
                          self.ns.name)
        ingress_name = random_test_name("ingress-test")
        host = route_entry_53_2
        path = "/name.html"
        rule = {"host": host,
                "paths": [{"path": path, "workloadIds": [self.workload.id],
                           "targetPort": TEST_IMAGE_PORT}]}
        tls = {"certificateId": self.certificate_ssc.id, "hosts": [host]}
        self.ingress = self.p_client.create_ingress(
            name=ingress_name, namespaceId=self.ns.id, rules=[rule], tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        # validate_ingress(host, path)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload], certcheck=True,
            is_insecure=True
        )

    def test_certificate_create_ssc_for_all_ns(self):
        """
        Test steps:
        1. Validate the workload available in ns-certificate namespace
        2. Create an ingress including self signed certificate scoped for
        all namespace and route53 host.
        3. validate the ingress using endpoint
        """
        ingress_name = random_test_name("ingress-test")
        host = route_entry_53_2
        path = "/name.html"
        rule = {"host": host,
                "paths": [{"path": path, "workloadIds": [self.workload.id],
                           "targetPort": TEST_IMAGE_PORT}]
                }
        tls = {"certificateId": self.certificate_all_ns_ssc.id, "hosts": [host]}
        self.ingress = self.p_client.create_ingress(
            name=ingress_name, namespaceId=self.ns.id, rules=[rule], tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload], certcheck=True,
            is_insecure=True
        )

    def test_certificate_create_ssc_for_all_ns_2(self):
        """
        Test steps:
        1. Create a namespace
        2. Create a workload in namespace created above.
        3. Validate the workload.
        4. Create an ingress including trusted certificate scoped for all
        namespace and route53 host.
        5. validate the ingress using endpoint
        """
        wl_name = random_test_name("workload-test")
        wl_con = [{"name": "wk2-test",
                   "image": TEST_IMAGE}]
        scheduling = {"node": {"nodeId": self.node_id}}
        ns_2 = create_ns(self.c_client, self.cluster, self.project)
        self.workload_2 = self.p_client.create_workload(
            name=wl_name, containers=wl_con, namespaceId=ns_2.id,
            scheduling=scheduling
        )
        validate_workload(self.p_client, self.workload_2, "deployment",
                          ns_2.name)
        ingress_name = random_test_name("ingress-test")
        host = route_entry_53_2
        path = "/name.html"
        rule = {"host": host,
                "paths": [{"path": path, "workloadIds": [self.workload_2.id],
                           "targetPort": TEST_IMAGE_PORT}]
                }
        tls = {"certificateId": self.certificate_all_ns_ssc.id, "hosts": [host]}
        self.ingress = self.p_client.create_ingress(
            name="{}-2".format(ingress_name), namespaceId=ns_2.id, rules=[rule],
            tls=[tls])
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload_2], certcheck=True,
            is_insecure=True
        )

    def test_certificate_edit_ssc_to_valid_for_single_ns(self):
        """
        Test steps:
        1. Create an ingress pointing to self signed certificate scoped to
        current namespace.
        2. Update the certificate key to trusted.
        3. Reload the certificate.
        4. Update the ingress.
        5. validate the ingress using endpoint.
        """
        ingress_name = random_test_name("ingress-test")
        host_1 = route_entry_53_2
        host_2 = route_entry_53_1
        path = "/name.html"
        rule_1 = {"host": host_1,
                  "paths": [{"path": path, "workloadIds": [self.workload.id],
                             "targetPort": TEST_IMAGE_PORT}]}
        rule_2 = {"host": host_2,
                  "paths": [{"path": path, "workloadIds": [self.workload.id],
                             "targetPort": TEST_IMAGE_PORT}]}
        tls = {"certificateId": self.certificate_ssc.id, "hosts": [host_1]}
        tls_2 = {"certificateId": self.certificate_ssc.id, "hosts": [host_2]}
        self.ingress = self.p_client.create_ingress(
            name=ingress_name,  namespaceId=self.ns.id, rules=[rule_1],
            tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        self.p_client.update(
            self.certificate_ssc, key=rancher_private_key, certs=rancher_cert
        )
        self.p_client.reload(self.certificate_ssc)
        self.p_client.update(self.ingress, rules=[rule_2], tls=[tls_2])
        self.p_client.reload(self.ingress)
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload], certcheck=True)

    def test_certificate_edit_ssc_to_valid_cert_for_all_ns(self):
        """
        Test steps:
        1. Create an ingress pointing to self signed certificate scoped to
        all namespace.
        2. Update the certificate key to trusted.
        3. Reload the certificate.
        4. Update the ingress.
        5. validate the ingress using endpoint.
        """
        ingress_name = random_test_name("ingress-test")
        host_1 = route_entry_53_2
        host_2 = route_entry_53_1
        path = "/name.html"
        rule_1 = {"host": host_1,
                  "paths": [{"path": path, "workloadIds": [self.workload.id],
                             "targetPort": TEST_IMAGE_PORT}]
                  }
        rule_2 = {"host": host_2,
                  "paths": [{"path": path, "workloadIds": [self.workload.id],
                             "targetPort": TEST_IMAGE_PORT}]
                  }
        tls = {"certificateId": self.certificate_all_ns_ssc.id,
               "hosts": [host_1]}
        tls_2 = {"certificateId": self.certificate_all_ns_ssc.id,
                 "hosts": [host_2]}
        self.ingress = self.p_client.create_ingress(
            name=ingress_name, namespaceId=self.ns.id, rules=[rule_1],
            tls=[tls]
        )
        wait_for_ingress_to_active(self.p_client, self.ingress)
        self.p_client.update(
            self.certificate_all_ns_ssc, key=rancher_private_key,
            certs=rancher_cert
        )
        self.p_client.reload(self.certificate_all_ns_ssc)
        self.p_client.update(self.ingress, rules=[rule_2], tls=[tls_2])
        self.p_client.reload(self.ingress)
        wait_for_ingress_to_active(self.p_client, self.ingress)
        validate_ingress_using_endpoint(
            self.p_client, self.ingress, [self.workload], certcheck=True)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_create_certificate(self, role):
        """
        Test steps:
        1. Create certificate all namespace for all role
        2. Delete the certificate
        """
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        cert_name = random_test_name("cert-rbac")
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            with pytest.raises(ApiError) as e:
                p_client.create_certificate(
                    name=cert_name, key=rancher_private_key,
                    certs=rancher_cert
                )
                assert e.value.error.status == 403
                assert e.value.error.code == 'Forbidden'
        else:
            certificate_allns_valid = p_client.create_certificate(
                name=cert_name, key=rancher_private_key,
                certs=rancher_cert
            )
            assert certificate_allns_valid.issuer == 'E5'
            # Delete the certificate
            p_client.delete(certificate_allns_valid)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_create_namespaced_certificate(self, role):
        """
        Test steps:
        1. Create certificate for single namespace for all role
        2. Delete the certificate
        """
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, token)
        cert_name = random_test_name("cert-rbac")
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            with pytest.raises(ApiError) as e:
                p_client.create_namespaced_certificate(
                    name=cert_name, key=rancher_private_key,
                    certs=rancher_cert,
                    namespaceId=ns['name']
                )
                assert e.value.error.status == 403
                assert e.value.error.code == 'Forbidden'
        else:
            certificate_valid = p_client.create_namespaced_certificate(
                name=cert_name, key=rancher_private_key, certs=rancher_cert,
                namespaceId=ns['name']
            )
            assert certificate_valid.issuer == 'E5'
            # Delete the certificate
            p_client.delete(certificate_valid)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_list_namespaced_certificate(self, role):
        """
        Test steps:
        1. Create certificate for single namespace for all role as
        cluster owner
        2. List the created certificate for all roles
        3. Delete the certificate
        """
        c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, token)
        p_client_owner = get_project_client_for_token(project, c_owner_token)
        cert_name = random_test_name("cert-rbac")
        certificate_valid = p_client_owner.create_namespaced_certificate(
            name=cert_name, key=rancher_private_key, certs=rancher_cert,
            namespaceId=ns['name']
        )
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            cert_count = p_client.list_namespaced_certificate(name=cert_name)
            assert len(cert_count) == 0, '{} is able to list the ' \
                                         'certificate'.format(role)
        else:
            cert_count = p_client.list_namespaced_certificate(name=cert_name)
            assert len(cert_count) > 0, "{} couldn't to list the " \
                                        "certificate".format(role)

            # Delete the resources
            p_client.delete(certificate_valid)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_list_certificate(self, role):
        """
        Test steps:
        1. Create certificate for all namespace for all role as
        cluster owner
        2. List the created certificate for all roles
        3. Delete the certificate
        """
        c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        p_client_owner = get_project_client_for_token(project,
                                                      c_owner_token)
        cert_name = random_test_name("cert-rbac")
        certificate_allns_valid = p_client_owner.create_certificate(
            name=cert_name, key=rancher_private_key,
            certs=rancher_cert
        )
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            cert_count = p_client.list_certificate(name=cert_name)
            assert len(cert_count) == 0, '{} is able to list the ' \
                                         'certificate'.format(role)
        else:
            cert_count = p_client.list_certificate(name=cert_name)
            assert len(cert_count) > 0, "{} couldn't to list the " \
                                        "certificate".format(role)

            # Delete the resources
            p_client.delete(certificate_allns_valid)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_edit_certificate(self, role):
        """
        Test steps:
        1. Create certificate for single and all namespace for all role as
        cluster owner
        2. Update the created certificate for all roles
        3. Delete the certificate
        """
        c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        p_client_owner = get_project_client_for_token(project,
                                                      c_owner_token)
        cert_name = random_test_name("cert-rbac")
        certificate_allns_valid = p_client_owner.create_certificate(
            name=cert_name, key=rancher_private_key,
            certs=rancher_cert
        )
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            with pytest.raises(ApiError) as e:
                p_client.update(
                    certificate_allns_valid, key=rancher_ssc_private_key,
                    certs=rancher_ssc_cert)
                assert e.value.error.status == 403
                assert e.value.error.code == 'Forbidden'
        else:
            certificate_allns_valid = p_client.update(
                certificate_allns_valid, key=rancher_ssc_private_key,
                certs=rancher_ssc_cert)
            p_client.reload(certificate_allns_valid)
            assert certificate_allns_valid.issuer == 'Rancher QA CA'
            # Delete the resources
            p_client.delete(certificate_allns_valid)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_edit_namespaced_certificate(self, role):
        """
        Test steps:
        1. Create certificate for single namespace for all role as
        cluster owner
        2. Update the created certificate for all roles
        3. Delete the certificate
        """
        c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, token)
        p_client_owner = get_project_client_for_token(project,
                                                      c_owner_token)
        cert_name = random_test_name("cert-rbac")
        certificate_valid = p_client_owner.create_namespaced_certificate(
            name=cert_name, key=rancher_private_key, certs=rancher_cert,
            namespaceId=ns['name']
        )
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            with pytest.raises(ApiError) as e:
                p_client.update(certificate_valid, key=rancher_ssc_private_key,
                                certs=rancher_ssc_cert)
                assert e.value.error.status == 403
                assert e.value.error.code == 'Forbidden'
        else:
            certificate_valid = p_client.update(
                certificate_valid, key=rancher_ssc_private_key,
                certs=rancher_ssc_cert)
            p_client.reload(certificate_valid)
            assert certificate_valid.issuer == 'Rancher QA CA'
            # Delete the resources
            p_client.delete(certificate_valid)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_delete_certificate(self, role):
        """
        Test steps:
        1. Create certificate for single and all namespace for all role as
        cluster owner
        2. Delete the certificate as different roles.
        """
        c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        p_client_owner = get_project_client_for_token(project,
                                                      c_owner_token)
        cert_name = random_test_name("cert-rbac")
        certificate_allns_valid = p_client_owner.create_certificate(
            name=cert_name, key=rancher_private_key,
            certs=rancher_cert
        )
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            with pytest.raises(ApiError) as e:
                p_client.delete(certificate_allns_valid)
                assert e.value.error.status == 403
                assert e.value.error.code == 'Forbidden'
            p_client_owner.delete(certificate_allns_valid)
        else:
            p_client.delete(certificate_allns_valid)
            time.sleep(2)
            cert_count = p_client.list_certificate(name=cert_name)
            assert len(cert_count) == 0, '{} is not able to delete the ' \
                                         'certificate'.format(role)

    @if_test_rbac
    @pytest.mark.parametrize("role", rbac_role_list)
    def test_delete_namespaced_certificate(self, role):
        """
        Test steps:
        1. Create certificate for single namespace for all role as
        cluster owner
        2. Delete the certificate as different roles.
        """
        c_owner_token = rbac_get_user_token_by_role(CLUSTER_OWNER)
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        ns = rbac_get_namespace()
        p_client = get_project_client_for_token(project, token)
        p_client_owner = get_project_client_for_token(project,
                                                      c_owner_token)
        cert_name = random_test_name("cert-rbac")
        certificate_valid = p_client_owner.create_namespaced_certificate(
            name=cert_name, key=rancher_private_key, certs=rancher_cert,
            namespaceId=ns['name']
        )
        if role in (CLUSTER_MEMBER, PROJECT_READ_ONLY):
            with pytest.raises(ApiError) as e:
                p_client.delete(certificate_valid)
                assert e.value.error.status == 403
                assert e.value.error.code == 'Forbidden'
            p_client_owner.delete(certificate_valid)
        else:
            p_client.delete(certificate_valid)
            time.sleep(2)
            cert_count = p_client.list_namespaced_certificate(name=cert_name)
            assert len(cert_count) == 0, '{} is not able to delete the ' \
                                         'certificate'.format(role)

    @if_test_rbac
    @pytest.mark.parametrize("role", [PROJECT_OWNER, PROJECT_MEMBER])
    def test_list_certificate_cross_project(self, role):
        """
        Test steps:
        1. List the created all namespaced certificate present in
        Test-certificate project by test-certificate project owner and the
        users created by rbac test set up.
        """
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        default_p_client = self.p_client
        cert_count_by_role = p_client.list_certificate(name='cert-all-ns-valid')
        cert_count_default = default_p_client.list_certificate(
            name='cert-all-ns-valid')
        assert len(cert_count_default) > 0, "{} couldn't to list the " \
                                            "certificate".format(role)
        assert len(cert_count_by_role) == 0, "{} could list certificate in " \
                                             "'Test Certificate' project."

    @if_test_rbac
    @pytest.mark.parametrize("role", [PROJECT_OWNER, PROJECT_MEMBER])
    def test_list_ns_certificate_cross_project(self, role):
        """
        Test steps:
        1. List the created certificate present in Test-certificate project
        by test-certificate project owner and the users created by rbac test
        set up.
        """
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        default_p_client = self.p_client
        cert_count_by_role = p_client.list_namespaced_certificate(
            name='cert-valid')
        cert_count_default = default_p_client.list_namespaced_certificate(
            name='cert-valid')
        assert len(cert_count_default) > 0, "{} couldn't to list the " \
                                            "certificate".format(role)
        assert len(cert_count_by_role) == 0, "{} could list certificate in " \
                                             "'Test Certificate' project."

    @if_test_rbac
    @pytest.mark.parametrize("role", [PROJECT_OWNER, PROJECT_MEMBER])
    def test_edit_namespaced_certificate_cross_project(self, role):
        """
        Test steps:
        1. Update the created certificate present in Test-certificate project
        by the users created by rbac test set up.
        """
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        certificate_valid = self.certificate_ssc
        with pytest.raises(ApiError) as e:
            p_client.update(certificate_valid, key=rancher_private_key,
                            certs=rancher_cert)
            assert e.value.error.status == 403
            assert e.value.error.code == 'Forbidden'

    @if_test_rbac
    @pytest.mark.parametrize("role", [PROJECT_OWNER, PROJECT_MEMBER])
    def test_edit_certificate_cross_project(self, role):
        """
        Test steps:
        1. Update the created certificate present in Test-certificate project
        by the users created by rbac test set up.
        """
        token = rbac_get_user_token_by_role(role)
        project = rbac_get_project()
        p_client = get_project_client_for_token(project, token)
        certificate_valid = self.certificate_all_ns_ssc
        with pytest.raises(ApiError) as e:
            p_client.update(certificate_valid, key=rancher_private_key,
                            certs=rancher_cert)
            assert e.value.error.status == 403
            assert e.value.error.code == 'Forbidden'


@pytest.fixture(scope='module', autouse="True")
def create_project_client(request):
    client, cluster = get_user_client_and_cluster()
    create_kubeconfig(cluster)
    project, ns = create_project_and_ns(USER_TOKEN, cluster,
                                        project_name="test-certificate",
                                        ns_name="ns-certificate")
    p_client = get_project_client_for_token(project, USER_TOKEN)
    c_client = get_cluster_client_for_token(cluster, USER_TOKEN)
    certificate_valid = p_client.create_namespaced_certificate(
        name="cert-valid", key=rancher_private_key, certs=rancher_cert,
        namespaceId=ns['name']
    )
    assert certificate_valid.issuer == 'E5'

    certificate_allns_valid = p_client.create_certificate(
        name="cert-all-ns-valid", key=rancher_private_key,
        certs=rancher_cert
    )
    certificate_ssc = p_client.create_namespaced_certificate(
        name="cert-ssc", key=rancher_ssc_private_key, certs=rancher_ssc_cert,
        namespaceId=ns['name']
    )
    assert certificate_ssc.issuer == 'Rancher QA CA'
    certificate_allns_ssc = p_client.create_certificate(
        name="cert-all-ns-ssc", key=rancher_ssc_private_key,
        certs=rancher_ssc_cert
    )

    nodes = client.list_node(clusterId=cluster.id).data
    node_ip, node_id = None, None
    for i in range(len(nodes)):
        if nodes[i].worker:
            node_ip = nodes[i].externalIpAddress
            node_id = nodes[i].nodePoolId
            break
    aws_services = AmazonWebServices()

    aws_services.upsert_route_53_record_cname(
        route_entry_53_1, node_ip, record_type='A', record_ttl=60)
    aws_services.upsert_route_53_record_cname(
        route_entry_53_2, node_ip, record_type='A', record_ttl=60)

    namespace["p_client"] = p_client
    namespace["c_client"] = c_client
    namespace["ns"] = ns
    namespace["cluster"] = cluster
    namespace["project"] = project
    namespace["cert_valid"] = certificate_valid
    namespace["cert_ssc"] = certificate_ssc
    namespace["cert_allns_valid"] = certificate_allns_valid
    namespace["cert_allns_ssc"] = certificate_allns_ssc
    namespace["node_id"] = node_id

    # def fin():
    #     client = get_user_client()
    #     client.delete(namespace["project"])
    #     aws_services.upsert_route_53_record_cname(
    #         route_entry_53_1, node_ip, action='DELETE', record_type='A',
    #         record_ttl=60)
    #     aws_services.upsert_route_53_record_cname(
    #         route_entry_53_2, node_ip, action='DELETE', record_type='A',
    #         record_ttl=60)
    # request.addfinalizer(fin)
