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
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("config: failed to load .env file: %w", err)
	}
	
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

func parseIntDefault(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	return strconv.Atoi(v)
}
