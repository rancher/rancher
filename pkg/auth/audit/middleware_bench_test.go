package audit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
)

// Benchmarks for memory and CPU profiling
//
// ## Overview
//
// These benchmarks are organized by audit log level to facilitate "apples to apples"
// comparisons. Each level section contains benchmarks with progressively larger
// response sizes, allowing you to compare memory usage both:
// - Across levels (same size, different levels)
// - Within a level (same level, different sizes)
//
// ## Running Comparative Benchmarks
//
// ### Compare memory usage across ALL audit levels for a specific response size:
//
//    # Compare 1GB responses across all 4 audit levels
//    go test -bench='Benchmark.*_1GB$' -benchmem -benchtime=1x
//
//    # Compare 100MB responses across all levels
//    go test -bench='Benchmark.*_100MB$' -benchmem -benchtime=1x
//
// ### Compare memory usage for ONE level across different sizes:
//
//    # See how LevelNull scales from small to gigabyte responses
//    go test -bench='BenchmarkLevelNull_' -benchmem -benchtime=1x
//
//    # See how LevelRequestResponse scales across sizes
//    go test -bench='BenchmarkLevelRequestResponse_' -benchmem -benchtime=1x
//
// ### Run a specific size/level combination:
//
//    go test -bench='BenchmarkLevelNull_1GB$' -benchmem -benchtime=1x
//    go test -bench='BenchmarkLevelRequestResponse_1GB$' -benchmem -benchtime=1x
//
// ## Memory Profiling
//
// ### Generate and analyze memory profiles:
//
//    # Profile a specific benchmark
//    go test -bench=BenchmarkLevelNull_1GB \
//            -memprofile=mem_null_1gb.prof \
//            -benchmem \
//            -benchtime=1x
//
//    # Analyze the profile
//    go tool pprof -alloc_space mem_null_1gb.prof
//    (pprof) top10
//    (pprof) list wrapWriter.Write
//
// ### Compare profiles across levels:
//
//    # Generate profiles for each level
//    go test -bench=BenchmarkLevelNull_1GB -memprofile=mem_null.prof -benchmem -benchtime=1x
//    go test -bench=BenchmarkLevelRequest_1GB -memprofile=mem_req.prof -benchmem -benchtime=1x
//    go test -bench=BenchmarkLevelRequestResponse_1GB -memprofile=mem_reqresp.prof -benchmem -benchtime=1x
//
//    # Compare allocations
//    go tool pprof -base=mem_null.prof mem_reqresp.prof
//
// ## Expected Behavior
//
// Current implementation (with bug at handler.go:65):
// - LevelNull: Buffers entire response (~1GB allocation for 1GB response) - BUG
// - LevelHeaders: Buffers entire response (~1GB allocation for 1GB response) - BUG
// - LevelRequest: Buffers entire response (~1GB allocation for 1GB response) - BUG
// - LevelRequestResponse: Buffers entire response (~1GB allocation for 1GB response) - CORRECT
//
// After fixing wrapWriter.Write() to conditionally buffer:
// - LevelNull: Minimal allocation (<10MB for 1GB response)
// - LevelHeaders: Minimal allocation (<10MB for 1GB response)
// - LevelRequest: Minimal allocation (<10MB for 1GB response)
// - LevelRequestResponse: Full buffering (~1GB allocation for 1GB response)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================
//
// Note: newTestRequest, newSizedResponseHandler, and newTestAuditWriter are
// defined in middleware_test.go and shared across both test files.

// Helper function for standard response size benchmarks
func benchmarkMiddlewareWithSize(b *testing.B, level auditlogv1.Level, responseSize int) {
	writer, _ := newTestAuditWriter(level)
	middleware := NewAuditLogMiddleware(writer)

	handler := newSizedResponseHandler(responseSize, http.StatusOK)
	wrappedHandler := middleware(handler)

	// Create a URL that won't match any special patterns
	testURL := "/v3/clusters"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := newTestRequest(http.MethodGet, testURL, nil)
		rw := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rw, req)
	}
}

