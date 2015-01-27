#!/usr/bin/env python
import sys
import requests

from cattle import from_env

url = sys.argv[1]

r = requests.get(url)

if r.status_code == 200 and r.text.startswith('#!/bin/sh'):
    print url
    sys.exit(0)

r = requests.get(sys.argv[1])
try:
    url = r.headers['X-API-Schemas']
except KeyError:
    url = sys.argv[1]

client = from_env(url=url)

tokens = client.list_registrationToken(state='active')

if len(tokens) == 0:
    token = client.create_registrationToken()
else:
    token = tokens[0]

token = client.wait_success(token)

print token.registrationUrl



