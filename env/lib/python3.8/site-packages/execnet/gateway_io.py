# -*- coding: utf-8 -*-
"""
execnet io initialization code

creates io instances used for gateway io
"""
import os
import shlex
import sys

try:
    from execnet.gateway_base import Popen2IO, Message
except ImportError:
    from __main__ import Popen2IO, Message

from functools import partial


class Popen2IOMaster(Popen2IO):
    def __init__(self, args, execmodel):
        self.popen = p = execmodel.PopenPiped(args)
        Popen2IO.__init__(self, p.stdin, p.stdout, execmodel=execmodel)

    def wait(self):
        try:
            return self.popen.wait()
        except OSError:
            pass  # subprocess probably dead already

    def kill(self):
        killpopen(self.popen)


def killpopen(popen):
    try:
        if hasattr(popen, "kill"):
            popen.kill()
        else:
            killpid(popen.pid)
    except EnvironmentError:
        sys.stderr.write("ERROR killing: %s\n" % (sys.exc_info()[1]))
        sys.stderr.flush()


def killpid(pid):
    if hasattr(os, "kill"):
        os.kill(pid, 15)
    elif sys.platform == "win32" or getattr(os, "_name", None) == "nt":
        import ctypes

        PROCESS_TERMINATE = 1
        handle = ctypes.windll.kernel32.OpenProcess(PROCESS_TERMINATE, False, pid)
        ctypes.windll.kernel32.TerminateProcess(handle, -1)
        ctypes.windll.kernel32.CloseHandle(handle)
    else:
        raise EnvironmentError("no method to kill {}".format(pid))


popen_bootstrapline = "import sys;exec(eval(sys.stdin.readline()))"


def shell_split_path(path):
    """
    Use shell lexer to split the given path into a list of components,
    taking care to handle Windows' '\' correctly.
    """
    if sys.platform.startswith("win"):
        # replace \\ by / otherwise shlex will strip them out
        path = path.replace("\\", "/")
    return shlex.split(path)


def popen_args(spec):
    args = shell_split_path(spec.python) if spec.python else [sys.executable]
    args.append("-u")
    if spec is not None and spec.dont_write_bytecode:
        args.append("-B")
    # Slight gymnastics in ordering these arguments because CPython (as of
    # 2.7.1) ignores -B if you provide `python -c "something" -B`
    args.extend(["-c", popen_bootstrapline])
    return args


def ssh_args(spec):
    # NOTE: If changing this, you need to sync those changes to vagrant_args
    # as well, or, take some time to further refactor the commonalities of
    # ssh_args and vagrant_args.
    remotepython = spec.python or "python"
    args = ["ssh", "-C"]
    if spec.ssh_config is not None:
        args.extend(["-F", str(spec.ssh_config)])

    args.extend(spec.ssh.split())
    remotecmd = '{} -c "{}"'.format(remotepython, popen_bootstrapline)
    args.append(remotecmd)
    return args


def vagrant_ssh_args(spec):
    # This is the vagrant-wrapped version of SSH. Unfortunately the
    # command lines are incompatible to just channel through ssh_args
    # due to ordering/templating issues.
    # NOTE: This should be kept in sync with the ssh_args behaviour.
    # spec.vagrant is identical to spec.ssh in that they both carry
    # the remote host "address".
    remotepython = spec.python or "python"
    args = ["vagrant", "ssh", spec.vagrant_ssh, "--", "-C"]
    if spec.ssh_config is not None:
        args.extend(["-F", str(spec.ssh_config)])
    remotecmd = '{} -c "{}"'.format(remotepython, popen_bootstrapline)
    args.extend([remotecmd])
    return args


def create_io(spec, execmodel):
    if spec.popen:
        args = popen_args(spec)
        return Popen2IOMaster(args, execmodel)
    if spec.ssh:
        args = ssh_args(spec)
        io = Popen2IOMaster(args, execmodel)
        io.remoteaddress = spec.ssh
        return io
    if spec.vagrant_ssh:
        args = vagrant_ssh_args(spec)
        io = Popen2IOMaster(args, execmodel)
        io.remoteaddress = spec.vagrant_ssh
        return io


