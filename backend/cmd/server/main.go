// Command server is the training-record backend HTTP API.
//
// Usage:
//
//	server                 run the API server
//	server -healthcheck    probe GET /api/health on $PORT; exit 0 iff 200
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "time/tzdata" // embed the tz database for LoadLocation in a scratch image

	trainingrecord "training-record"
	"training-record/internal/config"
	"training-record/internal/database"
	"training-record/internal/domain"
	"training-record/internal/hermes"
	"training-record/internal/httpapi"
	"training-record/internal/service"
	"training-record/internal/store"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-healthcheck" || os.Args[1] == "--healthcheck") {
		os.Exit(runHealthcheck())
	}
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:" + port + "/api/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	domain.SetTZ(cfg.TZ)
	log.Printf("startup: timezone=%s today=%s db=%s", domain.JST.String(), domain.Today(), cfg.DBPath)

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	fresh, err := database.HasAppSchema(db)
	if err != nil {
		return fmt.Errorf("inspect schema: %w", err)
	}
	freshDB := !fresh
	if freshDB {
		log.Printf("startup: database has no application schema (new database)")
	}

	if err := database.Migrate(db, trainingrecord.MigrationsFS); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if _, err := database.MaybeBootstrap(db, cfg.BootstrapDB, freshDB); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// Idempotent fixup: seed default presets and split legacy routine
	// snapshots into named presets. Runs every startup, no-op once done.
	if err := database.EnsureRoutinePresets(db); err != nil {
		return fmt.Errorf("ensure routine presets: %w", err)
	}

	st := store.New(db)
	svc := service.New(st)

	// 画像抽出（POST /api/spin-extract）は HERMES_API_URL があれば有効化
	var hermesClient *hermes.Client
	if cfg.HermesURL != "" {
		hermesClient = hermes.New(hermes.Config{URL: cfg.HermesURL, APIKey: cfg.HermesKey})
		log.Printf("startup: hermes image extraction enabled (%s)", cfg.HermesURL)
	}

	handler := httpapi.NewRouter(svc, cfg.APIKey, hermesClient)

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on :%s", cfg.Port)
	return srv.ListenAndServe()
}
