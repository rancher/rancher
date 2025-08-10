package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var (
	counter int64
	listen  string
)

func main() {
	flag.StringVar(&listen, "listen", ":8125", "Listen address")
	flag.Parse()

	fmt.Println("listening ", listen)
	http.ListenAndServe(listen, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next := atomic.AddInt64(&counter, 1)
		http.FileServer(http.Dir("./")).ServeHTTP(rw, req)
		fmt.Println("request", next, time.Now().Sub(start))
	}))
}
