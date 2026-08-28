// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	railway "github.com/micah5/terraform-provider-railway-next/graphql"

	"github.com/hashicorp/terraform-plugin-log/tfsdklog"
)

// TestLogRequestNamesEmptyVariables asserts the log line that would have made
// the empty-id bug a five-minute diagnosis instead of an afternoon.
//
// Railway reports a mutation carrying `serviceId: ""` as `Not Authorized`,
// naming neither the variable nor the resource. A key list alone does not help,
// because the key IS present — it is the value that is missing. So the empty
// ones are called out by name.
func TestLogRequestNamesEmptyVariables(t *testing.T) {

	logged := captureProviderLog(t, hclog.Trace, func(ctx context.Context) {
		logRequest(ctx, requestEnvelope{
			OperationName: "UpdateServiceInstance",
			Variables: map[string]any{
				"environmentId": "environment-fixture",
				"serviceId":     "",
			},
		})
	})
	if !strings.Contains(logged, "UpdateServiceInstance") {
		t.Errorf("the operation name is missing, so the log cannot say which call failed:\n%s", logged)
	}
	if !strings.Contains(logged, "railway_empty_variables") || !strings.Contains(logged, "serviceId") {
		t.Errorf("serviceId was empty and the log did not say so:\n%s", logged)
	}
}

// TestLogRequestRedactsEveryVariableAtEveryLevel exercises the real logger
// path with the shape sent by railway_secret plus values under harmless-looking
// names. Redaction must be structural, not a list of guessed sensitive keys.
func TestLogRequestRedactsEveryVariableAtEveryLevel(t *testing.T) {
	valueWOFixture := "fixture-write-only-value-must-not-appear"
	arbitraryFixture := "fixture-arbitrary-payload-must-not-appear"
	nestedKeyFixture := "fixture-practitioner-key-must-not-appear"
	levels := []hclog.Level{hclog.Trace, hclog.Debug, hclog.Info, hclog.Warn, hclog.Error}

	for _, level := range levels {
		logged := captureProviderLog(t, level, func(ctx context.Context) {
			logRequest(ctx, requestEnvelope{
				OperationName: "UpsertVariable",
				Variables: map[string]any{
					"input": map[string]any{
						"name":  "DATABASE_URL",
						"value": valueWOFixture,
						"metadata": map[string]any{
							nestedKeyFixture: arbitraryFixture,
						},
					},
				},
			})
		})

		for _, forbidden := range []string{valueWOFixture, arbitraryFixture, nestedKeyFixture, "DATABASE_URL"} {
			if strings.Contains(logged, forbidden) {
				t.Errorf("level %s logged protected GraphQL data %q:\n%s", level, forbidden, logged)
			}
		}
		if level <= hclog.Debug {
			if !strings.Contains(logged, "UpsertVariable") || !strings.Contains(logged, "input") {
				t.Errorf("level %s lost useful operation metadata:\n%s", level, logged)
			}
		}
		if level == hclog.Trace &&
			(!strings.Contains(logged, "railway_redacted_type") || !strings.Contains(logged, "railway_redacted_fields")) {
			t.Errorf("trace log lost redacted structural metadata:\n%s", logged)
		}
	}
}

// TestRailwaySecretValueWONeverReachesTrace exercises the generated operation
// used by railway_secret. The Terraform value_wo attribute becomes input.Value
// in this request, so this guards the real resource-to-transport boundary.
func TestRailwaySecretValueWONeverReachesTrace(t *testing.T) {
	valueWOFixture := "fixture-real-value-wo-path-must-not-appear"
	doer := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":{"variableUpsert":true}}`)),
		}, nil
	})
	railwayClient, err := New(Config{
		Token:     "fixture-token-not-logged",
		TokenType: TokenTypeAccount,
		Endpoint:  "https://fixture.invalid/graphql",
		Version:   "test",
		HTTPDoer:  doer,
	})
	if err != nil {
		t.Fatal(err)
	}
	serviceID := "fixture-service-id"
	skipDeploys := false

	logged := captureProviderLog(t, hclog.Trace, func(ctx context.Context) {
		_, _ = railway.UpsertVariable(ctx, railwayClient.GraphQL(), railway.VariableUpsertInput{
			ProjectId:     "fixture-project-id",
			EnvironmentId: "fixture-environment-id",
			ServiceId:     &serviceID,
			Name:          "FIXTURE_SECRET",
			Value:         valueWOFixture,
			SkipDeploys:   &skipDeploys,
		})
	})

	for _, forbidden := range []string{
		valueWOFixture, "fixture-token-not-logged", "fixture-project-id",
		"fixture-environment-id", serviceID, "FIXTURE_SECRET",
	} {
		if strings.Contains(logged, forbidden) {
			t.Errorf("railway_secret request logged protected data %q:\n%s", forbidden, logged)
		}
	}
	if !strings.Contains(logged, "UpsertVariable") || !strings.Contains(logged, "input") {
		t.Errorf("railway_secret request lost operation metadata:\n%s", logged)
	}
}

