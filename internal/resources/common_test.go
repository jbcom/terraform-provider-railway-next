// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestEnvironmentChangeSetsAreSerializedPerEnvironment(t *testing.T) {
	t.Parallel()

	environmentID := "environment-lock-fixture"
	unlockFirst := lockEnvironmentChangeSet(environmentID)
	acquiredSecond := make(chan struct{})
	releaseSecond := make(chan struct{})

	go func() {
		unlockSecond := lockEnvironmentChangeSet(environmentID)
		close(acquiredSecond)
		<-releaseSecond
		unlockSecond()
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("second change set acquired the same environment lock concurrently")
	case <-time.After(25 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-acquiredSecond:
		close(releaseSecond)
	case <-time.After(time.Second):
		t.Fatal("second change set did not acquire the environment lock after release")
	}
}

func TestEnvironmentChangeSetRetriesStaleBaseWithFreshPreview(t *testing.T) {
	t.Parallel()

	var etagReads, previews, applies int
	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		switch request.OperationName {
		case "GetEnvironmentConfiguration":
			etagReads++
			return map[string]any{"data": map[string]any{
				"environment": map[string]any{
					"id": "environment", "projectId": "project",
					"config": map[string]any{}, "configEtag": "etag-" + string(rune('0'+etagReads)),
				},
			}}, nil
		case "PreviewEnvironmentChangeSet":
			previews++
			return previewFixture(request.Variables["input"]), nil
		case "ApplyEnvironmentChangeSet":
			applies++
			wantETag := "etag-" + string(rune('0'+applies))
			if request.Variables["baseConfigEtag"] != wantETag {
				t.Errorf("attempt %d baseConfigEtag = %v, want %q", applies, request.Variables["baseConfigEtag"], wantETag)
			}
			if applies == 1 {
				return nil, gqlerror.List{&gqlerror.Error{
					Message:    "The environment changed since this plan was computed.",
					Extensions: map[string]any{"code": "STALE_ENVIRONMENT_BASE"},
				}}
			}
			return applyFixture(), nil
		default:
			return nil, errors.New("unexpected operation " + request.OperationName)
		}
	})

	raw := json.RawMessage(`{"version":4,"changes":[{"kind":"bucket.register"}]}`)
	result, err := applyEnvironmentChangeSet(
		context.Background(),
		testClient(t, "https://fixture.invalid/graphql", doer).GraphQL(),
		"environment",
		raw,
		"Terraform: register bucket cache",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.EnvironmentApplyChangeSet.Status != "applied" {
		t.Fatalf("apply status = %q", result.EnvironmentApplyChangeSet.Status)
	}
	if etagReads != 2 || previews != 2 || applies != 2 {
		t.Fatalf("calls: config=%d preview=%d apply=%d, want 2 each", etagReads, previews, applies)
	}
}

func TestEnvironmentChangeSetDoesNotRetryUnrelatedMutationError(t *testing.T) {
	t.Parallel()

	applies := 0
	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		switch request.OperationName {
		case "GetEnvironmentConfiguration":
			return environmentFixture(), nil
		case "PreviewEnvironmentChangeSet":
			return previewFixture(request.Variables["input"]), nil
		case "ApplyEnvironmentChangeSet":
			applies++
			return nil, gqlerror.List{&gqlerror.Error{
				Message:    "invalid change",
				Extensions: map[string]any{"code": "BAD_REQUEST"},
			}}
		default:
			return nil, errors.New("unexpected operation " + request.OperationName)
		}
	})

	_, err := applyEnvironmentChangeSet(
		context.Background(),
		testClient(t, "https://fixture.invalid/graphql", doer).GraphQL(),
		"environment",
		json.RawMessage(`{"version":4,"changes":[]}`),
		"Terraform: test",
	)
	if err == nil {
		t.Fatal("expected mutation error")
	}
	if applies != 1 {
		t.Fatalf("apply calls = %d, want 1", applies)
	}
}
