package dump

import (
	"bytes"
	"io"
	"os"
	"os/signal"
	"runtime"

	"github.com/maruel/panicparse/stack"
)

func GoroutineDumpOn(signals ...os.Signal) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)
	go func() {
		for range c {
			var (
				buf       []byte
				stackSize int
			)
			bufferLen := 16384
			for stackSize == len(buf) {
				buf = make([]byte, bufferLen)
				stackSize = runtime.Stack(buf, true)
				bufferLen *= 2
			}
			buf = buf[:stackSize]
			src := bytes.NewBuffer(buf)
			if goroutines, err := stack.ParseDump(src, os.Stderr); err == nil {
				buckets := stack.SortBuckets(stack.Bucketize(goroutines, stack.AnyValue))
				srcLen, pkgLen := stack.CalcLengths(buckets, true)
				p := &stack.Palette{}
				for _, bucket := range buckets {
					_, _ = io.WriteString(os.Stderr, p.BucketHeader(&bucket, true, len(buckets) > 1))
					_, _ = io.WriteString(os.Stderr, p.StackLines(&bucket.Signature, srcLen, pkgLen, true))
				}
				io.Copy(os.Stderr, src)
			} else {
				io.Copy(os.Stderr, src)
			}
		}
	}()
}
