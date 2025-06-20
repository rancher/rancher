/*
This package implements a jitter calculator and checker that asynchronously poll whether or not
a scheduled task should be pseudo-randomly scheduled based on the jitter configuration.
This package is not intended to be thread safe. Instances of the JitterChecker are
not intended to be shared across goroutines.
*/
package jitterbug
