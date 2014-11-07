#!/usr/bin/env python
import sys
from cattle import from_env

url = sys.argv[1]

client = from_env(url=sys.argv[1])

tokens = client.list_registrationToken(state='active')

if len(tokens) == 0:
    token = client.create_registrationToken()
else:
    token = tokens[0]

token = client.wait_success(token)

print token.registrationUrl



