#!/bin/bash

/usr/sbin/sshd -D &

python app.py

sleep infinity