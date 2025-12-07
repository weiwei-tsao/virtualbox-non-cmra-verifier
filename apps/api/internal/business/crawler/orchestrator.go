package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// WorkerFn processes a mailbox URL and returns the parsed mailbox.
type WorkerFn func(ctx context.Context, url string) (model.Mailbox, error)

// Orchestrator coordinates concurrent scraping with cancellation support.
type Orchestrator struct {
	workerCount int
}

func NewOrchestrator(workerCount int) *Orchestrator {
	if workerCount <= 0 {
		workerCount = 5
	}
	return &Orchestrator{workerCount: workerCount}
}

// Run processes the provided URLs with bounded concurrency. It stops when context is canceled
// or when all URLs are processed. Results and errors are pushed into the provided channels.
func (o *Orchestrator) Run(ctx context.Context, urls []string, fn WorkerFn) (<-chan model.Mailbox, <-chan error) {
	out := make(chan model.Mailbox)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		jobs := make(chan string)
		var wg sync.WaitGroup

		worker := func() {
			defer wg.Done()
			for url := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				m, err := fn(ctx, url)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}
				select {
				case out <- m:
				case <-ctx.Done():
					return
				}
			}
		}

		for i := 0; i < o.workerCount; i++ {
			wg.Add(1)
			go worker()
		}

		for _, url := range urls {
			select {
			case jobs <- url:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return
			}
		}
		close(jobs)
		wg.Wait()
	}()

	return out, errCh
}

// MarkAndSweep sets active=false for mailboxes whose crawlRunId != currentRunId.
func MarkAndSweep(ctx context.Context, repo MailboxStore, currentRunID string) error {
	all, err := repo.FetchAllMap(ctx)
	if err != nil {
		return err
	}
	var toUpdate []model.Mailbox
	for _, m := range all {
		if m.CrawlRunID != currentRunID && m.Active {
			m.Active = false
			toUpdate = append(toUpdate, m)
		}
	}
	if len(toUpdate) == 0 {
		return nil
	}
	return repo.BatchUpsert(ctx, toUpdate)
}

// RunLifecycleRepo persists crawl run metadata.
type RunLifecycleRepo interface {
	CreateRun(ctx context.Context, run model.CrawlRun) error
	UpdateRun(ctx context.Context, run model.CrawlRun) error
}

// StartRun initializes a CrawlRun record.
func StartRun(ctx context.Context, repo RunLifecycleRepo, runID string) error {
	return repo.CreateRun(ctx, model.CrawlRun{
		RunID:     runID,
		Status:    "running",
		StartedAt: time.Now().UTC(),
	})
}

// FinishRun finalizes a CrawlRun record with stats and status.
func FinishRun(ctx context.Context, repo RunLifecycleRepo, runID string, stats model.CrawlRunStats, status string) error {
	return repo.UpdateRun(ctx, model.CrawlRun{
		RunID:      runID,
		Status:     status,
		Stats:      stats,
		FinishedAt: time.Now().UTC(),
	})
}
