// Command api is the Go API Gateway entrypoint — see
// docs/02-system-architecture.md#go-api. Responsible for auth, CRUD, upload
// triggering, chat streaming, and WebSocket status updates.
// Never does embeddings/chunking/ML compute directly.
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

	authapp "course-assistant/internal/application/auth"
	chatapp "course-assistant/internal/application/chat"
	courseapp "course-assistant/internal/application/course"
	projectapp "course-assistant/internal/application/project"
	uploadapp "course-assistant/internal/application/upload"
	"course-assistant/internal/config"
	"course-assistant/internal/infrastructure/id"
	"course-assistant/internal/infrastructure/llm"
	"course-assistant/internal/infrastructure/postgres"
	qdrantinfra "course-assistant/internal/infrastructure/qdrant"
	r2infra "course-assistant/internal/infrastructure/r2"
	redisinfra "course-assistant/internal/infrastructure/redis"
	httpapi "course-assistant/internal/interfaces/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("api: config error: %v", err)
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
	if cfg.R2.AccountID != "" {
		objects, err = r2infra.NewStore(cfg.R2.AccountID, cfg.R2.AccessKeyID, cfg.R2.SecretAccessKey, cfg.R2.Bucket)
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

	// ── HTTP wiring ────────────────────────────────────────────────────────
	deps := httpapi.Dependencies{
		JWTSigningKey:  cfg.Auth.JWTSigningKey,
		AuthHandler:    httpapi.NewAuthHandler(authService),
		ProjectHandler: httpapi.NewProjectHandler(projectService),
		CourseHandler:  httpapi.NewCourseHandler(courseService),
		UploadHandler:  httpapi.NewUploadHandler(uploadService),
		ChatHandler:    httpapi.NewChatHandler(chatService),
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
}
