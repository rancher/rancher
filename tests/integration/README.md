# Integration scripts for Rancher


## Development

#### Install

Setup virtualenv and enter, then: `pip install -r requirements.txt`

#### Run tests

Start a local rancher instance on port 8443

Now you can execute the following to run:

* the entire suite: `tox` (from tests/integration)
* a single test: `pytest -k test_user_cant_delete_self`
* a file: `pytest tests/integration/suite/test_auth_proxy.py`
* a single file with tox: `tox -- -x suite/test_users.py` (from tests/integration)

To execute a full CI test, run `make ci` which will build Rancher locally (including any local changes) and run the test suite on it.
  

#### Notes

To debug, use the standard inline process: `import pdb;pdb.set_trace()`

The tests use a Rancher python client (https://github.com/rancher/client-python) which is dynamically generated based on the Rancher API, so any methods called on it do not exist until runtime.
It will be helpful to use the debugger and tools like `dir(admin_mc)` to see all methods on a variable.

conftest.py holds the primary supporting code, see `func admin_mc` for example.
That function is passed into the `test_user_cant_delete_self` test dynamically, as is any requested param.