func TestRedactVariablesDoesNotMutateInput(t *testing.T) {
	input := map[string]any{
		"input": map[string]any{
			"value": "fixture-protected-value",
			"list":  []any{"fixture-list-value", nil},
		},
	}
	want := map[string]any{
		"input": map[string]any{
			"value": "fixture-protected-value",
			"list":  []any{"fixture-list-value", nil},
		},
	}

	redactedValue := RedactVariables(input)
	encoded, err := json.Marshal(redactedValue)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"fixture-protected-value", "fixture-list-value", `"value"`, `"list"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("redacted structure contains protected input %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"input"`) || !strings.Contains(string(encoded), redacted) {
		t.Errorf("redacted structure lost top-level metadata or marker: %s", encoded)
	}
	if !reflect.DeepEqual(input, want) {
		t.Errorf("RedactVariables mutated its input: got %#v want %#v", input, want)
	}
}

// TestTransportLoggingExcludesResponseAndErrorData covers the two exits from
// the HTTP transport. Request metadata is logged before the call; response
// bodies, response headers and returned errors must never be added to that log.
func TestTransportLoggingExcludesResponseAndErrorData(t *testing.T) {
	requestFixture := "fixture-request-payload-must-not-appear"
	responseFixture := "fixture-response-payload-must-not-appear"
	errorFixture := "fixture-transport-error-must-not-appear"

	for _, test := range []struct {
		name string
		doer HTTPDoer
	}{
		{
			name: "response",
			doer: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"X-Fixture": []string{responseFixture}},
					Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"` + responseFixture + `"}]}`)),
				}, nil
			}),
		},
		{
			name: "error",
			doer: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New(errorFixture)
			}),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			doer := newRailwayDoer(Config{
				Token:      "fixture-token-not-logged",
				TokenType:  TokenTypeAccount,
				Version:    "test",
				Timeout:    time.Second,
				MaxRetries: 0,
				HTTPDoer:   test.doer,
			})
			body, err := json.Marshal(map[string]any{
				"query":         "mutation UpsertVariable { ok }",
				"operationName": "UpsertVariable",
				"variables":     map[string]any{"input": map[string]any{"value": requestFixture}},
			})
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1/graphql", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}

			logged := captureProviderLog(t, hclog.Trace, func(ctx context.Context) {
				response, _ := doer.Do(req.WithContext(ctx))
				if response != nil {
					_ = response.Body.Close()
				}
			})

			for _, forbidden := range []string{requestFixture, responseFixture, errorFixture, "fixture-token-not-logged"} {
				if strings.Contains(logged, forbidden) {
					t.Errorf("transport %s path logged protected data %q:\n%s", test.name, forbidden, logged)
				}
			}
			if !strings.Contains(logged, "UpsertVariable") || !strings.Contains(logged, "input") {
				t.Errorf("transport %s path lost operation metadata:\n%s", test.name, logged)
			}
		})
	}
}

// TestRequestEnvelopeDecodesOperationAndVariables guards the decode itself.
//
// The envelope originally captured only `query`, which is why the transport
// could not say what it was sending. If these tags drift, every log line above
// silently degrades to `(anonymous)` with no variables and the tests still pass
// unless this one is here.
func TestRequestEnvelopeDecodesOperationAndVariables(t *testing.T) {
	t.Parallel()

	var envelope requestEnvelope
	body := `{"query":"mutation X { a }","operationName":"X","variables":{"serviceId":"s-1"}}`
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatal(err)
	}

	if envelope.OperationName != "X" {
		t.Errorf("operationName = %q, want X", envelope.OperationName)
	}
	if envelope.Variables["serviceId"] != "s-1" {
		t.Errorf("variables = %v, want serviceId s-1", envelope.Variables)
	}
}

// captureProviderLog runs body with a real provider root logger at the given
// level and returns what it wrote.
//
// **IT CAPTURES STDERR RATHER THAN INJECTING A BUFFER**, because `tfsdklog` in
// this version exposes no output option — the logger it builds writes to
// stderr, which is how Terraform actually collects provider logs. Capturing the
// real sink tests the path an operator gets from `TF_LOG`, instead of a
// parallel one that could drift from it.
//
// These tests therefore cannot be `t.Parallel()`: os.Stderr is process-wide.
func captureProviderLog(t *testing.T, level hclog.Level, body func(context.Context)) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	original := os.Stderr
	os.Stderr = write
	defer func() { os.Stderr = original }()

	ctx := tfsdklog.NewRootProviderLogger(context.Background(),
		tfsdklog.WithLevel(level), tfsdklog.WithoutLocation())
	body(ctx)

	if err := write.Close(); err != nil {
		t.Fatal(err)
	}

	var captured bytes.Buffer
	if _, err := io.Copy(&captured, read); err != nil {
		t.Fatal(err)
	}
	return captured.String()
}
