package crawler

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler/ipost1"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// Service orchestrates end-to-end crawl.
type Service struct {
	fetcher   HTMLFetcher
	validator ValidationClient
	mailboxes *repository.MailboxRepository
	runs      *repository.RunRepository
	statsRepo *repository.StatsRepository
	workerCnt int
	seedLinks []string
}

func NewService(fetcher HTMLFetcher, validator ValidationClient, mailboxes *repository.MailboxRepository, runs *repository.RunRepository, statsRepo *repository.StatsRepository, workerCnt int, seedLinks []string) *Service {
	if workerCnt <= 0 {
		workerCnt = 5
	}
	return &Service{
		fetcher:   fetcher,
		validator: validator,
		mailboxes: mailboxes,
		runs:      runs,
		statsRepo: statsRepo,
		workerCnt: workerCnt,
		seedLinks: seedLinks,
	}
}

// Start kicks off a crawl run asynchronously.
func (s *Service) Start(ctx context.Context, links []string) (string, error) {
	if len(links) == 0 {
		links = s.seedLinks
	}
	if len(links) == 0 {
		return "", fmt.Errorf("no links provided to crawl (set CRAWL_LINK_SEEDS or pass links in request)")
	}
	startTime := time.Now().UTC()
	runID := generateRunID()
	if err := StartRun(ctx, s.runs, runID, "ATMB", startTime); err != nil {
		return "", err
	}
	// Guard long-running crawls to avoid stuck runs.
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	go func() {
		defer cancel()
		s.execute(runCtx, runID, links, startTime)
	}()
	return runID, nil
}

func (s *Service) execute(ctx context.Context, runID string, links []string, startedAt time.Time) {
	status := "running"
	stats := model.CrawlRunStats{}

	// Always finalize the run document, even on panic.
	defer func() {
		if rec := recover(); rec != nil {
			status = "failed"
			log.Printf("crawl panic run %s: %v", runID, rec)
		}
		if err := FinishRun(ctx, s.runs, runID, "ATMB", stats, status, startedAt); err != nil {
			log.Printf("finish run %s: %v", runID, err)
		}
	}()

	// If links look like listing pages (/l/usa or /l/usa/xx), attempt discovery even if seeds are empty.
	needsDiscovery := false
	for _, l := range links {
		if strings.Contains(l, "/l/usa") {
			needsDiscovery = true
			break
		}
	}
	if needsDiscovery {
		if discovered, err := DiscoverLinks(ctx, s.fetcher, links); err == nil && len(discovered) > 0 {
			links = discovered
			log.Printf("run %s: discovered %d detail links", runID, len(links))
		}
	}

	stats = model.CrawlRunStats{Found: len(links)}
	status = "success"

	progress := func(curr ScrapeStats) {
		// Update run in Firestore periodically.
		if (curr.Updated+curr.Skipped)%25 == 0 || curr.Updated+curr.Skipped == curr.Found {
			_ = s.runs.UpdateRun(ctx, model.CrawlRun{
				RunID:  runID,
				Status: "running",
				Stats: model.CrawlRunStats{
					Found:     curr.Found,
					Validated: curr.Validated,
					Skipped:   curr.Skipped,
					Failed:    curr.Failed,
				},
			})
		}
	}

	scrapeStats, err := ScrapeAndUpsert(ctx, s.fetcher, s.mailboxes, s.validator, links, runID, progress, func(msg string) {
		log.Printf("run %s: %s", runID, msg)
	})
	if err != nil {
		status = "failed"
		log.Printf("scrape error run %s: %v", runID, err)
	}
	stats.Found = scrapeStats.Found
	stats.Skipped = scrapeStats.Skipped
	stats.Validated = scrapeStats.Validated
	stats.Failed = scrapeStats.Failed

	if err := MarkAndSweep(ctx, s.mailboxes, runID, "ATMB"); err != nil {
		status = "partial_halt"
		log.Printf("mark and sweep error run %s: %v", runID, err)
	}

	// If nothing was processed successfully, mark as failed.
	if stats.Validated == 0 && stats.Skipped == 0 && stats.Found > 0 && stats.Failed >= stats.Found {
		status = "failed"
	}

	all, err := s.mailboxes.FetchAllMap(ctx)
	if err == nil {
		var list []model.Mailbox
		for _, m := range all {
			list = append(list, m)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Link < list[j].Link })
		sysStats := AggregateSystemStats(list)
		if err := s.statsRepo.SaveSystemStats(ctx, sysStats); err != nil {
			log.Printf("save system stats error run %s: %v", runID, err)
		}
	} else {
		log.Printf("fetch all mailboxes error run %s: %v", runID, err)
	}
}

