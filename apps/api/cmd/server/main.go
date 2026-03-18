package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/validation"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
	apirouter "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/http"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/smarty"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_ = godotenv.Load(".env.local", ".env")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	gin.SetMode(cfg.GinMode)

	firestoreClient, credsSource, err := firestoreclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("firestore init: %v", err)
	}
	defer firestoreClient.Close()

	if err := firestoreclient.Ping(ctx, firestoreClient); err != nil {
		log.Fatalf("firestore ping: %v", err)
	}
	log.Printf("connected to Firestore project %s using %s credentials", cfg.FirebaseProjectID, credsSource)

	mailboxRepo := repository.NewMailboxRepository(firestoreClient)
	runRepo := repository.NewRunRepository(firestoreClient)
	statsRepo := repository.NewStatsRepository(firestoreClient)

	fetcher := crawler.NewHTTPFetcher()
	validator := smarty.New(nil, smarty.Config{
		AuthIDs:    cfg.SmartyAuthIDs,
		AuthTokens: cfg.SmartyAuthTokens,
		Mock:       cfg.SmartyMock,
	})
	if cfg.SmartyMock {
		log.Printf("Smarty client initialized in MOCK mode")
	} else {
		log.Printf("Smarty client initialized with %d credential(s) for load balancing", len(cfg.SmartyAuthIDs))
	}

	jobManager := crawler.NewJobManager()
	crawlService := crawler.NewService(fetcher, validator, mailboxRepo, runRepo, statsRepo, 5, cfg.CrawlLinkSeeds, jobManager)

	// Load crawler configuration for validation service
	configLoader := config.NewConfigLoader(firestoreClient)
	crawlerConfig, err := configLoader.Load(ctx)
	if err != nil {
		log.Fatalf("load crawler config: %v", err)
	}
	log.Printf("Crawler config loaded - Daily validation budget: %d", crawlerConfig.DailyValidationBudget)

	// Initialize validation service components
	quotaConfig := validation.QuotaConfig{
		DailyBudget:         crawlerConfig.DailyValidationBudget,
		HighPriorityQuota:   crawlerConfig.HighPriorityQuota,
		MediumPriorityQuota: crawlerConfig.MediumPriorityQuota,
	}
	quotaManager := validation.NewQuotaManager(quotaConfig, firestoreClient)

	validationSvc := validation.NewValidationService(
		validator,
		mailboxRepo,
		quotaManager,
		crawlerConfig,
		func(msg string) { log.Printf("[validation] %s", msg) },
	)

	revalidationChk := validation.NewRevalidationChecker(
		mailboxRepo,
		crawlerConfig,
		func(msg string) { log.Printf("[revalidation] %s", msg) },
	)

	router := apirouter.NewRouter(mailboxRepo, runRepo, statsRepo, crawlService, validationSvc, revalidationChk, cfg.AllowedOrigins)

	// Start background workers if enabled (controlled by feature flags)
	if cfg.EnableValidationWorkers || cfg.EnableRevalidationChecker {
		startBackgroundWorkers(ctx, validationSvc, revalidationChk, cfg)
		log.Printf("Background workers enabled - Validation: %v, Revalidation: %v",
			cfg.EnableValidationWorkers, cfg.EnableRevalidationChecker)
	} else {
		log.Println("Background workers disabled (set ENABLE_VALIDATION_WORKERS=true or ENABLE_REVALIDATION_CHECKER=true to enable)")
	}

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Printf("server listening on :%s", cfg.Port)

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("server exited")
}

// startBackgroundWorkers starts background tasks for validation processing.
func startBackgroundWorkers(ctx context.Context, validationSvc *validation.ValidationService, revalidationChk *validation.RevalidationChecker, cfg config.Config) {
	// Worker 1: Validation processor (every 5 minutes)
	if cfg.EnableValidationWorkers {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()

			log.Println("Background validation worker started (interval: 5 minutes)")

			// Run immediately on startup
			runValidationWorker(ctx, validationSvc)

			for {
				select {
				case <-ctx.Done():
					log.Println("Validation worker stopped")
					return
				case <-ticker.C:
					runValidationWorker(ctx, validationSvc)
				}
			}
		}()
	}

	// Worker 2: Revalidation checker (daily at 2 AM UTC)
	if cfg.EnableRevalidationChecker {
		go func() {
			log.Println("Background revalidation checker started (daily at 2 AM UTC)")

		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, time.UTC)
			if now.Hour() >= 2 {
				// Already past 2 AM today, schedule for tomorrow
				next = next.Add(24 * time.Hour)
			}

			duration := next.Sub(now)
			log.Printf("Next revalidation check scheduled for %s (in %s)", next.Format("2006-01-02 15:04:05 MST"), duration)

			select {
			case <-ctx.Done():
				log.Println("Revalidation checker stopped")
				return
			case <-time.After(duration):
				runRevalidationChecker(ctx, revalidationChk)
			}
		}
		}()
	}
}

// runValidationWorker processes pending validations and retries.
func runValidationWorker(ctx context.Context, validationSvc *validation.ValidationService) {
	workerCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	log.Println("Validation worker running...")

	// Phase 1: Move retry_scheduled → pending if retry time arrived
	retryCount, err := validationSvc.ProcessRetries(workerCtx)
	if err != nil {
		log.Printf("Error processing retries: %v", err)
	} else if retryCount > 0 {
		log.Printf("Moved %d mailboxes from retry_scheduled to pending", retryCount)
	}

	// Phase 2: Process pending validations by priority
	stats, err := validationSvc.ProcessPendingValidations(workerCtx)
	if err != nil {
		log.Printf("Error processing validations: %v", err)
		return
	}

	log.Printf("Validation worker completed - High: %d, Medium: %d, Low: %d, Succeeded: %d, Failed: %d, Quota remaining: %d",
		stats.HighPriorityProcessed,
		stats.MediumPriorityProcessed,
		stats.LowPriorityProcessed,
		stats.Succeeded,
		stats.Failed,
		stats.QuotaRemaining,
	)

	if stats.QuotaExhausted {
		log.Println("Daily validation quota exhausted")
	}
}

// runRevalidationChecker marks old addresses for re-validation.
func runRevalidationChecker(ctx context.Context, revalidationChk *validation.RevalidationChecker) {
	checkerCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log.Println("Revalidation checker running...")

	stats, err := revalidationChk.CheckRevalidationNeeded(checkerCtx)
	if err != nil {
		log.Printf("Error checking revalidation: %v", err)
		return
	}

	log.Printf("Revalidation check completed - Checked: %d, Needs revalidation: %d, Approaching threshold: %d, Up-to-date: %d",
		stats.Checked,
		stats.NeedsRevalidation,
		stats.ApproachingThreshold,
		stats.UpToDate,
	)
}
