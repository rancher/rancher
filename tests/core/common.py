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
    type = schema.types[id]
    access_actual = set()

    try:
        if 'GET' in type.collectionMethods:
            access_actual.add('r')
    except AttributeError:
        pass

    try:
        if 'GET' in type.resourceMethods:
            access_actual.add('r')
    except AttributeError:
        pass

    try:
        if 'POST' in type.collectionMethods:
            access_actual.add('c')
    except AttributeError:
        pass

    try:
        if 'DELETE' in type.resourceMethods:
            access_actual.add('d')
    except AttributeError:
        pass

    try:
        if 'PUT' in type.resourceMethods:
            access_actual.add('u')
    except AttributeError:
        pass

    assert access_actual == set(access)

    if props is None:
        return 1

    for i in ['description', 'annotations', 'labels']:
        if i not in props and i in type.resourceFields:
            props[i] = 'cru'

    for i in ['created', 'removed', 'transitioning', 'transitioningProgress',
              'removeTime', 'transitioningMessage', 'id', 'uuid', 'kind',
              'state', 'creatorId', 'finalizers', 'ownerReferences', 'type']:
        if i not in props and i in type.resourceFields:
            props[i] = 'r'

    for i in ['name']:
        if i not in props and i in type.resourceFields:
            props[i] = 'cr'

    prop = set(props.keys())
    prop_actual = set(type.resourceFields.keys())

    if prop_actual != prop:
        for k in prop:
            assert k in prop_actual
        for k in prop_actual:
            assert k in prop

    assert prop_actual == prop

    for name, field in type.resourceFields.items():
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