// Helper function for request/response body benchmarks
func benchmarkMiddlewareWithRequestBody(b *testing.B, level auditlogv1.Level, requestSize, responseSize int) {
	writer, _ := newTestAuditWriter(level)
	middleware := NewAuditLogMiddleware(writer)

	handler := newSizedResponseHandler(responseSize, http.StatusOK)
	wrappedHandler := middleware(handler)

	// Pre-generate request body
	requestBody := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("x", requestSize-12))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := newTestRequest(http.MethodPost, "/v3/clusters", strings.NewReader(requestBody))
		rw := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rw, req)
	}
}

// Helper function for concurrent benchmarks
func benchmarkMiddlewareConcurrent(b *testing.B, level auditlogv1.Level, responseSize int) {
	writer, _ := newTestAuditWriter(level)
	middleware := NewAuditLogMiddleware(writer)

	handler := newSizedResponseHandler(responseSize, http.StatusOK)
	wrappedHandler := middleware(handler)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := newTestRequest(http.MethodGet, "/v3/clusters", nil)
			rw := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rw, req)
		}
	})
}

// Helper function for streaming benchmarks
// Simulates reverse proxy behavior by writing response in chunks
func benchmarkMiddlewareStreaming(b *testing.B, level auditlogv1.Level, totalSize, chunkSize int) {
	writer, _ := newTestAuditWriter(level)
	middleware := NewAuditLogMiddleware(writer)

	// Handler that writes response in chunks, simulating reverse proxy streaming
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)

		// Pre-allocate chunk to avoid allocation overhead in benchmark
		chunk := []byte(strings.Repeat("x", chunkSize))
		remaining := totalSize

		for remaining > 0 {
			writeSize := chunkSize
			if remaining < chunkSize {
				writeSize = remaining
			}
			_, _ = rw.Write(chunk[:writeSize])

			// Flush if available, like reverse proxy does
			if flusher, ok := rw.(http.Flusher); ok {
				flusher.Flush()
			}

			remaining -= writeSize
		}
	})

	wrappedHandler := middleware(handler)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := newTestRequest(http.MethodGet, "/v3/clusters", nil)
		rw := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rw, req)
	}
}

// =============================================================================
// LEVEL NULL (Metadata only) BENCHMARKS
// =============================================================================
//
// These benchmarks test auditlogv1.LevelNull, which should only log metadata
// (timestamps, user info, status code) without buffering request/response bodies.
//
// Expected behavior: Minimal memory usage regardless of response size
// Current bug: Buffers entire response, causing high memory usage

func BenchmarkLevelNull_1KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 1024)
}

func BenchmarkLevelNull_100KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 100*1024)
}

func BenchmarkLevelNull_1MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 1024*1024)
}

func BenchmarkLevelNull_10MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 10*1024*1024)
}

func BenchmarkLevelNull_100MB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelNull, 10*1024*1024, 100*1024*1024)
}

func BenchmarkLevelNull_1GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelNull, 1024*1024, 1024*1024*1024)
}

func BenchmarkLevelNull_2GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelNull, 1024*1024, 2*1024*1024*1024)
}

// Streaming benchmarks for LevelNull
func BenchmarkLevelNull_Streaming_100MB(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelNull, 100*1024*1024, 32*1024)
}

func BenchmarkLevelNull_Streaming_1GB(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelNull, 1024*1024*1024, 32*1024)
}

// Concurrent benchmark for LevelNull
func BenchmarkLevelNull_Concurrent_100KB(b *testing.B) {
	benchmarkMiddlewareConcurrent(b, auditlogv1.LevelNull, 100*1024)
}

// =============================================================================
// LEVEL HEADERS BENCHMARKS
// =============================================================================
//
// These benchmarks test auditlogv1.LevelHeaders, which logs metadata and
// request/response headers without buffering request/response bodies.
//
// Expected behavior: Minimal memory usage regardless of response size
// Current bug: Buffers entire response, causing high memory usage

