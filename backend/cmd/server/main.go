// Command server is the training-record backend HTTP API.
//
// Usage:
//
//	server                        run the API server
//	server -healthcheck           probe GET /api/health on $PORT; exit 0 iff 200
//	server -backup <dest>         write a consistent snapshot of $DB_PATH to <dest>
//	server -evaluate-training     run the daily LLM training evaluation batch (biweekly + bimonthly)
package main

import (
	"context"
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
	"training-record/internal/llmeval"
	"training-record/internal/service"
	"training-record/internal/store"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-healthcheck" || os.Args[1] == "--healthcheck") {
		os.Exit(runHealthcheck())
	}
	if len(os.Args) > 1 && (os.Args[1] == "-backup" || os.Args[1] == "--backup") {
		os.Exit(runBackup(os.Args[2:]))
	}
	if len(os.Args) > 1 && (os.Args[1] == "-evaluate-training" || os.Args[1] == "--evaluate-training") {
		os.Exit(runEvaluateTraining())
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

// runBackup writes a consistent snapshot of the database at $DB_PATH to
// args[0] (VACUUM INTO). It is intentionally independent of API_KEY so it can
// run as a one-shot maintenance command in an otherwise idle container.
func runBackup(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: server -backup <dest.db>")
		return 2
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./training.db"
	}
	db, err := database.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backup: open %q: %v\n", dbPath, err)
		return 1
	}
	defer db.Close()
	if err := database.Backup(db, args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "backup: %v\n", err)
		return 1
	}
	fmt.Printf("backup: wrote %s (from %s)\n", args[0], dbPath)
	return 0
}

// runEvaluateTraining runs the daily LLM training-evaluation batch (Phase 8):
// it gathers recent training history, asks the LLM evaluation server
// ($LLM_EVAL_API_URL) for a biweekly and a bimonthly evaluation, and
// persists both. Intended to be invoked once a day by an external scheduler
// (ofelia job-exec against this container; see compose.yaml).
func runEvaluateTraining() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluate-training: %v\n", err)
		return 1
	}
	if cfg.LLMEvalURL == "" {
		fmt.Fprintln(os.Stderr, "evaluate-training: LLM_EVAL_API_URL is not set")
		return 1
	}
	domain.SetTZ(cfg.TZ)

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluate-training: open %q: %v\n", cfg.DBPath, err)
		return 1
	}
	defer db.Close()
	if err := database.Migrate(db, trainingrecord.MigrationsFS); err != nil {
		fmt.Fprintf(os.Stderr, "evaluate-training: migrate: %v\n", err)
		return 1
	}

	st := store.New(db)
	svc := service.New(st)
	client := llmeval.New(llmeval.Config{URL: cfg.LLMEvalURL, APIKey: cfg.LLMEvalKey})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	biweekly, bimonthly, errs := svc.RunTrainingEvaluations(ctx, client, time.Now().In(domain.JST), cfg.LLMEvalHistoryWeeks)
	if biweekly != nil {
		fmt.Printf("evaluate-training: saved biweekly id=%d periodTo=%s\n", biweekly.ID, biweekly.PeriodTo)
	}
	if bimonthly != nil {
		fmt.Printf("evaluate-training: saved bimonthly id=%d periodTo=%s\n", bimonthly.ID, bimonthly.PeriodTo)
	}
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "evaluate-training: %v\n", e)
	}
	if len(errs) > 0 {
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
