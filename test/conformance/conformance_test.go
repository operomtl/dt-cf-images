//go:build conformance

package conformance

import (
	"os"
	"testing"
)

var (
	baseURL   string
	authToken string
	accountID string
	isRealAPI bool
)

func TestMain(m *testing.M) {
	baseURL = os.Getenv("DT_TARGET")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	authToken = os.Getenv("DT_AUTH_TOKEN")
	if authToken == "" {
		authToken = "test-token"
	}
	accountID = os.Getenv("DT_ACCOUNT_ID")
	if accountID == "" {
		accountID = "test-account"
	}
	isRealAPI = os.Getenv("DT_REAL_API") == "true"
	os.Exit(m.Run())
}
