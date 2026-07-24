// Command api is the archadiLM API Gateway entrypoint.
// Responsible for auth, CRUD, upload triggering, chat streaming, and
// status polling. Never does embeddings/chunking/ML compute directly.
//
// Run with: go run cmd/api/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authapp "archadilm/internal/application/auth"
	chatapp "archadilm/internal/application/chat"
	courseapp "archadilm/internal/application/course"
	projectapp "archadilm/internal/application/project"
	uploadapp "archadilm/internal/application/upload"
	"archadilm/internal/config"
	"archadilm/internal/infrastructure/id"
	"archadilm/internal/infrastructure/llm"
	"archadilm/internal/infrastructure/observability"
	"archadilm/internal/infrastructure/postgres"
	qdrantinfra "archadilm/internal/infrastructure/qdrant"
	r2infra "archadilm/internal/infrastructure/r2"
	redisinfra "archadilm/internal/infrastructure/redis"
	httpapi "archadilm/internal/interfaces/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("api: config error: %v", err)
	}

	// Initialize Sentry
    if cfg.Sentry.DSN != "" {
        if err := observability.InitSentry(cfg.Sentry.DSN, cfg.ServiceEnv); err != nil {
            log.Printf("api: sentry init failed: %v", err)
        }
        defer observability.Flush()
    }

	db, err := postgres.Open(cfg.Database.URL)
	if err != nil {
		log.Fatalf("api: database connection error: %v", err)
	}
	defer db.Close()

	ids := id.UUIDGenerator{}

	migrationsDir := os.Getenv("MIGRATIONS_PATH")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := postgres.RunMigrations(db, migrationsDir); err != nil {
		log.Fatalf("api: migration error: %v", err)
	}
	log.Println("api: migrations up to date")

	// ── Infrastructure ─────────────────────────────────────────────────────
	queue, err := redisinfra.NewQueue(cfg.Redis.URL)
	if err != nil {
		log.Fatalf("api: redis: %v", err)
	}
	defer queue.Close()

	aiClient := llm.NewClient(cfg.AIServiceURL)

	var vectors *qdrantinfra.Store
	if cfg.Qdrant.URL != "" {
		vectors, err = qdrantinfra.NewStore(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
		if err != nil {
			log.Printf("api: qdrant unavailable: %v (chat search disabled)", err)
		}
	}

	var objects *r2infra.Store
	r2Target := cfg.R2.AccountID
	if r2Target == "" {
		r2Target = cfg.R2.Endpoint
	}
	if r2Target != "" {
		objects, err = r2infra.NewStore(r2Target, cfg.R2.AccessKeyID, cfg.R2.SecretAccessKey, cfg.R2.Bucket)
		if err != nil {
			log.Printf("api: r2 unavailable: %v (uploads disabled)", err)
		}
	}

	// ── Repositories ──────────────────────────────────────────────────────
	users := postgres.NewUserRepository(db)
	workspaces := postgres.NewWorkspaceRepository(db)
	refreshTokens := postgres.NewRefreshTokenRepository(db)
	projects := postgres.NewProjectRepository(db)
	courses := postgres.NewCourseRepository(db)
	lessons := postgres.NewLessonRepository(db)
	documents := postgres.NewDocumentRepository(db)
	jobs := postgres.NewJobRepository(db)
	conversations := postgres.NewConversationRepository(db)
	messages := postgres.NewMessageRepository(db)
	citations := postgres.NewCitationRepository(db)
	chunks := postgres.NewChunkRepository(db)

	// ── Application services ───────────────────────────────────────────────
	authService := authapp.NewService(users, workspaces, refreshTokens, cfg.Auth.JWTSigningKey)
	projectService := projectapp.NewService(projects)
	courseService := courseapp.NewService(courses)

	var uploadService *uploadapp.Service
	if objects != nil && queue != nil {
		uploadService = uploadapp.NewService(courses, lessons, documents, jobs, objects, queue, ids)
	}

	var chatService *chatapp.Service
	if vectors != nil {
		chatService = chatapp.NewService(
			conversations, messages, citations, projects,
			courses, chunks,
			aiClient, vectors, aiClient,
			cfg.Flags.MaxEvaluatorRetries, ids,
		)
	}

	statusHandler := httpapi.NewStatusHandler(courses, jobs)

	// ── HTTP wiring ────────────────────────────────────────────────────────
	deps := httpapi.Dependencies{
		JWTSigningKey:  cfg.Auth.JWTSigningKey,
		AuthHandler:    httpapi.NewAuthHandler(authService),
		ProjectHandler: httpapi.NewProjectHandler(projectService),
		CourseHandler:  httpapi.NewCourseHandler(courseService),
		UploadHandler:  httpapi.NewUploadHandler(uploadService),
		ChatHandler:    httpapi.NewChatHandler(chatService),
		StatusHandler:  statusHandler,

		// Add health check dependencies
		RedisClient:  queue.(*redisinfra.Queue).Client(),
		PostgresDB:   db,
		QdrantClient: vectors.(*qdrantinfra.Store).Client(),
		AIClient:     aiClient,
	}
	router := httpapi.NewRouter(deps)

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 90 * time.Second, // longer for SSE streaming
	}

	go func() {
		log.Printf("api: listening on %s (env=%s)", addr, cfg.ServiceEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api: server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("api: shutdown error: %v", err)
	}
	log.Println("api: shut down cleanly")

	// Wrap router with Sentry handler
    sentryHandler := observability.GetSentryHandler()
    srv.Handler = sentryHandler.Handle(router)
}
