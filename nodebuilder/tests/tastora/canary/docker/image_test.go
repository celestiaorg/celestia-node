package docker

import (
	"context"
	"strings"
	"testing"
)

func TestLocalImageIDMismatchAllocatesNothing(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	cfg := config()
	cfg.Profile.Image = "sha256:" + strings.Repeat("b", 64)
	s, err := New(context.Background(), cfg)
	if s != nil {
		defer func() { _ = s.Cleanup(context.Background()) }()
	}
	if err == nil {
		t.Fatal("different actual local image ID accepted")
	}
	for _, r := range f.requests {
		if strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "DELETE ") {
			t.Fatalf("unverified local ID mutated daemon: %s", r)
		}
	}
}

func TestLocalImageRejectionsNeverPullOrAllocate(t *testing.T) {
	for _, kind := range []string{"missing", "source", "empty source", "short", "uppercase", "tag", "whitespace"} {
		t.Run(kind, func(t *testing.T) {
			f := &mobyFixture{imageMissing: kind == "missing"}
			fixture(t, f)
			cfg := config()
			cfg.Profile.Image = "sha256:" + strings.Repeat("a", 64)
			switch kind {
			case "source":
				cfg.Profile.SourceCommit = "wrong"
			case "empty source":
				cfg.Profile.SourceCommit = ""
			case "short":
				cfg.Profile.Image = "sha256:abc"
			case "uppercase":
				cfg.Profile.Image = "sha256:" + strings.Repeat("A", 64)
			case "tag":
				cfg.Profile.Image = "node:local"
			case "whitespace":
				cfg.Profile.Image += " "
			}
			s, err := New(context.Background(), cfg)
			if s != nil {
				defer func() { _ = s.Cleanup(context.Background()) }()
			}
			if err == nil {
				t.Fatal("unverified local image accepted")
			}
			for _, r := range f.requests {
				if strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "DELETE ") {
					t.Fatalf("unverified image mutated daemon: %s", r)
				}
			}
		})
	}
}

func TestAlreadyLocalExactImageIDNeverPulls(t *testing.T) {
	f := &mobyFixture{wrongDigest: true, pullError: true}
	fixture(t, f)
	cfg := config()
	cfg.Profile.Image = "sha256:" + strings.Repeat("a", 64)
	s, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	if s.imageID != cfg.Profile.Image || f.create["Image"] != cfg.Profile.Image {
		t.Fatal("local image ID not bound exactly")
	}
	for _, r := range f.requests {
		if r == "POST /images/create" {
			t.Fatal("already-local image was pulled")
		}
	}
}
