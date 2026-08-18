package main

import (
	"context"
	"crm/internal/activity"
	"crm/internal/assets"
	"crm/internal/auth"
	"crm/internal/config"
	"crm/internal/contact"
	"crm/internal/db"
	"crm/internal/export"
	"crm/internal/health"
	"crm/internal/lead"
	"crm/internal/middleware"
	"crm/internal/pipeline"
	"crm/internal/program"
	"crm/internal/ratelimit"
	"crm/internal/rbac"
	"crm/internal/seed"
	"crm/internal/tag"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	buildString   = "dev"
	versionString = "v0.1.0"
)

func main() {
	configPath := flag.String("config", "config.toml", "Path to config file")
	newConfigPath := flag.String("new-config", "", "Write a config template to this path and exit (overwrites only a pristine template, never edited files)")
	flag.Parse()

	if *newConfigPath != "" {
		if err := config.WriteTemplate(*newConfigPath); err != nil {
			slog.Error("failed to write config template", "error", err)
			os.Exit(1)
		}
		slog.Info("config template written", "path", *newConfigPath)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	level, err := cfg.Log.SlogLevel()
	if err != nil {
		slog.Error("failed to parse log level", "error", err)
		os.Exit(1)
	}
	var handler slog.Handler
	if strings.EqualFold(cfg.Log.Format, "json") {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))

	appAssets, err := assets.Load()
	if err != nil {
		slog.Error("failed to load assets", "error", err)
		os.Exit(1)
	}
	slog.Info("assets resolved", "stuffed", appAssets.Stuffed())

	slog.Info("starting "+cfg.App.Name, "version", versionString, "build", buildString)

	database, err := db.Connect(cfg.DB)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := runMigrations(cfg.DB.DSN(), appAssets); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	if err := seed.Seed(database, cfg.Auth, cfg.Superadmin); err != nil {
		slog.Error("failed to seed data", "error", err)
		os.Exit(1)
	}

	activitySvc := activity.NewService(database)

	authSvc := auth.NewService(database, cfg.Auth)
	authH := auth.NewHandler(authSvc, activitySvc)

	rbacSvc := rbac.NewService(database)
	rbacH := rbac.NewHandler(rbacSvc)

	contactSvc := contact.NewService(database)
	contactH := contact.NewHandler(contactSvc, rbacSvc)

	pipelineSvc := pipeline.NewService(database)
	pipelineH := pipeline.NewHandler(pipelineSvc)

	programSvc := program.NewService(database)
	programH := program.NewHandler(programSvc)

	leadSvc := lead.NewService(database)
	leadH := lead.NewHandler(leadSvc)

	activityH := activity.NewHandler(activitySvc)

	tagSvc := tag.NewService(database)
	tagH := tag.NewHandler(tagSvc)

	exportSvc := export.NewService(database)
	exportH := export.NewHandler(exportSvc)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	// ClientIP resolves the client IP for rate limiting from the socket peer,
	// honoring forwarded headers only from explicitly trusted proxies (never
	// from client-supplied values).
	r.Use(middleware.ClientIP(cfg.App.TrustedProxies))
	r.Use(middleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.BodyLimit)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		MaxAge:         300,
	}))

	r.Get("/healthz", health.Live)
	r.Get("/readyz", health.Ready(database))

	loginLimiter := ratelimit.New(10, time.Minute)
	refreshLimiter := ratelimit.New(30, time.Minute)
	// Per-account limit on current-password guesses so a stolen access token
	// cannot brute-force the account's password at full speed.
	passwordChangeLimiter := ratelimit.New(5, time.Minute)

	r.With(loginLimiter.Middleware).Post("/api/auth/login", authH.Login)
	r.With(middleware.CSRF, refreshLimiter.Middleware).Post("/api/auth/refresh", authH.Refresh)
	r.With(middleware.CSRF).Post("/api/auth/logout", authH.Logout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authSvc))

		// Self-service routes stay reachable while a forced password change
		// is pending so the account can clear the flag.
		r.Get("/api/auth/me", authH.Me)
		r.Patch("/api/auth/me", authH.UpdateProfile)
		r.With(passwordChangeLimiter.UserMiddleware).Patch("/api/auth/me/password", authH.ChangePassword)
		r.Get("/api/auth/me/permissions", rbacH.MePermissions)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequirePasswordChanged(authSvc))

			r.Get("/api/contacts", middleware.RequirePermission(rbacSvc, "contact:read", contactH.List))
			r.Post("/api/contacts", middleware.RequirePermission(rbacSvc, "contact:write", contactH.Create))
			r.Post("/api/contacts/bulk", middleware.RequirePermission(rbacSvc, "contact:write", contactH.BulkCreate))
			// Registered before /api/contacts/{id} so the literal path wins over
			// the param route (ADR 012 lead-entry resolve).
			r.Get("/api/contacts/resolve", middleware.RequirePermission(rbacSvc, "lead:write", contactH.Resolve))
			r.Get("/api/contacts/{id}", middleware.RequirePermission(rbacSvc, "contact:read", contactH.Get))
			r.Patch("/api/contacts/{id}", middleware.RequirePermission(rbacSvc, "contact:write", contactH.Update))
			r.Delete("/api/contacts/{id}", middleware.RequirePermission(rbacSvc, "contact:write", contactH.Delete))
			r.Get("/api/contacts/{id}/notes", middleware.RequirePermission(rbacSvc, "contact:read", contactH.ListNotes))
			r.Post("/api/contacts/{id}/notes", middleware.RequirePermission(rbacSvc, "contact:write", contactH.CreateNote))
			r.Delete(
				"/api/contacts/{id}/notes/{note_id}",
				middleware.RequirePermission(rbacSvc, "contact:write", contactH.DeleteNote),
			)

			r.Get("/api/leads", middleware.RequirePermission(rbacSvc, "lead:read", leadH.List))
			r.Post("/api/leads", middleware.RequirePermission(rbacSvc, "lead:write", leadH.Create))
			r.Patch("/api/leads/{id}", middleware.RequirePermission(rbacSvc, "lead:write", leadH.Update))
			r.Delete("/api/leads/{id}", middleware.RequirePermission(rbacSvc, "lead:write", leadH.Delete))

			r.Get("/api/leads/{id}/activities", middleware.RequirePermission(rbacSvc, "lead:read", leadH.ListActivities))
			r.Post("/api/leads/{id}/activities", middleware.RequirePermission(rbacSvc, "lead:write", leadH.CreateActivity))
			r.Patch(
				"/api/leads/{id}/activities/{activity_id}",
				middleware.RequirePermission(rbacSvc, "lead:write", leadH.UpdateActivity),
			)
			r.Delete(
				"/api/leads/{id}/activities/{activity_id}",
				middleware.RequirePermission(rbacSvc, "lead:write", leadH.DeleteActivity),
			)
			r.Get("/api/leads/{id}/history", middleware.RequirePermission(rbacSvc, "lead:read", leadH.ListHistory))

			r.Get("/api/reminders", middleware.RequirePermission(rbacSvc, "lead:read", leadH.PendingReminders))
			r.Get("/api/activities", middleware.RequirePermission(rbacSvc, "lead:read", leadH.ListAllActivities))
			r.Patch("/api/leads/{lead_id}/reminders/{id}", middleware.RequirePermission(rbacSvc, "lead:write", leadH.DismissReminder))
			r.Post("/api/leads/{lead_id}/reminders/{id}/snooze", middleware.RequirePermission(rbacSvc, "lead:write", leadH.SnoozeReminder))

			r.Get("/api/pipelines", middleware.RequirePermission(rbacSvc, "lead:read", pipelineH.List))
			r.Post("/api/pipelines", middleware.RequirePermission(rbacSvc, "settings:manage", pipelineH.Create))
			r.Patch("/api/pipelines/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", pipelineH.Update))
			r.Delete("/api/pipelines/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", pipelineH.Delete))

			r.Get("/api/programs", middleware.RequirePermission(rbacSvc, "lead:read", programH.ListActive))
			r.Get("/api/programs/manage", middleware.RequirePermission(rbacSvc, "settings:manage", programH.ListAll))
			r.Post("/api/programs", middleware.RequirePermission(rbacSvc, "settings:manage", programH.Create))
			r.Patch("/api/programs/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", programH.Update))
			r.Delete("/api/programs/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", programH.Archive))
			r.Post("/api/programs/{id}/restore", middleware.RequirePermission(rbacSvc, "settings:manage", programH.Restore))

			r.Post("/api/pipelines/{id}/stages", middleware.RequirePermission(rbacSvc, "settings:manage", pipelineH.CreateStage))
			r.Patch("/api/stages/{stage_id}", middleware.RequirePermission(rbacSvc, "settings:manage", pipelineH.UpdateStage))
			r.Delete("/api/stages/{stage_id}", middleware.RequirePermission(rbacSvc, "settings:manage", pipelineH.DeleteStage))

			r.Get("/api/roles", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.ListRoles))
			r.Post("/api/roles", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.CreateRole))
			r.Patch("/api/roles/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.UpdateRole))
			r.Delete("/api/roles/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.DeleteRole))

			r.Get("/api/permissions", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.ListPermissions))
			r.Get("/api/roles/{id}/permissions", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.GetRolePermissions))
			r.Post("/api/roles/{id}/permissions", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.AssignPermission))
			r.Put("/api/roles/{id}/permissions", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.SetRolePermissions))
			r.Delete(
				"/api/roles/{id}/permissions/{permission_id}",
				middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.RemovePermission),
			)

			r.Get("/api/users", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.ListUsers))
			r.Post("/api/users", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.CreateUser))
			r.Delete("/api/users/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.DeleteUser))
			r.Put("/api/users/{id}/role", middleware.RequirePermission(rbacSvc, "settings:manage", rbacH.SetUserRole))

			r.Get("/api/activity", middleware.RequirePermission(rbacSvc, "activity:read", activityH.List))

			r.Get("/api/tags", middleware.RequireAny(rbacSvc, []string{"contact:read", "lead:read", "settings:manage"}, tagH.List))
			r.Post("/api/tags", middleware.RequirePermission(rbacSvc, "settings:manage", tagH.Create))
			r.Patch("/api/tags/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", tagH.Update))
			r.Delete("/api/tags/{id}", middleware.RequirePermission(rbacSvc, "settings:manage", tagH.Delete))

			r.Get("/api/export/csv", middleware.RequirePermission(rbacSvc, "data:export", exportH.CSV))
		})
	})

	r.Get("/", appAssets.ServeFrontend)
	r.Get("/assets/*", appAssets.ServeFrontend)
	r.NotFound(appAssets.ServeFrontend)

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

func runMigrations(dsn string, appAssets *assets.Assets) error {
	src, err := iofs.New(appAssets.Migrations(), "migrations")
	if err != nil {
		return fmt.Errorf("migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
