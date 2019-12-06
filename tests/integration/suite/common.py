import random
import time


def random_str():
    return 'random-{0}-{1}'.format(random_num(), int(time.time()))


def random_num():
    return random.randint(0, 1000000)


def find_one(method, *args, **kw):
    return find_count(1, method, *args, **kw)[0]


def find_count(count, method, *args, **kw):
    ret = method(*args, **kw)
    assert len(ret) == count
    return ret


def auth_check(schema, id, access, props=None):
    schema_type = schema.types[id]
    access_actual = set()

    try:
        if 'GET' in schema_type.collectionMethods:
            access_actual.add('r')
    except AttributeError:
        pass

    try:
        if 'GET' in schema_type.resourceMethods:
            access_actual.add('r')
    except AttributeError:
        pass

    try:
        if 'POST' in schema_type.collectionMethods:
            access_actual.add('c')
    except AttributeError:
        pass

    try:
        if 'DELETE' in schema_type.resourceMethods:
            access_actual.add('d')
    except AttributeError:
        pass

    try:
        if 'PUT' in schema_type.resourceMethods:
            access_actual.add('u')
    except AttributeError:
        pass

    assert access_actual == set(access)

    if props is None:
        return 1

    for i in ['description', 'annotations', 'labels']:
        if i not in props and i in schema_type.resourceFields.keys():
            props[i] = 'cru'

    for i in ['created', 'removed', 'transitioning', 'transitioningProgress',
              'removeTime', 'transitioningMessage', 'id', 'uuid', 'kind',
              'state', 'creatorId', 'finalizers', 'ownerReferences', 'type']:
        if i not in props and i in schema_type.resourceFields.keys():
            props[i] = 'r'

    for i in ['name']:
        if i not in props and i in schema_type.resourceFields.keys():
            props[i] = 'cr'

    prop = set(props.keys())
    prop_actual = set(schema_type.resourceFields.keys())

    if prop_actual != prop:
        for k in prop:
            assert k in prop_actual
        for k in prop_actual:
            assert k in prop

    assert prop_actual == prop

    for name, field in schema_type.resourceFields.items():
        assert name in props

        prop = set(props[name])
        prop_actual = set('r')

        prop.add(name)
        prop_actual.add(name)

        if field.create:
            prop_actual.add('c')
        if field.update:
            prop_actual.add('u')
        if 'writeOnly' in field and field.writeOnly:
            prop_actual.add('o')

        if prop_actual != prop:
            assert prop_actual == prop

    return 1


def wait_for_template_to_be_created(client, name, timeout=45):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(catalogId=name)
        if len(templates) > 0:
            found = True
        time.sleep(interval)
        interval *= 2


def wait_for_template_to_be_deleted(client, name, timeout=60):
    found = False
    start = time.time()
    interval = 0.5
    while not found:
        if time.time() - start > timeout:
            raise AssertionError(
                "Timed out waiting for templates")
        templates = client.list_template(catalogId=name)
        if len(templates) == 0:
            found = True
        time.sleep(interval)
        interval *= 2


def check_subject_in_rb(rbac, ns, subject_id, name):
    rbs = rbac.list_namespaced_role_binding(ns)
    for rb in rbs.items:
        if rb.metadata.name == name:
            for i in range(0, len(rb.subjects)):
                if rb.subjects[i].name == subject_id:
                    return True
    return False


def wait_for_atleast_workload(pclient, nsid, timeout=60, count=0):
    start = time.time()
    interval = 0.5
    workloads = pclient.list_workload(namespaceId=nsid)
    while len(workloads.data) < count:
        if time.time() - start > timeout:
            raise Exception('Timeout waiting for workload service')
        time.sleep(interval)
        interval *= 2
        workloads = pclient.list_workload(namespaceId=nsid)
    return workloads
