import os
import random
import time

CATTLE_TEST_URL = os.environ.get('CATTLE_TEST_URL', "")
ADMIN_TOKEN = os.environ.get('ADMIN_TOKEN', "None")
USER_TOKEN = os.environ.get('USER_TOKEN', "None")
CLUSTER_NAME = os.environ.get("RANCHER_CLUSTER_NAME", "")
DEFAULT_TIMEOUT = 120


def random_int(start, end):
    return random.randint(start, end)


def random_test_name(name="test"):
    return name + "-" + str(random_int(10000, 99999))


def random_str():
    return 'random-{0}-{1}'.format(random_num(), int(time.time()))


def random_num():
    return random.randint(0, 1000000)


def random_name():
    return "test" + "-" + str(random_int(10000, 99999))


def wait_for(callback, timeout=DEFAULT_TIMEOUT, timeout_message=None):
    start = time.time()
    ret = callback()
    while ret is None or ret is False:
        time.sleep(.5)
        if time.time() - start > timeout:
            if timeout_message:
                raise Exception(timeout_message)
            else:
                raise Exception('Timeout waiting for condition')
        ret = callback()
    return ret
