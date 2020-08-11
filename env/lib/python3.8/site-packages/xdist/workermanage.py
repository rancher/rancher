from __future__ import print_function
import fnmatch
import os
import re
import threading

import py
import pytest
import execnet

import xdist.remote

from _pytest import runner  # XXX load dynamically


def parse_spec_config(config):
    xspeclist = []
    for xspec in config.getvalue("tx"):
        i = xspec.find("*")
        try:
            num = int(xspec[:i])
        except ValueError:
            xspeclist.append(xspec)
        else:
            xspeclist.extend([xspec[i + 1 :]] * num)
    if not xspeclist:
        raise pytest.UsageError(
            "MISSING test execution (tx) nodes: please specify --tx"
        )
    return xspeclist


class NodeManager(object):
    EXIT_TIMEOUT = 10
    DEFAULT_IGNORES = [".*", "*.pyc", "*.pyo", "*~"]

    def __init__(self, config, specs=None, defaultchdir="pyexecnetcache"):
        self.config = config
        self._nodesready = threading.Event()
        self.trace = self.config.trace.get("nodemanager")
        self.group = execnet.Group()
        if specs is None:
            specs = self._getxspecs()
        self.specs = []
        for spec in specs:
            if not isinstance(spec, execnet.XSpec):
                spec = execnet.XSpec(spec)
            if not spec.chdir and not spec.popen:
                spec.chdir = defaultchdir
            self.group.allocate_id(spec)
            self.specs.append(spec)
        self.roots = self._getrsyncdirs()
        self.rsyncoptions = self._getrsyncoptions()
        self._rsynced_specs = set()

    def rsync_roots(self, gateway):
        """Rsync the set of roots to the node's gateway cwd."""
        if self.roots:
            for root in self.roots:
                self.rsync(gateway, root, **self.rsyncoptions)

    def setup_nodes(self, putevent):
        self.config.hook.pytest_xdist_setupnodes(config=self.config, specs=self.specs)
        self.trace("setting up nodes")
        nodes = []
        for spec in self.specs:
            nodes.append(self.setup_node(spec, putevent))
        return nodes

    def setup_node(self, spec, putevent):
        gw = self.group.makegateway(spec)
        self.config.hook.pytest_xdist_newgateway(gateway=gw)
        self.rsync_roots(gw)
        node = WorkerController(self, gw, self.config, putevent)
        gw.node = node  # keep the node alive
        node.setup()
        self.trace("started node %r" % node)
        return node

    def teardown_nodes(self):
        self.group.terminate(self.EXIT_TIMEOUT)

    def _getxspecs(self):
        return [execnet.XSpec(x) for x in parse_spec_config(self.config)]

    def _getrsyncdirs(self):
        for spec in self.specs:
            if not spec.popen or spec.chdir:
                break
        else:
            return []
        import pytest
        import _pytest

        pytestpath = pytest.__file__.rstrip("co")
        pytestdir = py.path.local(_pytest.__file__).dirpath()
        config = self.config
        candidates = [py._pydir, pytestpath, pytestdir]
        candidates += config.option.rsyncdir
        rsyncroots = config.getini("rsyncdirs")
        if rsyncroots:
            candidates.extend(rsyncroots)
        roots = []
        for root in candidates:
            root = py.path.local(root).realpath()
            if not root.check():
                raise pytest.UsageError("rsyncdir doesn't exist: %r" % (root,))
            if root not in roots:
                roots.append(root)
        return roots

    def _getrsyncoptions(self):
        """Get options to be passed for rsync."""
        ignores = list(self.DEFAULT_IGNORES)
        ignores += self.config.option.rsyncignore
        ignores += self.config.getini("rsyncignore")

        return {"ignores": ignores, "verbose": self.config.option.verbose}

    def rsync(self, gateway, source, notify=None, verbose=False, ignores=None):
        """Perform rsync to remote hosts for node."""
        # XXX This changes the calling behaviour of
        #     pytest_xdist_rsyncstart and pytest_xdist_rsyncfinish to
        #     be called once per rsync target.
        rsync = HostRSync(source, verbose=verbose, ignores=ignores)
        spec = gateway.spec
        if spec.popen and not spec.chdir:
            # XXX This assumes that sources are python-packages
            #     and that adding the basedir does not hurt.
            gateway.remote_exec(
                """
                import sys ; sys.path.insert(0, %r)
            """
                % os.path.dirname(str(source))
            ).waitclose()
            return
        if (spec, source) in self._rsynced_specs:
            return

        def finished():
            if notify:
                notify("rsyncrootready", spec, source)

        rsync.add_target_host(gateway, finished=finished)
        self._rsynced_specs.add((spec, source))
        self.config.hook.pytest_xdist_rsyncstart(source=source, gateways=[gateway])
        rsync.send()
        self.config.hook.pytest_xdist_rsyncfinish(source=source, gateways=[gateway])


