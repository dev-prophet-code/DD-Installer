package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev-prophet-code/dd-installer/internal/install"
)

func release(t *testing.T, tag string) string {
	t.Helper()
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "VERSION"), []byte(tag+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(d, "install.sh"), []byte("#!/bin/sh\n"), 0o755)
	return d
}

func TestRecordsTestedAndLatest(t *testing.T) {
	work := t.TempDir()
	manifest := filepath.Join(work, "upstream", "upstream.json")
	src := filepath.Join(work, "upstream.go")
	_ = os.WriteFile(src, []byte("package install\nconst (\n\tUpstreamTag = \"v0.0.1\"\n\tUpstreamTreeSHA256 = \"x\"\n)\n"), 0o644)

	// "latest" before any "tested" release exists is refused
	if err := run("v1.0.0", false, false, release(t, "v1.0.0"), manifest, src); err == nil {
		t.Fatal("latest without tested must be refused")
	}
	// tested: manifest AND the built-in pin are written
	r1 := release(t, "v1.0.0")
	if err := run("v1.0.0", false, true, r1, manifest, src); err != nil {
		t.Fatal(err)
	}
	want, _ := install.TreeHash(r1)
	b, _ := os.ReadFile(manifest)
	m, err := install.ParseManifest(b)
	if err != nil || m.Tested.Tag != "v1.0.0" || m.Tested.TreeSHA256 != want || m.Latest != m.Tested {
		t.Fatalf("manifest: %+v %v", m, err)
	}
	code, _ := os.ReadFile(src)
	if !strings.Contains(string(code), `UpstreamTag = "v1.0.0"`) || !strings.Contains(string(code), want) {
		t.Fatalf("built-in pin not rewritten:\n%s", code)
	}
	// latest: only the "latest" entry changes
	if err := run("v1.1.0", false, false, release(t, "v1.1.0"), manifest, src); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(manifest)
	m, _ = install.ParseManifest(b)
	if m.Latest.Tag != "v1.1.0" || m.Tested.Tag != "v1.0.0" {
		t.Fatalf("tested must stay untouched: %+v", m)
	}
	// the VERSION file inside the release must match the tag
	if err := run("v1.2.0", false, false, release(t, "v9.9.9"), manifest, src); err == nil {
		t.Fatal("a release whose VERSION differs from the tag must be refused")
	}
	// hostile tags never reach the manifest
	if err := run("v1.0.0'; id; '", false, false, release(t, "v1.0.0"), manifest, src); err == nil {
		t.Fatal("hostile tag accepted")
	}
}
