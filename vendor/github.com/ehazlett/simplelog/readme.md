# Simple Log Formatter
Simple log formatting for [logrus](https://github.com/Sirupsen/logrus)

This log formatter removes the prefix from the output and just logs the text.

# Sample Usage

```go
package main

import (
    "github.com/ehazlett/simplelog"
    log "github.com/Sirupsen/logrus"
)

func init() {
    f := &simplelog.SimpleFormatter{}
    log.SetFormatter(f)
}

func main() {
    ...
}
```
