#!/usr/bin/env python
import sys
import os

from cattle import from_env


client = from_env(access_key=os.environ['CATTLE_REGISTRATION_ACCESS_KEY'],
                  secret_key=os.environ['CATTLE_REGISTRATION_SECRET_KEY'])

if not client.valid():
    print "echo Invalid API credentials; exit 1"
    sys.exit(1)

key = sys.argv[1]

rs = client.list_register(key=key)

if len(rs) > 0:
    r = rs[0]
else:
    r = client.create_register(key=key)
    r = client.wait_success(r)
    r = client.list_register(key=key)[0]

print "export CATTLE_ACCESS_KEY={}".format(r.accessKey)
print "export CATTLE_SECRET_KEY={}".format(r.secretKey)
