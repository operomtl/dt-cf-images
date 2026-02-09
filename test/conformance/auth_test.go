//go:build conformance

package conformance

import (
	"net/http"
	"testing"
)

func TestAuth_NoToken_401(t *testing.T) {
	req, err := http.NewRequest("GET", apiURL("/v1"), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	// Deliberately omit Authorization header
	resp := doRequest(t, req)
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestAuth_WrongToken_401(t *testing.T) {
	req, err := http.NewRequest("GET", apiURL("/v1"), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer wrong-token-xxx")
	resp := doRequest(t, req)
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestAuth_ValidToken_200(t *testing.T) {
	status, _ := doJSON(t, "GET", apiURL("/v1"), nil)
	if status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}
}
