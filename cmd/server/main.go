package main

import (
	"context"
	"crm/internal/activity"
	"crm/internal/auth"
	"crm/internal/config"
	"crm/internal/contact"
	"crm/internal/db"
	"crm/internal/lead"
	"crm/internal/middleware"
	"crm/internal/pipeline"
	"crm/internal/rbac"
	"crm/internal/seed"
	"crm/internal/tag"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var (
	buildString   = "dev"
	versionString = "v0.1.0"
)

func main() {
	configPath := flag.String("config", "config.toml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting "+cfg.App.Name, "version", versionString, "build", buildString)

	database, err := db.Connect(cfg.DB)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := runMigrations(cfg.DB.DSN()); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	if err := seed.Seed(database, cfg.Auth); err != nil {
		slog.Error("failed to seed data", "error", err)
		os.Exit(1)
	}

	authSvc := auth.NewService(database, cfg.Auth)
	authH := auth.NewHandler(authSvc)

	rbacSvc := rbac.NewService(database)
	rbacH := rbac.NewHandler(rbacSvc)

	contactSvc := contact.NewService(database)
	contactH := contact.NewHandler(contactSvc)

	pipelineSvc := pipeline.NewService(database)
	pipelineH := pipeline.NewHandler(pipelineSvc)

	leadSvc := lead.NewService(database)
	leadH := lead.NewHandler(leadSvc)

	activitySvc := activity.NewService(database)
	activityH := activity.NewHandler(activitySvc)

	tagSvc := tag.NewService(database)
	tagH := tag.NewHandler(tagSvc)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:           300,
	}))

	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.Refresh)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authSvc))

		r.Post("/api/auth/logout", authH.Logout)
		r.Get("/api/auth/me", authH.Me)
		r.Patch("/api/auth/me", authH.UpdateProfile)
		r.Get("/api/auth/me/permissions", rbacH.MePermissions)

		r.Get("/api/contacts", middleware.RequirePermission(rbacSvc, "contact:read", contactH.List))
		r.Post("/api/contacts", middleware.RequirePermission(rbacSvc, "contact:write", contactH.Create))
		r.Post("/api/contacts/bulk", middleware.RequirePermission(rbacSvc, "contact:write", contactH.BulkCreate))
		r.Get("/api/contacts/{id}", middleware.RequirePermission(rbacSvc, "contact:read", contactH.Get))
		r.Patch("/api/contacts/{id}", middleware.RequirePermission(rbacSvc, "contact:write", contactH.Update))
		r.Delete("/api/contacts/{id}", middleware.RequirePermission(rbacSvc, "contact:delete", contactH.Delete))

		r.Get("/api/leads", middleware.RequirePermission(rbacSvc, "lead:read", leadH.List))
		r.Post("/api/leads", middleware.RequirePermission(rbacSvc, "lead:write", leadH.Create))
		r.Patch("/api/leads/{id}", middleware.RequirePermission(rbacSvc, "lead:write", leadH.Update))
		r.Delete("/api/leads/{id}", middleware.RequirePermission(rbacSvc, "lead:delete", leadH.Delete))

		r.Get("/api/leads/{id}/activities", middleware.RequirePermission(rbacSvc, "lead:read", leadH.ListActivities))
		r.Post("/api/leads/{id}/activities", middleware.RequirePermission(rbacSvc, "lead:write", leadH.CreateActivity))
		r.Delete("/api/leads/{id}/activities/{activity_id}", middleware.RequirePermission(rbacSvc, "lead:write", leadH.DeleteActivity))

		r.Get("/api/reminders", leadH.PendingReminders)
		r.Patch("/api/reminders/{id}", leadH.DismissReminder)

		r.Get("/api/pipelines", pipelineH.List)
		r.Post("/api/pipelines", middleware.RequirePermission(rbacSvc, "pipeline:manage", pipelineH.Create))
		r.Patch("/api/pipelines/{id}", middleware.RequirePermission(rbacSvc, "pipeline:manage", pipelineH.Update))
		r.Delete("/api/pipelines/{id}", middleware.RequirePermission(rbacSvc, "pipeline:manage", pipelineH.Delete))

		r.Post("/api/pipelines/{id}/stages", middleware.RequirePermission(rbacSvc, "pipeline:manage", pipelineH.CreateStage))
		r.Patch("/api/stages/{stage_id}", middleware.RequirePermission(rbacSvc, "pipeline:manage", pipelineH.UpdateStage))
		r.Delete("/api/stages/{stage_id}", middleware.RequirePermission(rbacSvc, "pipeline:manage", pipelineH.DeleteStage))

		r.Get("/api/roles", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.ListRoles))
		r.Post("/api/roles", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.CreateRole))
		r.Patch("/api/roles/{id}", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.UpdateRole))
		r.Delete("/api/roles/{id}", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.DeleteRole))

		r.Get("/api/permissions", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.ListPermissions))
		r.Get("/api/roles/{id}/permissions", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.GetRolePermissions))
		r.Post("/api/roles/{id}/permissions", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.AssignPermission))
		r.Delete("/api/roles/{id}/permissions/{permission_id}", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.RemovePermission))

		r.Get("/api/users", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.ListUsers))
		r.Post("/api/users", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.CreateUser))
		r.Delete("/api/users/{id}", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.DeleteUser))
		r.Post("/api/users/{id}/roles", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.AssignUserRole))
		r.Delete("/api/users/{id}/roles/{role_id}", middleware.RequirePermission(rbacSvc, "rbac:manage", rbacH.RemoveUserRole))

		r.Get("/api/activity", middleware.RequirePermission(rbacSvc, "activity:read", activityH.List))

		r.Get("/api/tags", tagH.List)
		r.Post("/api/tags", middleware.RequirePermission(rbacSvc, "contact:write", tagH.Create))
		r.Delete("/api/tags/{id}", middleware.RequirePermission(rbacSvc, "contact:write", tagH.Delete))
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server listening", "port", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}
	slog.Info("server stopped")
}

func runMigrations(dsn string) error {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