class HostRSync(execnet.RSync):
    """ RSyncer that filters out common files
    """

    def __init__(self, sourcedir, *args, **kwargs):
        self._synced = {}
        self._ignores = []
        ignores = kwargs.pop("ignores", None) or []
        for x in ignores:
            x = getattr(x, "strpath", x)
            self._ignores.append(re.compile(fnmatch.translate(x)))
        super(HostRSync, self).__init__(sourcedir=sourcedir, **kwargs)

    def filter(self, path):
        path = py.path.local(path)
        for cre in self._ignores:
            if cre.match(path.basename) or cre.match(path.strpath):
                return False
        else:
            return True

    def add_target_host(self, gateway, finished=None):
        remotepath = os.path.basename(self._sourcedir)
        super(HostRSync, self).add_target(
            gateway, remotepath, finishedcallback=finished, delete=True
        )

    def _report_send_file(self, gateway, modified_rel_path):
        if self._verbose:
            path = os.path.basename(self._sourcedir) + "/" + modified_rel_path
            remotepath = gateway.spec.chdir
            print("%s:%s <= %s" % (gateway.spec, remotepath, path))


def make_reltoroot(roots, args):
    # XXX introduce/use public API for splitting pytest args
    splitcode = "::"
    result = []
    for arg in args:
        parts = arg.split(splitcode)
        fspath = py.path.local(parts[0])
        for root in roots:
            x = fspath.relto(root)
            if x or fspath == root:
                parts[0] = root.basename + "/" + x
                break
        else:
            raise ValueError("arg %s not relative to an rsync root" % (arg,))
        result.append(splitcode.join(parts))
    return result


class WorkerController(object):
    ENDMARK = -1

    def __init__(self, nodemanager, gateway, config, putevent):
        self.nodemanager = nodemanager
        self.putevent = putevent
        self.gateway = gateway
        self.config = config
        self.workerinput = {
            "workerid": gateway.id,
            "workercount": len(nodemanager.specs),
            "slaveid": gateway.id,
            "slavecount": len(nodemanager.specs),
        }
        # TODO: deprecated name, backward compatibility only. Remove it in future
        self.slaveinput = self.workerinput
        self._down = False
        self._shutdown_sent = False
        self.log = py.log.Producer("workerctl-%s" % gateway.id)
        if not self.config.option.debug:
            py.log.setconsumer(self.log._keywords, None)

    def __repr__(self):
        return "<%s %s>" % (self.__class__.__name__, self.gateway.id)

    @property
    def shutting_down(self):
        return self._down or self._shutdown_sent

    def setup(self):
        self.log("setting up worker session")
        spec = self.gateway.spec
        args = self.config.args
        if not spec.popen or spec.chdir:
            args = make_reltoroot(self.nodemanager.roots, args)
        option_dict = vars(self.config.option)
        if spec.popen:
            name = "popen-%s" % self.gateway.id
            if hasattr(self.config, "_tmpdirhandler"):
                basetemp = self.config._tmpdirhandler.getbasetemp()
                option_dict["basetemp"] = str(basetemp.join(name))
        self.config.hook.pytest_configure_node(node=self)
        self.channel = self.gateway.remote_exec(xdist.remote)
        self.channel.send((self.workerinput, args, option_dict))
        if self.putevent:
            self.channel.setcallback(self.process_from_remote, endmarker=self.ENDMARK)

    def ensure_teardown(self):
        if hasattr(self, "channel"):
            if not self.channel.isclosed():
                self.log("closing", self.channel)
                self.channel.close()
            # del self.channel
        if hasattr(self, "gateway"):
            self.log("exiting", self.gateway)
            self.gateway.exit()
            # del self.gateway

    def send_runtest_some(self, indices):
        self.sendcommand("runtests", indices=indices)

    def send_runtest_all(self):
        self.sendcommand("runtests_all")

    def shutdown(self):
        if not self._down:
            try:
                self.sendcommand("shutdown")
            except IOError:
                pass
            self._shutdown_sent = True

    def sendcommand(self, name, **kwargs):
        """ send a named parametrized command to the other side. """
        self.log("sending command %s(**%s)" % (name, kwargs))
        self.channel.send((name, kwargs))

    def notify_inproc(self, eventname, **kwargs):
        self.log("queuing %s(**%s)" % (eventname, kwargs))
        self.putevent((eventname, kwargs))

    def process_from_remote(self, eventcall):  # noqa too complex
        """ this gets called for each object we receive from
            the other side and if the channel closes.

            Note that channel callbacks run in the receiver
            thread of execnet gateways - we need to
            avoid raising exceptions or doing heavy work.
        """
        try:
            if eventcall == self.ENDMARK:
                err = self.channel._getremoteerror()
                if not self._down:
                    if not err or isinstance(err, EOFError):
                        err = "Not properly terminated"  # lost connection?
                    self.notify_inproc("errordown", node=self, error=err)
                    self._down = True
                return
            eventname, kwargs = eventcall
            if eventname in ("collectionstart",):
                self.log("ignoring %s(%s)" % (eventname, kwargs))
            elif eventname == "workerready":
                self.notify_inproc(eventname, node=self, **kwargs)
            elif eventname == "workerfinished":
                self._down = True
                self.workeroutput = self.slaveoutput = kwargs["workeroutput"]
                self.notify_inproc("workerfinished", node=self)
            elif eventname in ("logstart", "logfinish"):
                self.notify_inproc(eventname, node=self, **kwargs)
            elif eventname in ("testreport", "collectreport", "teardownreport"):
                item_index = kwargs.pop("item_index", None)
                rep = unserialize_report(eventname, kwargs["data"])
                if item_index is not None:
                    rep.item_index = item_index
                self.notify_inproc(eventname, node=self, rep=rep)
            elif eventname == "collectionfinish":
                self.notify_inproc(eventname, node=self, ids=kwargs["ids"])
            elif eventname == "runtest_protocol_complete":
                self.notify_inproc(eventname, node=self, **kwargs)
            elif eventname == "logwarning":
                self.notify_inproc(
                    eventname,
                    message=kwargs["message"],
                    code=kwargs["code"],
                    nodeid=kwargs["nodeid"],
                    fslocation=kwargs["nodeid"],
                )
            else:
                raise ValueError("unknown event: %s" % (eventname,))
        except KeyboardInterrupt:
            # should not land in receiver-thread
            raise
        except:  # noqa
            from _pytest._code import ExceptionInfo

            excinfo = ExceptionInfo()
            print("!" * 20, excinfo)
            self.config.notify_exception(excinfo)
            self.shutdown()
            self.notify_inproc("errordown", node=self, error=excinfo)