func generateRunID() string {
	return fmt.Sprintf("RUN_%d", time.Now().Unix())
}

// Reprocess re-parses mailboxes from stored RawHTML without re-fetching.
// Returns immediately with a runID; actual reprocessing happens asynchronously.
func (s *Service) Reprocess(ctx context.Context, opts ReprocessOptions) (string, error) {
	runID := generateRunID()
	startTime := time.Now().UTC()

	if err := StartRun(ctx, s.runs, runID, "ATMB", startTime); err != nil {
		return "", err
	}

	// Run reprocessing asynchronously
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	go func() {
		defer cancel()
		s.executeReprocess(runCtx, runID, opts, startTime)
	}()

	return runID, nil
}

func (s *Service) executeReprocess(ctx context.Context, runID string, opts ReprocessOptions, startedAt time.Time) {
	status := "running"
	stats := model.CrawlRunStats{}

	// Always finalize the run document
	defer func() {
		if rec := recover(); rec != nil {
			status = "failed"
			log.Printf("reprocess panic run %s: %v", runID, rec)
		}
		if err := FinishRun(ctx, s.runs, runID, "ATMB", stats, status, startedAt); err != nil {
			log.Printf("finish run %s: %v", runID, err)
		}
	}()

	progress := func(curr ReprocessStats) {
		// Update run status periodically
		if curr.Processed%25 == 0 || curr.Processed+curr.Skipped >= curr.Total {
			_ = s.runs.UpdateRun(ctx, model.CrawlRun{
				RunID:  runID,
				Status: "running",
				Stats: model.CrawlRunStats{
					Found:     curr.Total,
					Validated: curr.Processed,
					Skipped:   curr.Skipped,
					Failed:    curr.Failed,
				},
			})
		}
	}

	reprocessStats, err := ReprocessFromDB(ctx, s.mailboxes, s.validator, opts, func(msg string) {
		log.Printf("run %s: %s", runID, msg)
	}, progress)

	if err != nil {
		status = "failed"
		log.Printf("reprocess error run %s: %v", runID, err)
	} else {
		status = "success"
	}

	stats.Found = reprocessStats.Total
	stats.Validated = reprocessStats.Processed
	stats.Skipped = reprocessStats.Skipped
	stats.Failed = reprocessStats.Failed

	// Update system stats after reprocessing
	all, err := s.mailboxes.FetchAllMap(ctx)
	if err == nil {
		var list []model.Mailbox
		for _, m := range all {
			list = append(list, m)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Link < list[j].Link })
		sysStats := AggregateSystemStats(list)
		if err := s.statsRepo.SaveSystemStats(ctx, sysStats); err != nil {
			log.Printf("save system stats error run %s: %v", runID, err)
		}
	}
}

// StartIPost1Crawl kicks off an iPost1 crawl run asynchronously.
func (s *Service) StartIPost1Crawl(ctx context.Context) (string, error) {
	startTime := time.Now().UTC()
	runID := generateRunID()
	if err := StartRun(ctx, s.runs, runID, "iPost1", startTime); err != nil {
		return "", err
	}

	// Guard long-running crawls
	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	go func() {
		defer cancel()
		s.executeIPost1(runCtx, runID, startTime)
	}()
	return runID, nil
}

