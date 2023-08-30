# Integration Tests

## Dependencies

---

Instructions for installing dependencies can be found in the [wiki](https://github.com/rancher/rancher/wiki/Setting-Up-Rancher-2.0-Development-Environment#setting-up-and-running-the-tests).

## CI

---

To execute a full CI test, run `make test` or `make ci` which will build Rancher (including any local changes) and run the test suite on it.

## Install

---

```
pip install -r requirements.txt
pip install tox
```


## How to Run Integration Tests

---

*The tests require python 3.7. Some other versions may work, but are not guaranteed to be supported.*

Start a local rancher instance on port 8443. If the password is not set to `admin`, set the environment variable `RANCHER_SERVER_PASSWORD` to the appropriate password.

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
