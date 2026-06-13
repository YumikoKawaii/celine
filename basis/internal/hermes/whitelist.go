package hermes

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Whitelist is the set of email addresses allowed to obtain a Celine token.
// It guards Metabole (the OAuth exchange) — the one place an email is first
// known — so an unlisted account never gets a JWT and so never reaches the
// agent loop or burns Anthropic usage.
//
// A Whitelist with no entries is "open" (Allowed reports true for everyone),
// which is the dev default when no file is configured.
type Whitelist struct {
	emails map[string]struct{}
}

// NewWhitelist builds a Whitelist from an explicit set of emails (normalised
// to lower-case). Mainly for tests and wiring; production loads from YAML.
func NewWhitelist(emails []string) *Whitelist {
	m := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		if n := normalizeEmail(e); n != "" {
			m[n] = struct{}{}
		}
	}
	return &Whitelist{emails: m}
}

// whitelistFile is the on-disk YAML shape: a single `emails` list.
type whitelistFile struct {
	Emails []string `yaml:"emails"`
}

// LoadWhitelist reads a YAML whitelist from path. An empty path yields an open
// whitelist (everyone allowed) — the explicit dev default. A configured path
// that cannot be read or parsed is a hard error: failing open on a misconfigured
// access list would silently expose the assistant.
func LoadWhitelist(path string) (*Whitelist, error) {
	if strings.TrimSpace(path) == "" {
		return NewWhitelist(nil), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("whitelist: read %s: %w", path, err)
	}
	var f whitelistFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("whitelist: parse %s: %w", path, err)
	}
	return NewWhitelist(f.Emails), nil
}

// Open reports whether the whitelist admits everyone (no entries configured).
func (w *Whitelist) Open() bool { return len(w.emails) == 0 }

// Allowed reports whether email may obtain a token. An open whitelist allows all.
func (w *Whitelist) Allowed(email string) bool {
	if w.Open() {
		return true
	}
	_, ok := w.emails[normalizeEmail(email)]
	return ok
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}
