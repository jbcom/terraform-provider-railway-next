// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"testing"
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
