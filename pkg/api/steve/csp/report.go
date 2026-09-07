// Package csp receives Content-Security-Policy violation reports from browsers
// and records them in the Rancher log, so that a policy can be evaluated in
// report-only mode before it is enforced.
package csp

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// ReportPath must match the report-uri in the ui-csp-policy setting.
	ReportPath = "/v1/csp-report"

	// maxBodySize caps what a browser can make Rancher parse. Reports are a few
	// hundred bytes and browsers truncate long URLs themselves.
	maxBodySize = 64 * 1024

	// maxFieldLength caps a single field in the logged line.
	maxFieldLength = 512

	// logInterval is how long the same violation is suppressed for. Every page
	// load in every open browser reports the same handful of violations, and
	// the endpoint has to be unauthenticated because browsers report from the
	// login page too.
	logInterval = 10 * time.Minute

	// maxTrackedViolations bounds the suppression table. Past this the table is
	// dropped, which at worst logs a violation again earlier than intended.
	maxTrackedViolations = 1000
)

// report is the body browsers POST to a report-uri endpoint.
type report struct {
	CSPReport struct {
		DocumentURI        string `json:"document-uri"`
		BlockedURI         string `json:"blocked-uri"`
		ViolatedDirective  string `json:"violated-directive"`
		EffectiveDirective string `json:"effective-directive"`
		SourceFile         string `json:"source-file"`
		LineNumber         int    `json:"line-number"`
		Disposition        string `json:"disposition"`
	} `json:"csp-report"`
}

// Register adds the violation report endpoint to router.
func Register(router *http.ServeMux) {
	router.Handle(ReportPath, &handler{seen: map[string]time.Time{}})
}

type handler struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func (h *handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var parsed report
	if err := json.NewDecoder(http.MaxBytesReader(rw, req.Body, maxBodySize)).Decode(&parsed); err != nil {
		http.Error(rw, "invalid report", http.StatusBadRequest)
		return
	}

	violation := parsed.CSPReport
	directive := violation.EffectiveDirective
	if directive == "" {
		directive = violation.ViolatedDirective
	}
	if directive == "" {
		http.Error(rw, "invalid report", http.StatusBadRequest)
		return
	}

	// Everything in a report is browser-supplied and this endpoint is
	// unauthenticated, so the fields are truncated and logged quoted.
	if h.shouldLog(directive+" "+violation.BlockedURI+" "+violation.DocumentURI, time.Now()) {
		logrus.Infof("csp violation: directive=%q blocked=%q document=%q source=%q line=%d disposition=%q",
			truncate(directive),
			truncate(violation.BlockedURI),
			truncate(violation.DocumentURI),
			truncate(violation.SourceFile),
			violation.LineNumber,
			truncate(violation.Disposition))
	}

	rw.WriteHeader(http.StatusNoContent)
}

// shouldLog reports whether key has not been logged within logInterval.
func (h *handler) shouldLog(key string, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if last, ok := h.seen[key]; ok && now.Sub(last) < logInterval {
		return false
	}

	if len(h.seen) >= maxTrackedViolations {
		h.seen = map[string]time.Time{}
	}
	h.seen[key] = now

	return true
}

func truncate(value string) string {
	if len(value) <= maxFieldLength {
		return value
	}
	return value[:maxFieldLength] + "..."
}