func BenchmarkLevelHeaders_1KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 1024)
}

func BenchmarkLevelHeaders_100KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 100*1024)
}

func BenchmarkLevelHeaders_1MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 1024*1024)
}

func BenchmarkLevelHeaders_10MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 10*1024*1024)
}

func BenchmarkLevelHeaders_100MB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelHeaders, 10*1024*1024, 100*1024*1024)
}

func BenchmarkLevelHeaders_1GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelHeaders, 1024*1024, 1024*1024*1024)
}

func BenchmarkLevelHeaders_2GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelHeaders, 1024*1024, 2*1024*1024*1024)
}

// =============================================================================
// LEVEL REQUEST BENCHMARKS
// =============================================================================
//
// These benchmarks test auditlogv1.LevelRequest, which logs metadata, headers,
// and request body without buffering the response body.
//
// Expected behavior: Buffers request body only, minimal response buffering
// Current bug: Buffers entire response, causing high memory usage

func BenchmarkLevelRequest_1KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 1024)
}

func BenchmarkLevelRequest_100KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 100*1024)
}

func BenchmarkLevelRequest_1MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 1024*1024)
}

func BenchmarkLevelRequest_10MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 10*1024*1024)
}

func BenchmarkLevelRequest_100MB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 10*1024*1024, 100*1024*1024)
}

func BenchmarkLevelRequest_1GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 1024*1024, 1024*1024*1024)
}

func BenchmarkLevelRequest_2GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 1024*1024, 2*1024*1024*1024)
}

// With request body benchmarks for LevelRequest
func BenchmarkLevelRequest_WithRequestBody_1KB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 1024, 1024)
}

func BenchmarkLevelRequest_WithRequestBody_100KB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 100*1024, 100*1024)
}

func BenchmarkLevelRequest_WithRequestBody_1MB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 1024*1024, 1024*1024)
}

// =============================================================================
// LEVEL REQUEST+RESPONSE BENCHMARKS
// =============================================================================
//
// These benchmarks test auditlogv1.LevelRequestResponse, which logs everything
// including request and response bodies. This level MUST buffer response bodies.
//
// Expected behavior: Buffers both request and response bodies
// Current behavior: Correct - buffers as expected

func BenchmarkLevelRequestResponse_1KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 1024)
}

func BenchmarkLevelRequestResponse_100KB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 100*1024)
}

func BenchmarkLevelRequestResponse_1MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 1024*1024)
}

func BenchmarkLevelRequestResponse_10MB(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 10*1024*1024)
}

func BenchmarkLevelRequestResponse_100MB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 10*1024*1024, 100*1024*1024)
}

func BenchmarkLevelRequestResponse_1GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024*1024, 1024*1024*1024)
}

func BenchmarkLevelRequestResponse_2GB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024*1024, 2*1024*1024*1024)
}

// Streaming benchmarks for LevelRequestResponse
func BenchmarkLevelRequestResponse_Streaming_100MB(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelRequestResponse, 100*1024*1024, 32*1024)
}

func BenchmarkLevelRequestResponse_Streaming_1GB(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelRequestResponse, 1024*1024*1024, 32*1024)
}

// Concurrent benchmark for LevelRequestResponse
func BenchmarkLevelRequestResponse_Concurrent_100KB(b *testing.B) {
	benchmarkMiddlewareConcurrent(b, auditlogv1.LevelRequestResponse, 100*1024)
}

// With request body benchmarks for LevelRequestResponse
func BenchmarkLevelRequestResponse_WithRequestBody_1KB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024, 1024)
}

func BenchmarkLevelRequestResponse_WithRequestBody_100KB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 100*1024, 100*1024)
}

func BenchmarkLevelRequestResponse_WithRequestBody_1MB(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024*1024, 1024*1024)
}