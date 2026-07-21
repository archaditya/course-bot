// Package config is the only place allowed to call os.Getenv — see
// docs/09-deployment.md#configuration-strategy. Every other package receives
// a typed *Config (or a narrower sub-struct) at construction time.
// Validation happens at Load(), not on first use: a missing required value
// fails the process at boot, in seconds, rather than surfacing as a
// confusing error on the first real request.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AIServiceURL   string
	Database       DatabaseConfig
	Redis          RedisConfig
	R2             R2Config
	Qdrant         QdrantConfig
	Auth           AuthConfig
	Providers      ProvidersConfig
	Flags          FeatureFlags
	ServiceEnv     string // local | staging | prod
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

type QdrantConfig struct {
	URL    string
	APIKey string
}

type AuthConfig struct {
	JWTSigningKey       string
	GoogleOAuthClientID string
	GoogleOAuthSecret   string
}

// ProvidersConfig selects which concrete implementation backs each Provider
// interface from docs/02-system-architecture.md#provider-abstraction. This
// is itself a config value (e.g. LLM_PROVIDER=openai), never a compile-time
// choice.
type ProvidersConfig struct {
	LLMProvider       string
	EmbeddingProvider string
	RerankerProvider  string
	GuardrailProvider string
}

// FeatureFlags toggle behavior per-environment without a deploy — see
// docs/09-deployment.md#configuration-strategy.
type FeatureFlags struct {
	GuardrailsEnabled   bool
	MaxEvaluatorRetries int
}

// requiredVars lists env vars that must be non-empty for the process to
// boot. Kept as data (not scattered `if x == "" { panic }` calls) so the set
// of "what's actually required" is auditable in one place.
var requiredVars = []string{
	"POSTGRES_URL",
	"REDIS_URL",
	"JWT_SIGNING_KEY",
}

// Load reads and validates configuration from the process environment.
// Call it once at startup in each apps/* entrypoint (api, worker,
// ai-service's Go-side wiring); nothing below application/interfaces should
// read the environment directly.
func Load() (*Config, error) {
	// A missing .env file is expected and fine in any environment where the
	// platform injects env vars directly (Docker, systemd, VPS) rather than
	// shipping a .env file — that's the normal case in production, per
	// docs/09-deployment.md#configuration-strategy. Only a real read error
	// (bad permissions, malformed file) should stop boot.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: failed to load .env file: %w", err)
	}

	normalizeEnvAliases()

	var missing []string
	for _, name := range requiredVars {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required environment variables: %s", strings.Join(missing, ", "))
	}

	maxRetries, err := parseIntDefault("MAX_EVALUATOR_RETRIES", 3)
	if err != nil {
		return nil, fmt.Errorf("config: MAX_EVALUATOR_RETRIES: %w", err)
	}

	cfg := &Config{
		ServiceEnv: envOrDefault("SERVICE_ENV", "local"),
		AIServiceURL: envOrDefault("AI_SERVICE_URL", "http://localhost:8000"),
		Database:   DatabaseConfig{URL: os.Getenv("POSTGRES_URL")},
		Redis:      RedisConfig{URL: os.Getenv("REDIS_URL")},
		R2: R2Config{
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Bucket:          os.Getenv("R2_BUCKET"),
		},
		Qdrant: QdrantConfig{
			URL:    os.Getenv("QDRANT_URL"),
			APIKey: os.Getenv("QDRANT_API_KEY"),
		},
		Auth: AuthConfig{
			JWTSigningKey:       os.Getenv("JWT_SIGNING_KEY"),
			GoogleOAuthClientID: os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
			GoogleOAuthSecret:   os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		},
		Providers: ProvidersConfig{
			LLMProvider:       envOrDefault("LLM_PROVIDER", "openai"),
			EmbeddingProvider: envOrDefault("EMBEDDING_PROVIDER", "openai"),
			RerankerProvider:  envOrDefault("RERANKER_PROVIDER", "openai"),
			GuardrailProvider: envOrDefault("GUARDRAIL_PROVIDER", "openai"),
		},
		Flags: FeatureFlags{
			GuardrailsEnabled:   envOrDefault("GUARDRAILS_ENABLED", "true") == "true",
			MaxEvaluatorRetries: maxRetries,
		},
	}
	return cfg, nil
}

func envOrDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// normalizeEnvAliases reconciles a couple of naming mismatches that have
// shown up between this package's canonical var names and deployment
// configs (e.g. docker-compose.prod.yml). It mutates the process
// environment once, at boot, so every other line in this file — and the
// requiredVars check above — can keep referring to a single canonical name.
func normalizeEnvAliases() {
	// DATABASE_URL -> POSTGRES_URL
	if os.Getenv("POSTGRES_URL") == "" {
		if v := os.Getenv("DATABASE_URL"); v != "" {
			os.Setenv("POSTGRES_URL", v)
		}
	}

	// R2_BUCKET_NAME -> R2_BUCKET
	if os.Getenv("R2_BUCKET") == "" {
		if v := os.Getenv("R2_BUCKET_NAME"); v != "" {
			os.Setenv("R2_BUCKET", v)
		}
	}

	// REDIS_URL without a scheme (e.g. "redis:6379") -> add redis:// so
	// go-redis's ParseURL doesn't reject it.
	if v := os.Getenv("REDIS_URL"); v != "" &&
		!strings.HasPrefix(v, "redis://") && !strings.HasPrefix(v, "rediss://") {
		os.Setenv("REDIS_URL", "redis://"+v)
	}
}

func parseIntDefault(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}
