// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"

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

// TestLogRequestKeepsValuesOutOfDebug is the privacy half of the contract.
//
// Variables carry whatever the practitioner configured, so the values belong at
// TRACE — a deliberate opt-in — while DEBUG carries only the operation and the
// keys. A provider that spilled configuration values at DEBUG would be one
// nobody could safely run with `TF_LOG=DEBUG` in CI.
func TestLogRequestKeepsValuesOutOfDebug(t *testing.T) {

	logged := captureProviderLog(t, hclog.Debug, func(ctx context.Context) {
		logRequest(ctx, requestEnvelope{
			OperationName: "UpdateServiceInstance",
			Variables:     map[string]any{"serviceId": "a-configured-value"},
		})
	})

	if strings.Contains(logged, "a-configured-value") {
		t.Errorf("a variable VALUE reached the debug log:\n%s", logged)
	}
	if !strings.Contains(logged, "serviceId") {
		t.Errorf("the variable KEY should still be at debug:\n%s", logged)
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
