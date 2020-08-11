# -*- coding: utf-8 -*-
"""
execnet
-------

pure python lib for connecting to local and remote Python Interpreters.

(c) 2012, Holger Krekel and others
"""
import apipkg

apipkg.initpkg(
    __name__,
    {
        "__version__": "._version:version",
        "PopenGateway": ".deprecated:PopenGateway",
        "SocketGateway": ".deprecated:SocketGateway",
        "SshGateway": ".deprecated:SshGateway",
        "makegateway": ".multi:makegateway",
        "set_execmodel": ".multi:set_execmodel",
        "HostNotFound": ".gateway_bootstrap:HostNotFound",
        "RemoteError": ".gateway_base:RemoteError",
        "TimeoutError": ".gateway_base:TimeoutError",
        "XSpec": ".xspec:XSpec",
        "Group": ".multi:Group",
        "MultiChannel": ".multi:MultiChannel",
        "RSync": ".rsync:RSync",
        "default_group": ".multi:default_group",
        "dumps": ".gateway_base:dumps",
        "loads": ".gateway_base:loads",
        "load": ".gateway_base:load",
        "dump": ".gateway_base:dump",
        "DataFormatError": ".gateway_base:DataFormatError",
    },
)
