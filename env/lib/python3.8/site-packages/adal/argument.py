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
from .constants import OAuth2DeviceCodeResponseParameters

def validate_user_code_info(user_code_info):
    if not user_code_info:
        raise ValueError("the user_code_info parameter is required")

    if not user_code_info.get(OAuth2DeviceCodeResponseParameters.DEVICE_CODE):
        raise ValueError("the user_code_info is missing device_code")

    if not user_code_info.get(OAuth2DeviceCodeResponseParameters.INTERVAL):
        raise ValueError("the user_code_info is missing internal")

    if not user_code_info.get(OAuth2DeviceCodeResponseParameters.EXPIRES_IN):
        raise ValueError("the user_code_info is missing expires_in")
