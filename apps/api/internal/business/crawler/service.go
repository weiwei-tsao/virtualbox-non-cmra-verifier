package crawler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
	if err := StartRun(ctx, s.runs, runID, startTime); err != nil {
		return "", err
	}
	go s.execute(context.Background(), runID, links, startTime)
	return runID, nil
}

func (s *Service) execute(ctx context.Context, runID string, links []string, startedAt time.Time) {
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
		}
	}

	stats := model.CrawlRunStats{Found: len(links)}
	status := "success"

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

	scrapeStats, err := ScrapeAndUpsert(ctx, s.fetcher, s.mailboxes, s.validator, links, runID, progress)
	if err != nil {
		status = "failed"
	}
	stats.Found = scrapeStats.Found
	stats.Skipped = scrapeStats.Skipped
	stats.Validated = scrapeStats.Validated
	stats.Failed = scrapeStats.Failed

	if err := MarkAndSweep(ctx, s.mailboxes, runID); err != nil {
		status = "partial_halt"
	}

	all, err := s.mailboxes.FetchAllMap(ctx)
	if err == nil {
		var list []model.Mailbox
		for _, m := range all {
			list = append(list, m)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Link < list[j].Link })
		sysStats := AggregateSystemStats(list)
		_ = s.statsRepo.SaveSystemStats(ctx, sysStats)
	}

	_ = FinishRun(ctx, s.runs, runID, stats, status, startedAt)
}

func generateRunID() string {
	return fmt.Sprintf("RUN_%d", time.Now().Unix())
}
