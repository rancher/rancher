// Package multicall eases running different programs from the same binary, by registering different command names.
// Those names would be checked against os.Args[0], which contains the command name or path used to invoke the program.
// This is normally exercised via symlinks to a single binary.
package multicall

import (
	"fmt"
	"os"
)

var registered = make(map[string]func())

// Register a name or path with a given function, so a later call to Resolve will find it.
func Register(name string, fn func()) {
	if _, ok := registered[name]; ok {
		panic(fmt.Sprintf("multicall: command %q already registered", name))
	}
	registered[name] = fn
}

// Resolve returns any function registered for the current executions program name (argv[0]), or nil.
// It is meant to be called early in the program execution/main body.
func Resolve() func() {
	return registered[os.Args[0]]
}
