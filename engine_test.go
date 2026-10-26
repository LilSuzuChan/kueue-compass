package main

import (
	"os"
	"strings"
	"testing"
)

func TestFixture(t *testing.T) {
	r := Report{Findings: []Finding{}}
	var before, objects []Obj
	var e error
	objects, e = read("examples/waiting.json")
	if e != nil {
		t.Fatal(e)
	}
	diagnose(&r, objects, before, "default", "demo", "client", "backend", 80)
	found := false
	for _, f := range r.Findings {
		if f.ID == "request-over-nominal-quota" {
			found = true
		}
		if len(f.Evidence) == 0 {
			t.Fatalf("missing evidence: %+v", f)
		}
	}
	if !found {
		t.Fatalf("expected diagnostic absent: %+v", r.Findings)
	}
}
func TestMalformedInput(t *testing.T) {
	p := t.TempDir() + "/bad.yaml"
	if e := os.WriteFile(p, []byte("x: ["), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := read(p); e == nil {
		t.Fatal("malformed YAML accepted")
	}
}
func TestMissingSubject(t *testing.T) {
	r := Report{}
	missing(&r, "Pod", "default", "absent")
	if r.Findings[0].Classification != "unknown" {
		t.Fatal("missing subject is not unknown")
	}
}
func TestNamespaceIsolation(t *testing.T) {
	objects := []Obj{{"kind": "Pod", "metadata": Obj{"name": "same", "namespace": "other"}}}
	if find(objects, "Pod", "default", "same") != nil {
		t.Fatal("cross namespace match")
	}
}
func TestSecretNotEchoed(t *testing.T) {
	r := Report{}
	secret := Obj{"kind": "Secret", "metadata": Obj{"name": "secret"}, "data": Obj{"password": "sensitive-marker"}}
	diagnose(&r, []Obj{secret}, nil, "default", "demo", "client", "backend", 80)
	for _, f := range r.Findings {
		if strings.Contains(f.Summary, "sensitive-marker") {
			t.Fatal("secret exposed")
		}
	}
}
