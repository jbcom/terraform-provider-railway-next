package changeset

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegisterBucketFixture(t *testing.T) {
	t.Parallel()
	payload, err := RegisterBucket("langgraph-pa-cache", "ams").JSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"changes":[{"kind":"resource.create","path":"resources.bucket.langgraph-pa-cache","summary":"Create bucket langgraph-pa-cache","severity":"safe","deployEffect":"none","address":"bucket.langgraph-pa-cache","resource":{"address":"bucket.langgraph-pa-cache","type":"bucket","name":"langgraph-pa-cache","config":{"region":"ams"}}}],"diagnostics":[]}`
	if string(payload) != want {
		t.Fatalf("bucket change set mismatch\n got: %s\nwant: %s", payload, want)
	}
}

func TestVariableCollectionIsOneChangeSet(t *testing.T) {
	t.Parallel()
	set := VariableCollection(
		"api",
		map[string]string{"REMOVE": "old", "KEEP": "same"},
		map[string]string{"ADD": "new", "KEEP": "same"},
	)
	if len(set.Changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(set.Changes))
	}
	payload, err := set.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(payload), `"version":1`) != 1 {
		t.Fatalf("expected one logical change set: %s", payload)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := len(decoded["changes"].([]any)); got != 2 {
		t.Fatalf("got %d serialized changes, want 2", got)
	}
}

func TestPostgresFixtureMatchesOfficialDatabaseNode(t *testing.T) {
	t.Parallel()

	payload, err := CreatePostgres("Postgres", "18", "ams").JSON()
	if err != nil {
		t.Fatal(err)
	}
	var set Set
	if err := json.Unmarshal(payload, &set); err != nil {
		t.Fatal(err)
	}
	if len(set.Changes) != 1 || set.Changes[0].Kind != "resource.create" {
		t.Fatalf("changes = %#v", set.Changes)
	}
	var database DatabaseResource
	if err := json.Unmarshal(set.Changes[0].Resource, &database); err != nil {
		t.Fatal(err)
	}
	if database.Engine != "postgres" ||
		database.Image != "ghcr.io/railwayapp-templates/postgres-ssl:18" ||
		database.DefaultMountPath != "/var/lib/postgresql/data" ||
		database.Output != "DATABASE_URL" {
		t.Fatalf("database resource = %#v", database)
	}
	if database.Deploy == nil ||
		database.Deploy.MultiRegionConfig["ams"]["numReplicas"] != 1 {
		t.Fatalf("database deploy = %#v", database.Deploy)
	}
}
