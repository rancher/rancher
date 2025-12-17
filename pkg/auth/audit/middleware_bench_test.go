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
// ## Running Benchmarks with Memory Profiling
//
// To reproduce the user-reported 1.7GB heap issue, run these commands:
//
// 1. Run a specific benchmark with memory profiling:
//    go test -bench=BenchmarkMiddleware_GigabyteResponse_LevelMetadata \
//            -memprofile=mem.prof \
//            -benchmem \
//            -benchtime=1x
//
// 2. Run the streaming benchmark (simulates reverse proxy behavior):
//    go test -bench=BenchmarkMiddleware_StreamingResponse_1GB_LevelMetadata \
//            -memprofile=mem.prof \
//            -benchmem \
//            -benchtime=1x
//
// 3. Analyze the memory profile:
//    go tool pprof -alloc_space mem.prof
//    (pprof) top10
//    (pprof) list wrapWriter.Write
//
// 4. Compare memory usage between levels (shows the bug):
//    # This should use ~1GB but DOES because of bug:
//    go test -bench=BenchmarkMiddleware_GigabyteResponse_LevelMetadata -benchmem -benchtime=1x
//
//    # This SHOULD use ~1GB and does (correct behavior):
//    go test -bench=BenchmarkMiddleware_GigabyteResponse_LevelRequestResponse -benchmem -benchtime=1x
//
// 5. Run with heap dump (like user's scenario):
//    GODEBUG=allocfreetrace=1 go test -bench=BenchmarkMiddleware_StreamingResponse_1GB_LevelMetadata -benchtime=1x
//
// ## Expected Results (Current Bug)
//
// With the current implementation, you'll see:
// - LevelMetadata: ~1GB allocation (BUG - should be minimal)
// - LevelRequestResponse: ~1GB allocation (CORRECT - needs the data)
//
// After fixing handler.go:65 to conditionally buffer:
// - LevelMetadata: <10MB allocation (only metadata/headers)
// - LevelRequestResponse: ~1GB allocation (buffers response as needed)

// BenchmarkMiddleware_SmallResponse_* benchmarks with small responses (1KB)
func BenchmarkMiddleware_SmallResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 1024)
}

func BenchmarkMiddleware_SmallResponse_LevelHeaders(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 1024)
}

func BenchmarkMiddleware_SmallResponse_LevelRequest(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 1024)
}

func BenchmarkMiddleware_SmallResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 1024)
}

// BenchmarkMiddleware_MediumResponse_* benchmarks with medium responses (100KB)
func BenchmarkMiddleware_MediumResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 100*1024)
}

func BenchmarkMiddleware_MediumResponse_LevelHeaders(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 100*1024)
}

func BenchmarkMiddleware_MediumResponse_LevelRequest(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 100*1024)
}

func BenchmarkMiddleware_MediumResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 100*1024)
}

// BenchmarkMiddleware_LargeResponse_* benchmarks with large responses (1MB)
func BenchmarkMiddleware_LargeResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 1024*1024)
}

func BenchmarkMiddleware_LargeResponse_LevelHeaders(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 1024*1024)
}

func BenchmarkMiddleware_LargeResponse_LevelRequest(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 1024*1024)
}

func BenchmarkMiddleware_LargeResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 1024*1024)
}

// BenchmarkMiddleware_VeryLargeResponse_* benchmarks with very large responses (10MB)
func BenchmarkMiddleware_VeryLargeResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelNull, 10*1024*1024)
}

func BenchmarkMiddleware_VeryLargeResponse_LevelHeaders(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelHeaders, 10*1024*1024)
}

func BenchmarkMiddleware_VeryLargeResponse_LevelRequest(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequest, 10*1024*1024)
}

func BenchmarkMiddleware_VeryLargeResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithSize(b, auditlogv1.LevelRequestResponse, 10*1024*1024)
}

// Helper function for response size benchmarks
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

// BenchmarkMiddleware_WithRequestBody benchmarks with both request and response bodies
func BenchmarkMiddleware_WithRequestBody_Small(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024, 1024)
}

func BenchmarkMiddleware_WithRequestBody_Medium(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 100*1024, 100*1024)
}

func BenchmarkMiddleware_WithRequestBody_Large(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024*1024, 1024*1024)
}

// BenchmarkMiddleware_ExtraLargeResponse_* benchmarks with very large responses (100MB)
func BenchmarkMiddleware_ExtraLargeResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelNull, 10*1024*1024, 100*1024*1024)
}

func BenchmarkMiddleware_ExtraLargeResponse_LevelHeaders(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelHeaders, 10*1024*1024, 100*1024*1024)
}

func BenchmarkMiddleware_ExtraLargeResponse_LevelRequest(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 10*1024*1024, 100*1024*1024)
}

func BenchmarkMiddleware_ExtraLargeResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 10*1024*1024, 100*1024*1024)
}

// BenchmarkMiddleware_GigabyteResponse_* benchmarks with gigabyte responses (1GB, 2GB)
// These simulate the user-reported scenario with 1.7GB heap usage
func BenchmarkMiddleware_GigabyteResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelNull, 1024*1024, 1024*1024*1024)
}

func BenchmarkMiddleware_GigabyteResponse_LevelHeaders(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelHeaders, 1024*1024, 1024*1024*1024)
}

func BenchmarkMiddleware_GigabyteResponse_LevelRequest(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequest, 1024*1024, 1024*1024*1024)
}

func BenchmarkMiddleware_GigabyteResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024*1024, 1024*1024*1024)
}

// BenchmarkMiddleware_2GBResponse_* benchmarks with 2GB responses
// Useful for stress testing and confirming the issue scales with size
func BenchmarkMiddleware_2GBResponse_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelNull, 1024*1024, 2*1024*1024*1024)
}

func BenchmarkMiddleware_2GBResponse_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareWithRequestBody(b, auditlogv1.LevelRequestResponse, 1024*1024, 2*1024*1024*1024)
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

// BenchmarkMiddleware_Concurrent tests concurrent request handling
func BenchmarkMiddleware_Concurrent_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareConcurrent(b, auditlogv1.LevelNull, 100*1024)
}

func BenchmarkMiddleware_Concurrent_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareConcurrent(b, auditlogv1.LevelRequestResponse, 100*1024)
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

// BenchmarkMiddleware_StreamingResponse_* benchmarks simulate reverse proxy streaming scenario
// These write responses in chunks like httputil.ReverseProxy.copyBuffer does
func BenchmarkMiddleware_StreamingResponse_100MB_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelNull, 100*1024*1024, 32*1024)
}

func BenchmarkMiddleware_StreamingResponse_100MB_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelRequestResponse, 100*1024*1024, 32*1024)
}

func BenchmarkMiddleware_StreamingResponse_1GB_LevelMetadata(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelNull, 1024*1024*1024, 32*1024)
}

func BenchmarkMiddleware_StreamingResponse_1GB_LevelRequestResponse(b *testing.B) {
	benchmarkMiddlewareStreaming(b, auditlogv1.LevelRequestResponse, 1024*1024*1024, 32*1024)
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
