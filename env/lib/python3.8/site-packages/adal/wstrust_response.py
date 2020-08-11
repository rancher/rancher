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
    from xml.etree import cElementTree as ET
except ImportError:
    from xml.etree import ElementTree as ET
import re

from . import xmlutil
from . import log
from .adal_error import AdalError
from .constants import WSTrustVersion

# Creates a log message that contains the RSTR scrubbed of the actual SAML assertion.
def scrub_rstr_log_message(response_str):
    # A regular expression for finding the SAML Assertion in an response_str.  Used to remove the SAML
    # assertion when logging the response_str.
    assertion_regex = r'RequestedSecurityToken.*?((<.*?:Assertion.*?>).*<\/.*?Assertion>).*?'
    single_line_rstr, _ = re.subn(r'(\r\n|\n|\r)', '', response_str)

    match = re.search(assertion_regex, single_line_rstr)
    if not match:
        #No Assertion was matched so just return the response_str as is.
        scrubbed_rstr = single_line_rstr
    else:
        saml_assertion = match.group(1)
        saml_assertion_start_tag = match.group(2)
        scrubbed_rstr = single_line_rstr.replace(
            saml_assertion, saml_assertion_start_tag + 'ASSERTION CONTENTS REDACTED</saml:Assertion>')

    return 'RSTR Response: ' + scrubbed_rstr

def findall_content(xml_string, tag):
    """
    Given a tag name without any prefix,
    this function returns a list of the raw content inside this tag as-is.

    >>> findall_content("<ns0:foo> what <bar> ever </bar> content </ns0:foo>", "foo")
    [" what <bar> ever </bar> content "]

    Motivation:

    Usually we would use XML parser to extract the data by xpath.
    However the ElementTree in Python will implicitly normalize the output
    by "hoisting" the inner inline namespaces into the outmost element.
    The result will be a semantically equivalent XML snippet,
    but not fully identical to the original one.
    While this effect shouldn't become a problem in all other cases,
    it does not seem to fully comply with Exclusive XML Canonicalization spec
    (https://www.w3.org/TR/xml-exc-c14n/), and void the SAML token signature.
    SAML signature algo needs the "XML -> C14N(XML) -> Signed(C14N(Xml))" order.

    The binary extention lxml is probably the canonical way to solve this
    (https://stackoverflow.com/questions/22959577/python-exclusive-xml-canonicalization-xml-exc-c14n)
    but here we use this workaround, based on Regex, to return raw content as-is.
    """
    # \w+ is good enough for https://www.w3.org/TR/REC-xml/#NT-NameChar
    pattern = r"<(?:\w+:)?%(tag)s(?:[^>]*)>(.*)</(?:\w+:)?%(tag)s" % {"tag": tag}
    return re.findall(pattern, xml_string, re.DOTALL)


