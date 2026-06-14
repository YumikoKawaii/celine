package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Server holds all configuration for the celine RPC server binary.
type Server struct {
	Addr             string        // CELINE_ADDR, default ":8080"
	DBDsn            string        // CELINE_DB_DSN, required
	RedisAddr        string        // CELINE_REDIS_ADDR, required
	AnthropicKey     string        // ANTHROPIC_API_KEY, required
	Model            string        // CELINE_MODEL, optional (defaults inside llm package)
	MaxTokens        int64         // CELINE_MAX_TOKENS, optional (default 8192)
	JWTSecret        string        // CELINE_JWT_SECRET, optional — empty = dev mode, no auth enforced
	TokenTTL         time.Duration // CELINE_TOKEN_TTL, default 72h; parsed as Go duration string
	GoogleClientID   string        // GOOGLE_CLIENT_ID, optional — empty = Google OAuth disabled
	GoogleSecret     string        // GOOGLE_CLIENT_SECRET, required when GoogleClientID is set
	WhitelistPath    string        // CELINE_WHITELIST, optional — YAML list of allowed emails; empty = open access
	BraveAPIKey      string        // BRAVE_API_KEY, optional — empty = web_search returns error
	OllamaURL        string        // OLLAMA_URL, default "http://localhost:11434" — embeds the recall query (§12.5)
	DebounceDuration time.Duration // CELINE_DEBOUNCE, default 45s; fallback flush — the client normally triggers Sigao when the user stops typing
	DevAnon          bool          // CELINE_DEV_ANON, default false — local-only: bypass auth, treat every request as the anon prosopon (id=2). Never enable in prod.
}

// Worker holds all configuration for the graphe worker binary.
type Worker struct {
	DBDsn     string // CELINE_DB_DSN, required
	RedisAddr string // CELINE_REDIS_ADDR, required
	OllamaURL string // OLLAMA_URL, default "http://localhost:11434"
}

func LoadServer() (Server, error) {
	ttl, err := parseDuration("CELINE_TOKEN_TTL", 72*time.Hour)
	if err != nil {
		return Server{}, err
	}
	maxTokens, err := parseInt64("CELINE_MAX_TOKENS", 0)
	if err != nil {
		return Server{}, err
	}
	debounce, err := parseDuration("CELINE_DEBOUNCE", 45*time.Second)
	if err != nil {
		return Server{}, err
	}
	c := Server{
		Addr:             getenv("CELINE_ADDR", ":8080"),
		DBDsn:            os.Getenv("CELINE_DB_DSN"),
		RedisAddr:        os.Getenv("CELINE_REDIS_ADDR"),
		AnthropicKey:     os.Getenv("ANTHROPIC_API_KEY"),
		Model:            os.Getenv("CELINE_MODEL"),
		MaxTokens:        maxTokens,
		JWTSecret:        os.Getenv("CELINE_JWT_SECRET"),
		TokenTTL:         ttl,
		GoogleClientID:   os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:     os.Getenv("GOOGLE_CLIENT_SECRET"),
		WhitelistPath:    os.Getenv("CELINE_WHITELIST"),
		BraveAPIKey:      os.Getenv("BRAVE_API_KEY"),
		OllamaURL:        getenv("OLLAMA_URL", "http://localhost:11434"),
		DebounceDuration: debounce,
		DevAnon:          os.Getenv("CELINE_DEV_ANON") == "true",
	}
	return c, c.validate()
}

func LoadWorker() (Worker, error) {
	c := Worker{
		DBDsn:     os.Getenv("CELINE_DB_DSN"),
		RedisAddr: os.Getenv("CELINE_REDIS_ADDR"),
		OllamaURL: getenv("OLLAMA_URL", "http://localhost:11434"),
	}
	return c, c.validate()
}

func (c *Server) validate() error {
	var missing []string
	require := func(v, name string) {
		if v == "" {
			missing = append(missing, name)
		}
	}
	require(c.DBDsn, "CELINE_DB_DSN")
	require(c.RedisAddr, "CELINE_REDIS_ADDR")
	require(c.AnthropicKey, "ANTHROPIC_API_KEY")
	if c.GoogleClientID != "" {
		require(c.GoogleSecret, "GOOGLE_CLIENT_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c *Worker) validate() error {
	var missing []string
	if c.DBDsn == "" {
		missing = append(missing, "CELINE_DB_DSN")
	}
	if c.RedisAddr == "" {
		missing = append(missing, "CELINE_REDIS_ADDR")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return d, nil
}

func parseInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return n, nil
}
