#!/bin/bash

/usr/sbin/sshd -D &

python3.11 app.py

sleep infinity