package supervisor

import (
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestResolveStartOrder_Simple(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "api", DependsOn: []string{"db"}},
		{Name: "db"},
	}

	result, err := ResolveStartOrder(configs)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Name != "db" {
		t.Fatalf("expected db first, got %s", result[0].Name)
	}
	if result[1].Name != "api" {
		t.Fatalf("expected api second, got %s", result[1].Name)
	}
}

func TestResolveStartOrder_Cycle(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "a", DependsOn: []string{"b"}},
		{Name: "b", DependsOn: []string{"a"}},
	}

	_, err := ResolveStartOrder(configs)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestResolveStartOrder_MissingDep(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "api", DependsOn: []string{"missing"}},
	}

	_, err := ResolveStartOrder(configs)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestResolveStartOrder_Priority(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "worker", Priority: 10},
		{Name: "api", Priority: 1},
		{Name: "db", Priority: 0},
	}

	result, err := ResolveStartOrder(configs)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Name != "db" {
		t.Fatalf("expected db (priority 0) first, got %s", result[0].Name)
	}
	if result[1].Name != "api" {
		t.Fatalf("expected api (priority 1) second, got %s", result[1].Name)
	}
}

func TestResolveStartOrder_Complex(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "web", DependsOn: []string{"api"}},
		{Name: "api", DependsOn: []string{"db", "cache"}},
		{Name: "db"},
		{Name: "cache"},
	}

	result, err := ResolveStartOrder(configs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result))
	}
	// web must come after api, api must come after db and cache.
	names := make(map[string]int)
	for i, r := range result {
		names[r.Name] = i
	}
	if names["web"] <= names["api"] {
		t.Fatal("web must come after api")
	}
	if names["api"] <= names["db"] || names["api"] <= names["cache"] {
		t.Fatal("api must come after db and cache")
	}
}