func (s *Service) executeIPost1(ctx context.Context, runID string, startedAt time.Time) {
	status := "running"
	stats := model.CrawlRunStats{}

	// Always finalize the run document
	defer func() {
		if rec := recover(); rec != nil {
			status = "failed"
			log.Printf("ipost1 crawl panic run %s: %v", runID, rec)
		}
		if err := FinishRun(ctx, s.runs, runID, "iPost1", stats, status, startedAt); err != nil {
			log.Printf("finish run %s: %v", runID, err)
		}
	}()

	// Import iPost1 package - note: this creates a dependency
	// Using a separate adapter to avoid direct import
	ipostStats, err := s.executeIPost1Discovery(ctx, runID)
	if err != nil {
		status = "failed"
		log.Printf("ipost1 discovery error run %s: %v", runID, err)
	} else {
		status = "success"
	}

	// Map iPost1 stats to CrawlRunStats
	stats.Found = ipostStats.Found
	stats.Validated = ipostStats.Validated
	stats.Skipped = ipostStats.Skipped
	stats.Failed = ipostStats.Failed

	// Mark and sweep for iPost1 source only
	if err := MarkAndSweep(ctx, s.mailboxes, runID, "iPost1"); err != nil {
		status = "partial_halt"
		log.Printf("mark and sweep error run %s: %v", runID, err)
	}

	// If nothing was processed successfully, mark as failed
	if stats.Validated == 0 && stats.Skipped == 0 && stats.Found > 0 && stats.Failed >= stats.Found {
		status = "failed"
	}

	// Update system stats
	all, err := s.mailboxes.FetchAllMap(ctx)
	if err == nil {
		var list []model.Mailbox
		for _, m := range all {
			list = append(list, m)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Link < list[j].Link })
		sysStats := AggregateSystemStats(list)
		if err := s.statsRepo.SaveSystemStats(ctx, sysStats); err != nil {
			log.Printf("save system stats error run %s: %v", runID, err)
		}
	}
}

// IPost1Stats represents statistics from iPost1 crawl (to avoid circular import).
type IPost1Stats struct {
	Found     int
	Validated int
	Skipped   int
	Failed    int
}

func (s *Service) executeIPost1Discovery(ctx context.Context, runID string) (IPost1Stats, error) {
	// Import ipost1 discovery dynamically to avoid tight coupling
	// Note: You could also inject this as a dependency in NewService
	stats, err := s.runIPost1ProcessAndValidate(ctx, runID)
	if err != nil {
		return IPost1Stats{}, err
	}

	return IPost1Stats{
		Found:     stats.Found,
		Validated: stats.Validated,
		Skipped:   stats.Skipped,
		Failed:    stats.Failed,
	}, nil
}

// runIPost1ProcessAndValidate executes the iPost1 discovery and validation process.
// This method acts as an adapter between the Service and the ipost1 package.
func (s *Service) runIPost1ProcessAndValidate(ctx context.Context, runID string) (struct {
	Found     int
	Validated int
	Skipped   int
	Failed    int
}, error) {
	// Call iPost1 ProcessAndValidate with proper adapters
	stats, err := ipost1.ProcessAndValidate(
		ctx,
		s.validator, // Already implements the ValidationClient interface
		s.mailboxes, // Already implements the MailboxStore interface
		runID,
		func(msg string) {
			log.Printf("run %s: %s", runID, msg)
		},
	)

	if err != nil {
		return struct {
			Found     int
			Validated int
			Skipped   int
			Failed    int
		}{}, err
	}

	return struct {
		Found     int
		Validated int
		Skipped   int
		Failed    int
	}{
		Found:     stats.Found,
		Validated: stats.Validated,
		Skipped:   stats.Skipped,
		Failed:    stats.Failed,
	}, nil
}
