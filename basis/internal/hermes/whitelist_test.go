package hermes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWhitelistOpenWhenEmpty(t *testing.T) {
	w := NewWhitelist(nil)
	if !w.Open() {
		t.Fatal("empty whitelist should be open")
	}
	if !w.Allowed("anyone@example.com") {
		t.Fatal("open whitelist should allow everyone")
	}
}

func TestWhitelistAllowedNormalised(t *testing.T) {
	w := NewWhitelist([]string{"  Yumiko@Example.com  ", "other@gmail.com"})
	if w.Open() {
		t.Fatal("non-empty whitelist must not be open")
	}
	cases := map[string]bool{
		"yumiko@example.com": true,  // lower-cased match
		"YUMIKO@EXAMPLE.COM": true,  // caller case ignored
		" other@gmail.com ":  true,  // caller whitespace trimmed
		"stranger@evil.com":  false, // not listed
		"":                   false,
	}
	for email, want := range cases {
		if got := w.Allowed(email); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", email, got, want)
		}
	}
}

func TestLoadWhitelistEmptyPathIsOpen(t *testing.T) {
	w, err := LoadWhitelist("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !w.Open() {
		t.Fatal("empty path should yield an open whitelist")
	}
}

func TestLoadWhitelistFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "whitelist.yaml")
	if err := os.WriteFile(path, []byte("emails:\n  - a@b.com\n  - C@D.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := LoadWhitelist(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Open() {
		t.Fatal("loaded list should not be open")
	}
	if !w.Allowed("a@b.com") || !w.Allowed("c@d.com") {
		t.Fatal("listed emails should be allowed")
	}
	if w.Allowed("x@y.com") {
		t.Fatal("unlisted email should be denied")
	}
}

func TestLoadWhitelistMissingFileErrors(t *testing.T) {
	if _, err := LoadWhitelist(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("a configured-but-unreadable whitelist must be a hard error, not fail-open")
	}
}
