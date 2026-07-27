// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"testing"
	"time"
)

func TestBucketEnvironmentRegistration(t *testing.T) {
	t.Parallel()

	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		return map[string]any{"data": map[string]any{
			"environment": map[string]any{
				"id": "environment", "projectId": "project", "configEtag": "etag",
				"config": map[string]any{
					"buckets": map[string]any{
						"bucket-id": map[string]any{
							"region": "ams", "isCreated": true, "isDeleted": false,
						},
					},
				},
			},
		}}, nil
	})
	resource := &Bucket{client: testClient(t, "https://fixture.invalid/graphql", doer)}
	registered, err := resource.isRegistered(context.Background(), "environment", "bucket-id")
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("bucket exists at project scope but registration was not detected")
	}
}

func TestDeletedBucketIsNotRegistered(t *testing.T) {
	t.Parallel()

	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		return map[string]any{"data": map[string]any{
			"environment": map[string]any{
				"id": "environment", "projectId": "project", "configEtag": "etag",
				"config": map[string]any{
					"buckets": map[string]any{
						"bucket-id": map[string]any{
							"region": "ams", "isCreated": true, "isDeleted": true,
						},
					},
				},
			},
		}}, nil
	})
	resource := &Bucket{client: testClient(t, "https://fixture.invalid/graphql", doer)}
	registered, err := resource.isRegistered(context.Background(), "environment", "bucket-id")
	if err != nil {
		t.Fatal(err)
	}
	if registered {
		t.Fatal("isDeleted bucket must be absent for Terraform lifecycle purposes")
	}
}

func TestBucketRegistrationWaitsForEnvironmentConvergence(t *testing.T) {
	t.Parallel()

	listRequests := 0
	doer := fixtureDoer(func(request graphqlFixtureRequest) (map[string]any, error) {
		switch request.OperationName {
		case "ListProjectBuckets":
			listRequests++
			edges := []any{}
			if listRequests >= 2 {
				edges = append(edges, map[string]any{"node": map[string]any{
					"id": "bucket-id", "name": "cache", "projectId": "project",
					"createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z",
				}})
			}
			return map[string]any{"data": map[string]any{
				"project": map[string]any{"buckets": map[string]any{"edges": edges}},
			}}, nil
		case "GetEnvironmentConfiguration":
			return map[string]any{"data": map[string]any{
				"environment": map[string]any{
					"id": "environment", "projectId": "project", "configEtag": "etag",
					"config": map[string]any{"buckets": map[string]any{
						"bucket-id": map[string]any{
							"region": "ams", "isCreated": true, "isDeleted": false,
						},
					}},
				},
			}}, nil
		default:
			return nil, context.Canceled
		}
	})
	resource := &Bucket{client: testClient(t, "https://fixture.invalid/graphql", doer)}

	bucket, err := resource.waitForBucketRegistration(
		context.Background(),
		"project",
		"environment",
		"cache",
		time.Millisecond,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bucket == nil || bucket.Id != "bucket-id" || listRequests < 2 {
		t.Fatalf("bucket = %#v after %d list requests, want convergence after a retry", bucket, listRequests)
	}
}
