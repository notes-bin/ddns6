package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ExposesCounters(t *testing.T) {
	ResetForTest()
	IncIPv6Fetch(true)
	IncSync(true)
	MarkSuccess()
	IncSyncSkipped()

	rr := httptest.NewRecorder()
	Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	s := string(body)
	for _, want := range []string{
		`ddns6_sync_total{result="ok"} 1`,
		`ddns6_ipv6_fetch_total{result="ok"} 1`,
		`ddns6_sync_skipped_total 1`,
		`ddns6_last_success_timestamp `,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("metrics 缺少 %q\n%s", want, s)
		}
	}
}
