package references

import "testing"

func TestExpressionAndHCLLiteral(t *testing.T) {
	t.Parallel()
	expression, err := Expression("langgraph-pa-cache", "ACCESS_KEY_ID")
	if err != nil {
		t.Fatal(err)
	}
	if want := "${{langgraph-pa-cache.ACCESS_KEY_ID}}"; expression != want {
		t.Fatalf("Expression() = %q, want %q", expression, want)
	}
	if got, want := HCLLiteral(expression), "$${{langgraph-pa-cache.ACCESS_KEY_ID}}"; got != want {
		t.Fatalf("HCLLiteral() = %q, want %q", got, want)
	}
}

func TestReferenceMapsNeverContainCredentials(t *testing.T) {
	t.Parallel()
	bucket := Bucket("cache")
	if got := bucket["SECRET_ACCESS_KEY"]; got != "${{cache.SECRET_ACCESS_KEY}}" {
		t.Fatalf("unexpected secret reference: %q", got)
	}
	postgres := Postgres("Postgres")
	if got := postgres["PGPASSWORD"]; got != "${{Postgres.PGPASSWORD}}" {
		t.Fatalf("unexpected password reference: %q", got)
	}
}
