# Integration Tests

## Dependencies

---

Instructions for installing dependencies can be found in the [wiki](https://github.com/rancher/rancher/wiki/Setting-Up-Rancher-Development-Environment).

## CI

---

To execute a full CI test, run `make test` or `make ci` which will build Rancher (including any local changes) and run the test suite on it.

## Install

---

You can create a Python virtual environment to install all this into with:

This can be done at the top-level of the Rancher repository clone:

```
python3.11 -m venv .venv && source .venv/bin/activate
```

```
pip install -r requirements.txt
pip install tox
```

## How to Run Integration Tests

---

*The tests require python 3.11. Some other versions may work, but are not guaranteed to be supported.*

Start a local rancher instance on port 8443. If the password is not set to `admin`, set the environment variable `RANCHER_SERVER_PASSWORD` to the appropriate password.

```
make run
```

Export the address of the local rancher instance for the tests to use.

Replace the address `172.17.0.2` below with the address that Rancher logs out when starting.

```
export CATTLE_TEST_URL=https://172.17.0.2:8443
```

Run with [Tox](https://tox.wiki/en/4.11.0/) - from tests/integration dir. See [tox.ini](./tox.ini) for configuration

* the entire suite: `tox` (from tests/integration)
* a single file with tox: `tox -- -x suite/test_users.py` (from tests/integration)

Run with [pytest](https://docs.pytest.org/en/7.4.x/)

* a single test: `pytest -k test_user_cant_delete_self`
* a file: `pytest tests/integration/suite/test_auth_proxy.py`


## Notes

---

To debug, use the standard inline process: `breakpoint()`

The tests use a [Rancher python client](https://github.com/rancher/client-python) which is dynamically generated based on the Rancher API, so any methods called on it do not exist until runtime.
It will be helpful to use the debugger and tools like `dir(admin_mc)` to see all methods on a variable.

`conftest.py` holds the primary supporting code. See [pytest docs](https://docs.pytest.org) for more info.