#
# Proxy Gateway handling code
#
# master: proxy initiator
# forwarder: forwards between master and sub
# sub: sub process that is proxied to the initiator

RIO_KILL = 1
RIO_WAIT = 2
RIO_REMOTEADDRESS = 3
RIO_CLOSE_WRITE = 4


class ProxyIO(object):
    """ A Proxy IO object allows to instantiate a Gateway
    through another "via" gateway.  A master:ProxyIO object
    provides an IO object effectively connected to the sub
    via the forwarder.  To achieve this, master:ProxyIO interacts
    with forwarder:serve_proxy_io() which itself
    instantiates and interacts with the sub.
    """

    def __init__(self, proxy_channel, execmodel):
        # after exchanging the control channel we use proxy_channel
        # for messaging IO
        self.controlchan = proxy_channel.gateway.newchannel()
        proxy_channel.send(self.controlchan)
        self.iochan = proxy_channel
        self.iochan_file = self.iochan.makefile("r")
        self.execmodel = execmodel

    def read(self, nbytes):
        return self.iochan_file.read(nbytes)

    def write(self, data):
        return self.iochan.send(data)

    def _controll(self, event):
        self.controlchan.send(event)
        return self.controlchan.receive()

    def close_write(self):
        self._controll(RIO_CLOSE_WRITE)

    def kill(self):
        self._controll(RIO_KILL)

    def wait(self):
        return self._controll(RIO_WAIT)

    @property
    def remoteaddress(self):
        return self._controll(RIO_REMOTEADDRESS)

    def __repr__(self):
        return "<RemoteIO via {}>".format(self.iochan.gateway.id)


class PseudoSpec:
    def __init__(self, vars):
        self.__dict__.update(vars)

    def __getattr__(self, name):
        return None


def serve_proxy_io(proxy_channelX):
    execmodel = proxy_channelX.gateway.execmodel
    log = partial(
        proxy_channelX.gateway._trace, "serve_proxy_io:%s" % proxy_channelX.id
    )
    spec = PseudoSpec(proxy_channelX.receive())
    # create sub IO object which we will proxy back to our proxy initiator
    sub_io = create_io(spec, execmodel)
    control_chan = proxy_channelX.receive()
    log("got control chan", control_chan)

    # read data from master, forward it to the sub
    # XXX writing might block, thus blocking the receiver thread
    def forward_to_sub(data):
        log("forward data to sub, size %s" % len(data))
        sub_io.write(data)

    proxy_channelX.setcallback(forward_to_sub)

    def controll(data):
        if data == RIO_WAIT:
            control_chan.send(sub_io.wait())
        elif data == RIO_KILL:
            control_chan.send(sub_io.kill())
        elif data == RIO_REMOTEADDRESS:
            control_chan.send(sub_io.remoteaddress)
        elif data == RIO_CLOSE_WRITE:
            control_chan.send(sub_io.close_write())

    control_chan.setcallback(controll)

    # write data to the master coming from the sub
    forward_to_master_file = proxy_channelX.makefile("w")

    # read bootstrap byte from sub, send it on to master
    log("reading bootstrap byte from sub", spec.id)
    initial = sub_io.read(1)
    assert initial == "1".encode("ascii"), initial
    log("forwarding bootstrap byte from sub", spec.id)
    forward_to_master_file.write(initial)

    # enter message forwarding loop
    while True:
        try:
            message = Message.from_io(sub_io)
        except EOFError:
            log("EOF from sub, terminating proxying loop", spec.id)
            break
        message.to_io(forward_to_master_file)
    # proxy_channelX will be closed from remote_exec's finalization code


if __name__ == "__channelexec__":
    serve_proxy_io(channel)  # noqa
