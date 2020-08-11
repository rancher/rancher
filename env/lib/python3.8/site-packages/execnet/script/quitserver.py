# -*- coding: utf-8 -*-
"""

  send a "quit" signal to a remote server

"""
import socket
import sys

hostport = sys.argv[1]
host, port = hostport.split(":")
hostport = (host, int(port))

sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.connect(hostport)
sock.sendall('"raise KeyboardInterrupt"\n')
