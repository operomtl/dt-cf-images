//go:build conformance

package conformance

import (
	"net/http"
	"testing"
)

func TestStats_ResponseShape(t *testing.T) {
	status, raw := doJSON(t, "GET", apiURL("/v1/stats"), nil)
	if status != http.StatusOK {
		t.Fatalf("stats returned %d", status)
	}

	assertEnvelopeShape(t, raw)

	result := assertField[map[string]any](t, raw, "result")
	count := assertField[map[string]any](t, result, "count")
	assertField[float64](t, count, "current")
	assertField[float64](t, count, "allowed")
}
