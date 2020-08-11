# -*- coding: utf-8 -*-
"""
some deprecated calls

(c) 2008-2009, Holger Krekel and others
"""
import execnet


def PopenGateway(python=None):
    """ instantiate a gateway to a subprocess
        started with the given 'python' executable.
    """
    APIWARN("1.0.0b4", "use makegateway('popen')")
    spec = execnet.XSpec("popen")
    spec.python = python
    return execnet.default_group.makegateway(spec)


def SocketGateway(host, port):
    """ This Gateway provides interaction with a remote process
        by connecting to a specified socket.  On the remote
        side you need to manually start a small script
        (py/execnet/script/socketserver.py) that accepts
        SocketGateway connections or use the experimental
        new_remote() method on existing gateways.
    """
    APIWARN("1.0.0b4", "use makegateway('socket=host:port')")
    spec = execnet.XSpec("socket={}:{}".format(host, port))
    return execnet.default_group.makegateway(spec)


def SshGateway(sshaddress, remotepython=None, ssh_config=None):
    """ instantiate a remote ssh process with the
        given 'sshaddress' and remotepython version.
        you may specify an ssh_config file.
    """
    APIWARN("1.0.0b4", "use makegateway('ssh=host')")
    spec = execnet.XSpec("ssh=%s" % sshaddress)
    spec.python = remotepython
    spec.ssh_config = ssh_config
    return execnet.default_group.makegateway(spec)


def APIWARN(version, msg, stacklevel=3):
    import warnings

    Warn = DeprecationWarning("(since version {}) {}".format(version, msg))
    warnings.warn(Warn, stacklevel=stacklevel)
