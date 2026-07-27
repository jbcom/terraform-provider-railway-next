// Command schemafetch snapshots Railway's public GraphQL schema for genqlient.
//
// It intentionally has no authentication support: schema generation must never
// require a developer token or risk persisting one in command history.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/suessflorian/gqlfetch"
)

const defaultEndpoint = "https://backboard.railway.com/graphql/v2"

func main() {
	endpoint := flag.String("endpoint", defaultEndpoint, "GraphQL endpoint")
	output := flag.String("output", "graphql/schema.graphql", "schema output path")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema, err := gqlfetch.BuildClientSchema(ctx, *endpoint, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch Railway schema: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create schema directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, []byte(schema), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write Railway schema: %v\n", err)
		os.Exit(1)
	}
}
