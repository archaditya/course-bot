// Command seed_admin seeds or promotes an admin user in PostgreSQL.
// Usage:
//   go run ./scripts/seed_admin.go -email admin@coursebot.com -password "Admin@123456" -name "Admin User"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"archadilm/internal/config"
	"archadilm/internal/domain/entities"
	"archadilm/internal/infrastructure/id"
	"archadilm/internal/infrastructure/postgres"
	"archadilm/internal/infrastructure/security"
)

func main() {
	emailFlag := flag.String("email", "admin@archadi.dev", "Admin email address")
	passFlag := flag.String("password", "Admin@123456", "Admin password")
	nameFlag := flag.String("name", "Admin User", "Admin full name")
	flag.Parse()

	email := strings.ToLower(strings.TrimSpace(*emailFlag))
	password := strings.TrimSpace(*passFlag)
	name := strings.TrimSpace(*nameFlag)

	if email == "" || password == "" || name == "" {
		log.Fatalf("email, password, and name are required")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: loading config from env: %v (will use POSTGRES_URL env if present)", err)
	}

	dbURL := cfg.Database.URL
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://postgres:564321@localhost:5432/courseassistant?sslmode=disable"
	}

	db, err := postgres.Open(dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Ensure migrations are up to date
	migrationsDir := os.Getenv("MIGRATIONS_PATH")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := postgres.RunMigrations(db, migrationsDir); err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	ctx := context.Background()
	userRepo := postgres.NewUserRepository(db)
	workspaceRepo := postgres.NewWorkspaceRepository(db)
	generator := id.UUIDGenerator{}

	// Check if user already exists
	existingUser, err := userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		log.Printf("User %s already exists. Promoting to admin role and enabling account...", email)
		// Update role and status
		if err := userRepo.UpdateRole(ctx, existingUser.ID, entities.UserRoleAdmin); err != nil {
			log.Fatalf("Failed to promote user to admin: %v", err)
		}
		if err := userRepo.UpdateStatus(ctx, existingUser.ID, false); err != nil {
			log.Fatalf("Failed to enable user: %v", err)
		}
		log.Println("==================================================")
		log.Printf("🎉 SUCCESS! User %s promoted to ADMIN successfully!", email)
		log.Println("==================================================")
		return
	}

	// Create new admin user
	hash, err := security.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	adminUser := &entities.User{
		ID:           generator.New(),
		FullName:     name,
		Email:        email,
		PasswordHash: hash,
		AuthProvider: entities.AuthProviderPassword,
		Role:         entities.UserRoleAdmin,
		IsDisabled:   false,
	}

	if err := userRepo.Create(ctx, adminUser); err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	// Create admin's workspace
	workspace := &entities.Workspace{
		ID:     generator.New(),
		UserID: adminUser.ID,
		Name:   name + "'s Workspace",
	}
	if err := workspaceRepo.Create(ctx, workspace); err != nil {
		log.Fatalf("Failed to create admin workspace: %v", err)
	}

	fmt.Println("\n==================================================")
	fmt.Printf("🎉 ADMIN USER CREATED SUCCESSFULLY!\n")
	fmt.Printf("   Email    : %s\n", email)
	fmt.Printf("   Password : %s\n", password)
	fmt.Printf("   Role     : %s\n", entities.UserRoleAdmin)
	fmt.Println("==================================================")
}
