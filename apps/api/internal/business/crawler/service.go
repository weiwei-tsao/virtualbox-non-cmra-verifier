package crawler

import (
	"context"
	"fmt"
	"sort"
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
	runID := generateRunID()
	if err := StartRun(ctx, s.runs, runID); err != nil {
		return "", err
	}
	go s.execute(context.Background(), runID, links)
	return runID, nil
}

func (s *Service) execute(ctx context.Context, runID string, links []string) {
	stats := model.CrawlRunStats{}
	status := "success"

	scrapeStats, err := ScrapeAndUpsert(ctx, s.fetcher, s.mailboxes, s.validator, links, runID)
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

	_ = FinishRun(ctx, s.runs, runID, stats, status)
}

func generateRunID() string {
	return fmt.Sprintf("RUN_%d", time.Now().Unix())
}
