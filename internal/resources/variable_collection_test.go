// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/micah5/terraform-provider-railway-next/internal/client"
)

func TestVariableCollectionManyVariablesOneApply(t *testing.T) {
	t.Parallel()

	var applyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var request graphqlFixtureRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.OperationName {
		case "GetService":
			writeJSON(w, serviceFixture())
		case "GetEnvironmentConfiguration":
			writeJSON(w, environmentFixture())
		case "ApplyEnvironmentChangeSet":
			applyCalls.Add(1)
			input := request.Variables["input"]
			encoded, _ := json.Marshal(input)
			if got := strings.Count(string(encoded), `"kind":"variable.set"`); got != 50 {
				t.Errorf("variable.set count = %d, want 50", got)
			}
			writeJSON(w, applyFixture())
		default:
			http.Error(w, "unexpected "+request.OperationName, http.StatusBadRequest)
		}
	}))
	defer server.Close()

	r := &VariableCollection{client: testClient(t, server.URL, nil)}
	state := variableCollectionFixture()
	desired := make(map[string]string, 50)
	for i := 0; i < 50; i++ {
		desired[string(rune('A'+i))] = "value"
	}
	var diagnostics diag.Diagnostics
	if !r.apply(context.Background(), &state, map[string]string{}, desired, &diagnostics) {
		t.Fatalf("apply failed: %v", diagnostics)
	}
	if got := applyCalls.Load(); got != 1 {
		t.Fatalf("environmentApplyChangeSet calls = %d, want 1", got)
	}
}

func TestAmbiguousMutationSuccessReconcilesByRead(t *testing.T) {
	t.Parallel()

	var applyCalls atomic.Int32
	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		switch request.OperationName {
		case "GetService":
			return serviceFixture(), nil
		case "GetEnvironmentConfiguration":
			return environmentFixture(), nil
		case "ApplyEnvironmentChangeSet":
			applyCalls.Add(1)
			// Models the server committing the mutation and the response timing
			// out before reaching the provider.
			return nil, context.DeadlineExceeded
		case "ListVariables":
			return map[string]any{"data": map[string]any{
				"variables": map[string]string{"DATABASE_URL": "${{Postgres.DATABASE_URL}}"},
			}}, nil
		default:
			return nil, errors.New("unexpected operation " + request.OperationName)
		}
	})
	r := &VariableCollection{client: testClient(t, "https://fixture.invalid/graphql", doer)}
	state := variableCollectionFixture()
	after := map[string]string{"DATABASE_URL": "${{Postgres.DATABASE_URL}}"}
	var diagnostics diag.Diagnostics
	if !r.apply(context.Background(), &state, map[string]string{}, after, &diagnostics) {
		t.Fatalf("ambiguous apply was not reconciled: %v", diagnostics)
	}
	if diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	if got := applyCalls.Load(); got != 1 {
		t.Fatalf("apply calls = %d, want 1", got)
	}
}

func variableCollectionFixture() variableCollectionModel {
	return variableCollectionModel{
		ProjectID:     types.StringValue("project"),
		EnvironmentID: types.StringValue("environment"),
		ServiceID:     types.StringValue("service"),
	}
}

func serviceFixture() map[string]any {
	return map[string]any{"data": map[string]any{
		"service": map[string]any{
			"id": "service", "name": "api", "projectId": "project",
			"deletedAt": nil, "repoTriggers": map[string]any{"edges": []any{}},
		},
		"environment": map[string]any{
			"config": map[string]any{}, "serviceInstances": map[string]any{"edges": []any{}},
		},
		"limitOverride": nil,
	}}
}

func environmentFixture() map[string]any {
	return map[string]any{"data": map[string]any{
		"environment": map[string]any{
			"id": "environment", "projectId": "project",
			"config": map[string]any{}, "configEtag": "etag-1",
		},
	}}
}

func applyFixture() map[string]any {
	return map[string]any{"data": map[string]any{
		"environmentApplyChangeSet": map[string]any{
			"id": "operation", "status": "applied", "deploymentId": "deployment",
			"stagedPatchId": nil, "diagnostics": []any{}, "changes": []any{},
		},
	}}
}

type graphqlFixtureRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

type fixtureDoer func(graphqlFixtureRequest) (map[string]any, error)

func (f fixtureDoer) Do(req *http.Request) (*http.Response, error) {
	defer req.Body.Close()
	var request graphqlFixtureRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		return nil, err
	}
	payload, err := f(request)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(strings.NewReader(string(encoded))),
		ContentLength: int64(len(encoded)),
		Request:       req,
	}, nil
}

func testClient(t *testing.T, endpoint string, doer client.HTTPDoer) *client.Client {
	t.Helper()
	result, err := client.New(client.Config{
		Token: "fixture", TokenType: client.TokenTypeAccount, Endpoint: endpoint,
		Timeout: time.Second, MaxRetries: 1, HTTPDoer: doer, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func writeJSON(w io.Writer, value any) {
	_ = json.NewEncoder(w).Encode(value)
}
