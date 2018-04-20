# log: Simple wrapper for logrus

`logrus` by default outputs everything to `stderr` which makes Infof, Debugf messages appear to be error messages.

This package provides only the following:

* Sends Infof, Debugf messages to `stdout`.
* Sends Errorf messages to go `stderr`.
* Allow the set the log level.

All other things are not supported because I only care about Infof, Debugf and Errorf.

Example: https://github.com/leodotcloud/log-example


## Dynamically change loglevel

### server

This repo has a package thats runs a http server over a unix socket, using which the log level can be controlled.

### client

There is also a client binary available that can be used to change the log level.
https://github.com/leodotcloud/loglevel

```shell
# To get the current log level
loglevel

# To change the log level to debug
logevel --set debug

# To change it back to info
loglevel --set info

# To set it to show only errors
loglevel --set error
```
