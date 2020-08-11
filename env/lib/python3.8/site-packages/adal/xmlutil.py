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
    
from . import constants

XPATH_PATH_TEMPLATE = '*[local-name() = \'LOCAL_NAME\' and namespace-uri() = \'NAMESPACE\']'

def expand_q_names(xpath):

    namespaces = constants.XmlNamespaces.namespaces
    path_parts = xpath.split('/')
    for index, part in enumerate(path_parts):
        if part.find(":") != -1:
            q_parts = part.split(':')
            if len(q_parts) != 2:
                raise IndexError("Unable to parse XPath string: {} with QName: {}".format(xpath, part))

            expanded_path = XPATH_PATH_TEMPLATE.replace('LOCAL_NAME', q_parts[1])
            expanded_path = expanded_path.replace('NAMESPACE', namespaces[q_parts[0]])
            path_parts[index] = expanded_path

    return '/'.join(path_parts)

def xpath_find(dom, xpath):
    return dom.findall(xpath, constants.XmlNamespaces.namespaces)

def serialize_node_children(node):

    doc = ""
    for child in node.iter():
        if is_element_node(child):
            estring = ET.tostring(child)
            doc += estring if isinstance(estring, str) else estring.decode()

    return doc if doc else None

def is_element_node(node):
    return hasattr(node, 'tag')

def find_element_text(node):

    for child in node.iter():
        if child.text:
            return child.text
