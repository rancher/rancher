#------------------------------------------------------------------------------
#
# Copyright (c) Microsoft Corporation. 
# All rights reserved.
# 
# This code is licensed under the MIT License.
# 
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files(the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and / or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions :
# 
# The above copyright notice and this permission notice shall be included in
# all copies or substantial portions of the Software.
# 
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
# THE SOFTWARE.
#
#------------------------------------------------------------------------------

try:
    from urllib.parse import urlparse
except ImportError:
    from urlparse import urlparse # pylint: disable=import-error

try:
    from xml.etree import cElementTree as ET
except ImportError:
    from xml.etree import ElementTree as ET

import requests

from . import log
from . import util
from . import xmlutil
from .constants import XmlNamespaces, WSTrustVersion
from .adal_error import AdalError

TRANSPORT_BINDING_XPATH = 'wsp:ExactlyOne/wsp:All/sp:TransportBinding'
TRANSPORT_BINDING_2005_XPATH = 'wsp:ExactlyOne/wsp:All/sp2005:TransportBinding' #pylint: disable=invalid-name

SOAP_ACTION_XPATH = 'wsdl:operation/soap12:operation'
RST_SOAP_ACTION_13 = 'http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue'
RST_SOAP_ACTION_2005 = 'http://schemas.xmlsoap.org/ws/2005/02/trust/RST/Issue' #pylint: disable=invalid-name
SOAP_TRANSPORT_XPATH = 'soap12:binding'
SOAP_HTTP_TRANSPORT_VALUE = 'http://schemas.xmlsoap.org/soap/http'

PORT_XPATH = 'wsdl:service/wsdl:port'
ADDRESS_XPATH = 'wsa10:EndpointReference/wsa10:Address'

def _url_is_secure(endpoint_url):
    parsed = urlparse(endpoint_url)
    return parsed.scheme == 'https'

class Mex(object):

    def __init__(self, call_context, url):

        self._log = log.Logger("MEX", call_context.get('log_context'))
        self._call_context = call_context
        self._url = url
        self._dom = None
        self._parents = None
        self._mex_doc = None
        self.username_password_policy = {}
        self._log.debug("Mex created with url: %(mex_url)s",
                        {"mex_url": self._url})

    def discover(self):
        options = util.create_request_options(self, {'headers': {'Content-Type': 'application/soap+xml'}})

        try:
            operation = "Mex Get"
            resp = requests.get(self._url, headers=options['headers'],
                                verify=self._call_context.get('verify_ssl', None),
                                proxies=self._call_context.get('proxies', None))
            util.log_return_correlation_id(self._log, operation, resp)
        except Exception:
            self._log.exception(
                "%(operation)s request failed", {"operation": operation})
            raise

        if resp.status_code == 429:
            resp.raise_for_status()  # Will raise requests.exceptions.HTTPError
        if not util.is_http_success(resp.status_code):
            return_error_string = u"{} request returned http error: {}".format(operation, resp.status_code)
            error_response = ""
            if resp.text:
                return_error_string = u"{} and server response: {}".format(return_error_string, resp.text)
                try:
                    error_response = resp.json()
                except ValueError:
                    pass
            raise AdalError(return_error_string, error_response)
        else:
            try:
                self._mex_doc = resp.text
                #options = {'errorHandler':self._log.error}
                self._dom = ET.fromstring(self._mex_doc)
                self._parents = {c:p for p in self._dom.iter() for c in p}
                self._parse()
            except Exception:
                self._log.info('Failed to parse mex response in to DOM')
                raise

    def _check_policy(self, policy_node):
        policy_id = policy_node.attrib["{{{}}}Id".format(XmlNamespaces.namespaces['wsu'])]
        
        # Try with Transport Binding XPath
        transport_binding_nodes = xmlutil.xpath_find(policy_node, TRANSPORT_BINDING_XPATH)
        
        # If unsuccessful, try again with 2005 XPath
        if not transport_binding_nodes:
            transport_binding_nodes = xmlutil.xpath_find(policy_node, TRANSPORT_BINDING_2005_XPATH)

        # If we did not find any binding, this is potentially bad.
        if not transport_binding_nodes:
            self._log.debug(
                "Potential policy did not match required transport binding: %(policy_id)s",
                {"policy_id": policy_id})
        else:
            self._log.debug("Found matching policy id: %(policy_id)s",
                            {"policy_id": policy_id})

        return policy_id

    def _select_username_password_polices(self, xpath):

        policies = {}
        username_token_nodes = xmlutil.xpath_find(self._dom, xpath)
        if not username_token_nodes:
            self._log.warn("No username token policy nodes found.")
            return

        for node in username_token_nodes:
            policy_node = self._parents[self._parents[self._parents[self._parents[self._parents[self._parents[self._parents[node]]]]]]]
            policy_id = self._check_policy(policy_node)
            if policy_id:
                id_ref = '#' + policy_id
                policies[id_ref] = {policy_id:id_ref}

        return policies if policies else None

    def _check_soap_action_and_transport(self, binding_node):

        soap_action = ""
        soap_transport = ""
        name = binding_node.get('name')

        soap_transport_attributes = ""
        soap_action_attributes = xmlutil.xpath_find(binding_node, SOAP_ACTION_XPATH)[0].attrib['soapAction']

        if soap_action_attributes:
            soap_action = soap_action_attributes
            soap_transport_attributes = xmlutil.xpath_find(binding_node, SOAP_TRANSPORT_XPATH)[0].attrib['transport']

        if soap_transport_attributes:
            soap_transport = soap_transport_attributes

        if soap_transport == SOAP_HTTP_TRANSPORT_VALUE:
            if soap_action == RST_SOAP_ACTION_13:
                self._log.debug(
                    'found binding matching Action and Transport: %(binding_node)s',
                    {"binding_node": name})
                return WSTrustVersion.WSTRUST13
            elif soap_action == RST_SOAP_ACTION_2005:
                self._log.debug(
                    'found binding matching Action and Transport: %(binding_node)s',
                    {"binding_node": name})
                return WSTrustVersion.WSTRUST2005

        self._log.debug(
            'binding node did not match soap Action or Transport: %(binding_node)s',
            {"binding_node": name})
        return WSTrustVersion.UNDEFINED

    def _get_matching_bindings(self, policies):

        bindings = {}
        binding_policy_ref_nodes = xmlutil.xpath_find(self._dom, 'wsdl:binding/wsp:PolicyReference')

        for node in binding_policy_ref_nodes:
            uri = node.get('URI')
            policy = policies.get(uri)
            if policy:
                binding_node = self._parents[node]
                binding_name = binding_node.get('name')

                version = self._check_soap_action_and_transport(binding_node)
                if version != WSTrustVersion.UNDEFINED:                  
                    bindings[binding_name] = {
                        'url': uri,
                        'version': version
                        }

        return bindings if bindings else None

    def _get_ports_for_policy_bindings(self, bindings, policies):

        port_nodes = xmlutil.xpath_find(self._dom, PORT_XPATH)
        if not port_nodes:
            self._log.warn("No ports found")

        for node in port_nodes:
            binding_id = node.get('binding')
            binding_id = binding_id.split(':')[-1]

            trust_policy = bindings.get(binding_id)
            if trust_policy:
                binding_policy = policies.get(trust_policy.get('url'))
                if binding_policy and not binding_policy.get('url', None):
                    binding_policy['version'] = trust_policy['version']
                    address_node = node.find(ADDRESS_XPATH, XmlNamespaces.namespaces)
                    if address_node is None:
                        raise AdalError("No address nodes on port")

                    address = xmlutil.find_element_text(address_node)
                    if _url_is_secure(address):
                        binding_policy['url'] = address
                    else:
                        self._log.warn(
                            "Skipping insecure endpoint: %(mex_endpoint)s",
                            {"mex_endpoint": address})

    def _select_single_matching_policy(self, policies):

        matching_policies = [p for p in policies.values() if p.get('url')]
        if not matching_policies:
            self._log.warn("No policies found with a url.")
            return

        wstrust13_policy = None
        wstrust2005_policy = None
        for policy in matching_policies:
            version = policy.get('version', None)
            if  version == WSTrustVersion.WSTRUST13:
                wstrust13_policy = policy
            elif version == WSTrustVersion.WSTRUST2005:
                wstrust2005_policy = policy

        if wstrust13_policy is None and wstrust2005_policy is None:
            self._log.warn('No policies found for either wstrust13 or wstrust2005')

        self.username_password_policy = wstrust13_policy or wstrust2005_policy

    def _parse(self):
        policies = self._select_username_password_polices(
            'wsp:Policy/wsp:ExactlyOne/wsp:All/sp:SignedEncryptedSupportingTokens/wsp:Policy/sp:UsernameToken/wsp:Policy/sp:WssUsernameToken10')

        xpath2005 = 'wsp:Policy/wsp:ExactlyOne/wsp:All/sp2005:SignedSupportingTokens/wsp:Policy/sp2005:UsernameToken/wsp:Policy/sp2005:WssUsernameToken10'       
        if policies:
            policies2005 = self._select_username_password_polices(xpath2005)
            if policies2005:
                policies.update(policies2005)
        else:
            policies = self._select_username_password_polices(xpath2005)

        if not policies:
            raise AdalError("No matching policies.")
            

        bindings = self._get_matching_bindings(policies)
        if not bindings:
            raise AdalError("No matching bindings.")

        self._get_ports_for_policy_bindings(bindings, policies)
        self._select_single_matching_policy(policies)

        if not self._url:
            raise AdalError("No ws-trust endpoints match requirements.")
