// This package contains an implementation of the jsonpath query spec defined in https://goessner.net/articles/JsonPath.
//
// While there are other golang implementations of the same spec, none are complete, actively maintained, and support
// mutation. Since field mutation is the primary need for rancher audit logging, those existing libraries did not
// satisfy our needs.
package jsonpath
