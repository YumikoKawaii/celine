package config

import (
	"fmt"
	"os"
	"strings"
)

// Server holds all configuration for the celine RPC server binary.
type Server struct {
	Addr           string // CELINE_ADDR, default ":8080"
	DBDsn          string // CELINE_DB_DSN, required
	RedisAddr      string // CELINE_REDIS_ADDR, required
	AnthropicKey   string // ANTHROPIC_API_KEY, required
	Model          string // CELINE_MODEL, optional (defaults inside llm package)
	JWTSecret      string // CELINE_JWT_SECRET, optional — empty = dev mode, no auth enforced
	GoogleClientID string // GOOGLE_CLIENT_ID, optional — empty = Google OAuth disabled
	GoogleSecret   string // GOOGLE_CLIENT_SECRET, required when GoogleClientID is set
}

// Worker holds all configuration for the graphe worker binary.
type Worker struct {
	DBDsn     string // CELINE_DB_DSN, required
	RedisAddr string // CELINE_REDIS_ADDR, required
	OllamaURL string // OLLAMA_URL, default "http://localhost:11434"
}

func LoadServer() (Server, error) {
	c := Server{
		Addr:           getenv("CELINE_ADDR", ":8080"),
		DBDsn:          os.Getenv("CELINE_DB_DSN"),
		RedisAddr:      os.Getenv("CELINE_REDIS_ADDR"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		Model:          os.Getenv("CELINE_MODEL"),
		JWTSecret:      os.Getenv("CELINE_JWT_SECRET"),
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
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
