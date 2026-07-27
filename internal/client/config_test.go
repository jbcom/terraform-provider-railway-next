// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"
	"time"
)

func TestNormalizeEnvironmentAndDefaults(t *testing.T) {
	t.Setenv("RAILWAY_API_TOKEN", "account-token")
	t.Setenv("RAILWAY_TOKEN", "fallback-token")
	t.Setenv("RAILWAY_GRAPHQL_ENDPOINT", "http://127.0.0.1:1234/graphql")

	got, err := normalize(Config{})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got.Token != "account-token" {
		t.Fatalf("token = %q", got.Token)
	}
	if got.TokenType != TokenTypeAccount {
		t.Fatalf("token type = %q", got.TokenType)
	}
	if got.Timeout != 30*time.Second || got.MaxRetries != 3 {
		t.Fatalf("defaults = timeout %s retries %d", got.Timeout, got.MaxRetries)
	}
}

func TestNormalizeRejectsInsecureRemoteEndpoint(t *testing.T) {
	_, err := normalize(Config{Token: "token", Endpoint: "http://example.com/graphql"})
	if err == nil {
		t.Fatal("expected insecure endpoint error")
	}
}
