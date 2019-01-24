# Go Debounce

[![Build Status](https://travis-ci.org/bep/debounce.svg)](https://travis-ci.org/bep/debounce)
[![GoDoc](https://godoc.org/github.com/bep/debounce?status.svg)](https://godoc.org/github.com/bep/debounce)
[![Go Report Card](https://goreportcard.com/badge/github.com/bep/debounce)](https://goreportcard.com/report/github.com/bep/debounce)
[![codecov](https://codecov.io/gh/bep/debounce/branch/master/graph/badge.svg)](https://codecov.io/gh/bep/debounce)
[![Release](https://img.shields.io/github/release/bep/debounce.svg?style=flat-square)](https://github.com/bep/debounce/releases/latest)

## Example

```go
func ExampleNew() {
	var counter uint64

	f := func() {
		atomic.AddUint64(&counter, 1)
	}

	debounced := debounce.New(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		for j := 0; j < 10; j++ {
			debounced(f)
		}

		time.Sleep(200 * time.Millisecond)
	}

	c := int(atomic.LoadUint64(&counter))

	fmt.Println("Counter is", c)
	// Output: Counter is 3
}
```

