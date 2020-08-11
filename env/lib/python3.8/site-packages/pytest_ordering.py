import re

replacements = {
    'first': 0,
    'second': 1,
    'third': 2,
    'fourth': 3,
    'fifth': 4,
    'sixth': 5,
    'seventh': 6,
    'eighth': 7,
    'last': -1,
    'second_to_last': -2,
    'third_to_last': -3,
    'fourth_to_last': -4,
    'fifth_to_last': -5,
    'sixth_to_last': -6,
    'seventh_to_last': -7,
    'eighth_to_last': -8,
}

def pytest_collection_modifyitems(session, config, items):
    items[:] = list(_order_tests(items))


def orderable(marker):
    match = re.match('^order(\d+)$', marker)
    return bool(match) or marker in replacements


def get_index(marker):
    match = re.match('^order(\d+)$', marker)
    if match:
        return int(match.group(1)) - 1
    return replacements[marker]


def split(dictionary):
    from_beginning, from_end = {}, {}
    for key, val in dictionary.items():
        if key >= 0:
            from_beginning[key] = val
        else:
            from_end[key] = val
    return from_beginning, from_end


def _order_tests(tests):
    ordered_tests = {}
    remaining_tests = []
    for test in tests:
        # There has got to be an API for this. :-/
        markers = test.keywords.__dict__['_markers']
        orderable_markers = [m for m in markers if orderable(m)]
        if len(orderable_markers) == 1:
            [orderable_marker] = orderable_markers
            ordered_tests[get_index(orderable_marker)] = test
        else:
            remaining_tests.append(test)
    from_beginning, from_end = split(ordered_tests)
    remaining_iter = iter(remaining_tests)
    for i in range(max(from_beginning or [-1]) + 1):
        if i in from_beginning:
            yield from_beginning[i]
        else:
            yield next(remaining_iter)
    # TODO TODO TODO
    for i in range(min(from_end or [0]), 0):
        if i in from_end:
            yield from_end[i]
        else:
            yield next(remaining_iter)
    for test in remaining_iter:
        yield test
