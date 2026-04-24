package config

import (
	"testing"

	"github.com/runixio/runix/pkg/types"
)

func TestResolveExtends_Simple(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "base", Entrypoint: "./base", Runtime: "go", Env: map[string]string{"A": "1"}},
		{Name: "child", Extends: "base", Env: map[string]string{"B": "2"}},
	}

	result, err := ResolveExtends(configs)
	if err != nil {
		t.Fatal(err)
	}

	child := result[1]
	if child.Entrypoint != "./base" {
		t.Fatalf("expected inherited entrypoint, got %s", child.Entrypoint)
	}
	if child.Env["A"] != "1" {
		t.Fatal("expected inherited env A")
	}
	if child.Env["B"] != "2" {
		t.Fatal("expected own env B")
	}
	if child.Extends != "" {
		t.Fatal("extends should be cleared after resolution")
	}
}

func TestResolveExtends_Override(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "base", Entrypoint: "./base", Runtime: "go"},
		{Name: "child", Extends: "base", Runtime: "python", Entrypoint: "app.py"},
	}

	result, err := ResolveExtends(configs)
	if err != nil {
		t.Fatal(err)
	}

	child := result[1]
	if child.Runtime != "python" {
		t.Fatalf("expected overridden runtime, got %s", child.Runtime)
	}
	if child.Entrypoint != "app.py" {
		t.Fatalf("expected overridden entrypoint, got %s", child.Entrypoint)
	}
}

func TestResolveExtends_MissingParent(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "child", Extends: "nonexistent"},
	}

	_, err := ResolveExtends(configs)
	if err == nil {
		t.Fatal("expected error for missing parent")
	}
}

func TestResolveExtends_Circular(t *testing.T) {
	configs := []types.ProcessConfig{
		{Name: "a", Extends: "b"},
		{Name: "b", Extends: "a"},
	}

	_, err := ResolveExtends(configs)
	if err == nil {
		t.Fatal("expected error for circular extends")
	}
}
