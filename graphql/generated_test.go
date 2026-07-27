// SPDX-License-Identifier: MPL-2.0

package graphql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	genqlient "github.com/Khan/genqlient/graphql"
)

func TestGeneratedBucketRequestConstruction(t *testing.T) {
	t.Parallel()

	var captured struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}
	doer := graphQLDoer(func(req *http.Request) (*http.Response, error) {
		defer req.Body.Close()
		if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
			return nil, err
		}
		body := `{"data":{"bucketCreate":{"id":"bucket-id","name":"cache","projectId":"project","groupId":null,"createdAt":"2026-01-01T00:00:00Z","deletedAt":null}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	gql := genqlient.NewClient("https://fixture.invalid/graphql", doer)
	name := "cache"
	result, err := CreateBucket(context.Background(), gql, BucketCreateInput{
		ProjectId: "project",
		Name:      &name,
	})
	if err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if result.BucketCreate.Id != "bucket-id" {
		t.Fatalf("bucket ID = %q", result.BucketCreate.Id)
	}
	if captured.OperationName != "CreateBucket" {
		t.Fatalf("operationName = %q", captured.OperationName)
	}
	if !strings.Contains(captured.Query, "mutation CreateBucket") ||
		!strings.Contains(captured.Query, "$input: BucketCreateInput!") {
		t.Fatalf("unexpected generated query: %s", captured.Query)
	}
	input, ok := captured.Variables["input"].(map[string]any)
	if !ok {
		t.Fatalf("input variables = %#v", captured.Variables["input"])
	}
	if input["projectId"] != "project" || input["name"] != "cache" {
		t.Fatalf("input = %#v", input)
	}
	if environment, exists := input["environmentId"]; exists && environment != nil {
		t.Fatalf("detached bucket environmentId = %#v, want null or omitted", environment)
	}
}

func TestHTTP200GraphQLErrorsAreErrors(t *testing.T) {
	t.Parallel()

	doer := graphQLDoer(func(req *http.Request) (*http.Response, error) {
		body := `{"errors":[{"message":"rate limited","extensions":{"code":"RATE_LIMITED"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	gql := genqlient.NewClient("https://fixture.invalid/graphql", doer)
	_, err := GetProject(context.Background(), gql, "project")
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("error = %v, want GraphQL error from HTTP 200 response", err)
	}
}

type graphQLDoer func(*http.Request) (*http.Response, error)

func (f graphQLDoer) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
