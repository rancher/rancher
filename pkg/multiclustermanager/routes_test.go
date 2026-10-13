package multiclustermanager

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestReadOnlyStripsMethodOverride guards against the unauthenticated
// _method/action=remove override that let an anonymous GET be reinterpreted
// as a write against public settings (ui-pl, first-login, ...).
func TestReadOnlyStripsMethodOverride(t *testing.T) {
	tests := []struct {
		name      string
		rawQuery  string
		wantQuery string
	}{
		{
			name:      "strips _method override",
			rawQuery:  "_method=PUT",
			wantQuery: "",
		},
		{
			name:      "strips action=remove override",
			rawQuery:  "action=remove",
			wantQuery: "",
		},
		{
			name:      "strips both overrides together",
			rawQuery:  "_method=DELETE&action=remove",
			wantQuery: "",
		},
		{
			name:      "leaves unrelated query params untouched",
			rawQuery:  "foo=bar",
			wantQuery: "foo=bar",
		},
		{
			name:      "leaves requests with no query untouched",
			rawQuery:  "",
			wantQuery: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/v3/settings/ui-pl?"+tt.rawQuery, nil)
			rec := httptest.NewRecorder()

			readOnly(inner).ServeHTTP(rec, req)

			if gotQuery != tt.wantQuery {
				t.Fatalf("readOnly() passed query %q to handler, want %q", gotQuery, tt.wantQuery)
			}
		})
	}
}