def unserialize_report(name, reportdict):
    def assembled_report(reportdict):
        from _pytest._code.code import (
            ReprEntry,
            ReprEntryNative,
            ReprExceptionInfo,
            ReprFileLocation,
            ReprFuncArgs,
            ReprLocals,
            ReprTraceback,
        )

        if reportdict["longrepr"]:
            if (
                "reprcrash" in reportdict["longrepr"]
                and "reprtraceback" in reportdict["longrepr"]
            ):

                reprtraceback = reportdict["longrepr"]["reprtraceback"]
                reprcrash = reportdict["longrepr"]["reprcrash"]

                unserialized_entries = []
                reprentry = None
                for entry_data in reprtraceback["reprentries"]:
                    data = entry_data["data"]
                    entry_type = entry_data["type"]
                    if entry_type == "ReprEntry":
                        reprfuncargs = None
                        reprfileloc = None
                        reprlocals = None
                        if data["reprfuncargs"]:
                            reprfuncargs = ReprFuncArgs(**data["reprfuncargs"])
                        if data["reprfileloc"]:
                            reprfileloc = ReprFileLocation(**data["reprfileloc"])
                        if data["reprlocals"]:
                            reprlocals = ReprLocals(data["reprlocals"]["lines"])

                        reprentry = ReprEntry(
                            lines=data["lines"],
                            reprfuncargs=reprfuncargs,
                            reprlocals=reprlocals,
                            filelocrepr=reprfileloc,
                            style=data["style"],
                        )
                    elif entry_type == "ReprEntryNative":
                        reprentry = ReprEntryNative(data["lines"])
                    else:
                        report_unserialization_failure(entry_type, name, reportdict)
                    unserialized_entries.append(reprentry)
                reprtraceback["reprentries"] = unserialized_entries

                exception_info = ReprExceptionInfo(
                    reprtraceback=ReprTraceback(**reprtraceback),
                    reprcrash=ReprFileLocation(**reprcrash),
                )

                for section in reportdict["longrepr"]["sections"]:
                    exception_info.addsection(*section)
                reportdict["longrepr"] = exception_info
        return reportdict

    if name == "testreport":
        return runner.TestReport(**assembled_report(reportdict))
    elif name == "collectreport":
        return runner.CollectReport(**assembled_report(reportdict))


def report_unserialization_failure(type_name, report_name, reportdict):
    from pprint import pprint

    url = "https://github.com/pytest-dev/pytest-xdist/issues"
    stream = py.io.TextIO()
    pprint("-" * 100, stream=stream)
    pprint("INTERNALERROR: Unknown entry type returned: %s" % type_name, stream=stream)
    pprint("report_name: %s" % report_name, stream=stream)
    pprint(reportdict, stream=stream)
    pprint("Please report this bug at %s" % url, stream=stream)
    pprint("-" * 100, stream=stream)
    assert 0, stream.getvalue()