class WSTrustResponse(object):

    def __init__(self, call_context, response, wstrust_version):

        self._log = log.Logger("WSTrustResponse", call_context['log_context'])
        self._call_context = call_context
        self._response = response
        self._dom = None
        self._parents = None
        self.error_code = None
        self.fault_message = None
        self.token_type = None
        self.token = None
        self._wstrust_version = wstrust_version

        if response:
            self._log.debug(scrub_rstr_log_message(response))

    # Sample error message
    #<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing" xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
    #   <s:Header>
    #    <a:Action s:mustUnderstand="1">http://www.w3.org/2005/08/addressing/soap/fault</a:Action>
    #  - <o:Security s:mustUnderstand="1" xmlns:o="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
    #      <u:Timestamp u:Id="_0">
    #      <u:Created>2013-07-30T00:32:21.989Z</u:Created>
    #      <u:Expires>2013-07-30T00:37:21.989Z</u:Expires>
    #      </u:Timestamp>
    #    </o:Security>
    #    </s:Header>
    #  <s:Body>
    #    <s:Fault>
    #      <s:Code>
    #        <s:Value>s:Sender</s:Value>
    #        <s:Subcode>
    #        <s:Value xmlns:a="http://docs.oasis-open.org/ws-sx/ws-trust/200512">a:RequestFailed</s:Value>
    #        </s:Subcode>
    #      </s:Code>
    #      <s:Reason>
    #      <s:Text xml:lang="en-US">MSIS3127: The specified request failed.</s:Text>
    #      </s:Reason>
    #    </s:Fault>
    # </s:Body>
    #</s:Envelope>

    def _parse_error(self):

        error_found = False

        fault_node = xmlutil.xpath_find(self._dom, 's:Body/s:Fault/s:Reason/s:Text')
        if fault_node:
            self.fault_message = fault_node[0].text

            if self.fault_message:
                error_found = True

        # Subcode has minoccurs=0 and maxoccurs=1(default) according to the http://www.w3.org/2003/05/soap-envelope
        # Subcode may have another subcode as well. This is only targetting at top level subcode.
        # Subcode value may have different messages not always uses http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd.
        # text inside the value is not possible to select without prefix, so substring is necessary
        subnode = xmlutil.xpath_find(self._dom, 's:Body/s:Fault/s:Code/s:Subcode/s:Value')
        if len(subnode) > 1:
            raise AdalError("Found too many fault code values: {}".format(len(subnode)))

        if subnode:
            error_code = subnode[0].text
            self.error_code = error_code.split(':')[1]

        return error_found

    def _parse_token(self):
        if self._wstrust_version == WSTrustVersion.WSTRUST2005:
            token_type_nodes_xpath = 's:Body/t:RequestSecurityTokenResponse/t:TokenType'
            security_token_xpath = 't:RequestedSecurityToken'
        else:
            token_type_nodes_xpath = 's:Body/wst:RequestSecurityTokenResponseCollection/wst:RequestSecurityTokenResponse/wst:TokenType'
            security_token_xpath = 'wst:RequestedSecurityToken'

        token_type_nodes = xmlutil.xpath_find(self._dom, token_type_nodes_xpath)
        if not token_type_nodes:
            raise AdalError("No TokenType nodes found in RSTR")

        for node in token_type_nodes:
            if self.token:
                self._log.warn("Found more than one returned token. Using the first.")
                break

            token_type = xmlutil.find_element_text(node)
            if not token_type:
                self._log.warn("Could not find token type in RSTR token.")

            requested_token_node = xmlutil.xpath_find(self._parents[node], security_token_xpath)
            if len(requested_token_node) > 1:
                raise AdalError("Found too many RequestedSecurityToken nodes for token type: {}".format(token_type))

            if not requested_token_node:
                self._log.warn(
                    "Unable to find RequestsSecurityToken element associated with TokenType element: %(token_type)s",
                    {"token_type": token_type})
                continue

            # Adjust namespaces (without this they are autogenerated) so this is understood
            # by the receiver.  Then make a string repr of the element tree node.
            # See also http://blog.tomhennigan.co.uk/post/46945128556/elementtree-and-xmlns
            ET.register_namespace('saml', 'urn:oasis:names:tc:SAML:1.0:assertion')
            ET.register_namespace('ds', 'http://www.w3.org/2000/09/xmldsig#')

            token = ET.tostring(requested_token_node[0][0])

            if token is None:
                self._log.warn(
                    "Unable to find token associated with TokenType element: %(token_type)s",
                    {"token_type": token_type})
                continue

            self.token = token
            self.token_type = token_type

            self._log.info(
                "Found token of type: %(token_type)s",
                {"token_type": self.token_type})

        if self.token is None:
            raise AdalError("Unable to find any tokens in RSTR.")

    @staticmethod
    def _parse_token_by_re(raw_response):
        for rstr in findall_content(raw_response, "RequestSecurityTokenResponse"):
            token_types = findall_content(rstr, "TokenType")
            tokens = findall_content(rstr, "RequestedSecurityToken")
            if token_types and tokens:
                return tokens[0].encode('us-ascii'), token_types[0]


    def parse(self):
        if not self._response:
            raise AdalError("Received empty RSTR response body.")

        try:
            self._dom = ET.fromstring(self._response)
        except Exception as exp:
            raise AdalError('Failed to parse RSTR in to DOM', exp)
        
        try:
            self._parents = {c:p for p in self._dom.iter() for c in p}
            error_found = self._parse_error()
            if error_found:
                str_error_code = self.error_code or 'NONE'
                str_fault_message = self.fault_message or 'NONE'
                error_template = 'Server returned error in RSTR - ErrorCode: {} : FaultMessage: {}'
                raise AdalError(error_template.format(str_error_code, str_fault_message))

            token_found = self._parse_token_by_re(self._response)
            if token_found:
                self.token, self.token_type = token_found
            else:  # fallback to old logic
                self._parse_token()
        finally:
            self._dom = None
            self._parents = None

