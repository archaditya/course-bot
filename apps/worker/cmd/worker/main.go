// Command worker is the Go background worker entrypoint. It runs all pipeline
// stages concurrently (each in its own goroutine), consuming from Redis
// Streams. See docs/04-indexing-pipeline.md for the full pipeline.
//
// Run: go run cmd/worker/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"archadilm/internal/application/worker"
	"archadilm/internal/config"
	"archadilm/internal/infrastructure/id"
	"archadilm/internal/infrastructure/llm"
	pginfra "archadilm/internal/infrastructure/postgres"
	qdrantinfra "archadilm/internal/infrastructure/qdrant"
	r2infra "archadilm/internal/infrastructure/r2"
	redisinfra "archadilm/internal/infrastructure/redis"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("worker: config error: %v", err)
	}

	// ── Storage connections ────────────────────────────────────────────────
	db, err := pginfra.Open(cfg.Database.URL)
	if err != nil {
		log.Fatalf("worker: postgres: %v", err)
	}
	defer db.Close()

	queue, err := redisinfra.NewQueue(cfg.Redis.URL)
	if err != nil {
		log.Fatalf("worker: redis: %v", err)
	}
	defer queue.Close()

	vectors, err := qdrantinfra.NewStore(cfg.Qdrant.URL, cfg.Qdrant.APIKey)
	if err != nil {
		log.Fatalf("worker: qdrant: %v", err)
	}

	var objects *r2infra.Store
	r2Target := cfg.R2.AccountID
	if r2Target == "" {
		r2Target = cfg.R2.Endpoint
	}
	if r2Target != "" {
		objects, err = r2infra.NewStore(r2Target, cfg.R2.AccessKeyID, cfg.R2.SecretAccessKey, cfg.R2.Bucket)
		if err != nil {
			log.Fatalf("worker: r2: %v", err)
		}
	} else {
		log.Println("worker: R2 not configured — uploads will fail (set R2_ACCOUNT_ID or R2_ENDPOINT)")
	}

	// ── AI Service client ──────────────────────────────────────────────────
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8000"
	}
	aiClient := llm.NewClient(aiServiceURL)

	// ── Repositories ──────────────────────────────────────────────────────
	courses := pginfra.NewCourseRepository(db)
	documents := pginfra.NewDocumentRepository(db)
	jobs := pginfra.NewJobRepository(db)
	chunks := pginfra.NewChunkRepository(db)

	ids := id.UUIDGenerator{}

	// ── Wire workers ──────────────────────────────────────────────────────
	manifestWorker := worker.NewManifestWorker(courses, jobs, documents, queue, ids)
	parserWorker := worker.NewParserWorker(courses, jobs, documents, objects, queue, ids, aiClient)
	chunkWorker := worker.NewChunkWorker(courses, jobs, documents, objects, queue, ids)
	metadataWorker := worker.NewMetadataWorker(courses, jobs, queue, ids, aiClient)
	embeddingWorker := worker.NewEmbeddingWorker(courses, jobs, chunks, vectors, queue, aiClient)

	// ── Start all stages ──────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errs := make(chan error, 5)
	go func() { errs <- manifestWorker.Run(ctx) }()
	go func() { errs <- parserWorker.Run(ctx) }()
	go func() { errs <- chunkWorker.Run(ctx) }()
	go func() { errs <- metadataWorker.Run(ctx) }()
	go func() { errs <- embeddingWorker.Run(ctx) }()

	log.Printf("worker: all pipeline stages started (env=%s, ai-service=%s)",
		cfg.ServiceEnv, aiServiceURL)

	// ── Graceful shutdown ─────────────────────────────────────────────────
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		log.Println("worker: shutting down")
		cancel()
	case err := <-errs:
		if err != nil {
			log.Printf("worker: stage error: %v", err)
		}
		cancel()
	}
}
