package http

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
    
    goredis "github.com/redis/go-redis/v9"
    "github.com/jackc/pgx/v5/pgxpool"
    qdrant "github.com/qdrant/go-client/qdrant"
    
    "archadilm/internal/infrastructure/llm"
    "archadilm/internal/infrastructure/observability"
    "archadilm/internal/infrastructure/resilience"
)

const apiVersion = "0.1.0"

type healthzResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: errorBody{Code: code, Message: message}})
}

// Dependencies bundles everything the router needs to wire up route groups.
// Constructed once in cmd/api/main.go and passed in here so router.go stays
// the single place that knows how the HTTP surface maps to handlers.
type Dependencies struct {
	JWTSigningKey  string
	AuthHandler    *AuthHandler
	ProjectHandler *ProjectHandler
	CourseHandler  *CourseHandler
	UploadHandler  *UploadHandler
	ChatHandler    *ChatHandler
	StatusHandler  *StatusHandler

	RedisClient    interface{} // *goredis.Client
    PostgresDB     interface{} // *pgxpool.Pool
    QdrantClient   interface{} // *qdrant.Client
    AIClient       interface{} // *llm.Client
}

func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Security headers
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        
        next.ServeHTTP(w, r)
    })
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := map[string]interface{}{
        "parser_processing_time_ns":    observability.GlobalMetrics.ParserProcessingTime.Load(),
        "chunker_processing_time_ns":   observability.GlobalMetrics.ChunkerProcessingTime.Load(),
        "metadata_processing_time_ns":  observability.GlobalMetrics.MetadataProcessingTime.Load(),
        "embedding_processing_time_ns": observability.GlobalMetrics.EmbeddingProcessingTime.Load(),
        "parser_errors":                observability.GlobalMetrics.ParserErrors.Load(),
        "chunker_errors":               observability.GlobalMetrics.ChunkerErrors.Load(),
        "metadata_errors":              observability.GlobalMetrics.MetadataErrors.Load(),
        "embedding_errors":             observability.GlobalMetrics.EmbeddingErrors.Load(),
        "ai_service_latency_ns":        observability.GlobalMetrics.AIServiceLatency.Load(),
        "ai_service_errors":            observability.GlobalMetrics.AIServiceErrors.Load(),
        "ai_service_calls":             observability.GlobalMetrics.AIServiceCalls.Load(),
    }
    
    writeJSON(w, http.StatusOK, metrics)
}

func NewRouter(deps Dependencies) http.Handler {
    rateLimiter := NewRateLimiter(60, 10)
 
    public := http.NewServeMux()
    public.HandleFunc("GET /healthz", handleHealthz(deps))
    public.HandleFunc("GET /metrics", handleMetrics)
    
 
    deps.AuthHandler.Register(public)
 
    protected := http.NewServeMux()
    deps.ProjectHandler.Register(protected)
    deps.CourseHandler.Register(protected)
    deps.UploadHandler.Register(protected)
    deps.ChatHandler.Register(protected)
    deps.StatusHandler.Register(protected)
    protected.HandleFunc("GET /auth/me", deps.AuthHandler.me)
 
    top := http.NewServeMux()
    top.Handle("/", public)
 
    // Protected route prefixes — All require valid Bearer token + rate limit
    auth := RequireAuth(deps.JWTSigningKey)
    rateLimited := RateLimitMiddleware(rateLimiter)
    
    top.Handle("/auth/me", auth(protected))
    top.Handle("/projects", auth(rateLimited(protected)))  // ADD rateLimited
    top.Handle("/projects/", auth(rateLimited(protected)))  // ADD rateLimited
    top.Handle("/courses/", auth(rateLimited(protected)))  // ADD rateLimited
    top.Handle("/collections/", auth(rateLimited(protected)))  // ADD rateLimited
    top.Handle("/conversations", auth(rateLimited(protected)))  // ADD rateLimited
    top.Handle("/conversations/", auth(rateLimited(protected)))  // ADD rateLimited
    top.Handle("/chunks/", auth(rateLimited(protected)))  // ADD rateLimited
 
    return Recovery(SecurityHeaders(CORS(Logging(top))))
}

// CORS middleware for frontend development.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Workspace-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("api: %s %s", r.Method, r.URL.Path)

		next.ServeHTTP(w, r)

		log.Printf("api: %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// handleHealthz checks all critical dependencies and returns aggregated status
func handleHealthz(deps Dependencies) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel()
        
        checks := []struct {
            name  string
            check func(context.Context) error
        }{
            {
                name: "redis",
                check: func(ctx context.Context) error {
                    if deps.RedisClient == nil {
                        return fmt.Errorf("redis client not configured")
                    }
                    return deps.RedisClient.(*goredis.Client).Ping(ctx).Err()
                },
            },
            {
                name: "postgres",
                check: func(ctx context.Context) error {
                    if deps.PostgresDB == nil {
                        return fmt.Errorf("postgres not configured")
                    }
                    return deps.PostgresDB.(*pgxpool.Pool).Ping(ctx).Err()
                },
            },
            {
                name: "qdrant",
                check: func(ctx context.Context) error {
                    if deps.QdrantClient == nil {
                        return fmt.Errorf("qdrant not configured")
                    }
                    _, err := deps.QdrantClient.(*qdrant.Client).HealthCheck(ctx)
                    return err
                },
            },
            {
                name: "ai-service",
                check: func(ctx context.Context) error {
                    if deps.AIClient == nil {
                        return fmt.Errorf("ai service client not configured")
                    }
                    // Check if circuit breaker is not open
                    client := deps.AIClient.(*llm.Client)
                    if client.embedCB.State() == resilience.StateOpen {
                        return fmt.Errorf("circuit breaker open")
                    }
                    return nil
                },
            },
        }
        
        var unhealthy []string
        for _, c := range checks {
            if err := c.check(ctx); err != nil {
                unhealthy = append(unhealthy, c.name)
            }
        }
        
        if len(unhealthy) > 0 {
            writeJSON(w, http.StatusServiceUnavailable, map[string]any{
                "status":   "unhealthy",
                "unhealthy": unhealthy,
            })
            return
        }
        
        writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
    }
}
