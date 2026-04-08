package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/validation"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// Router wires HTTP handlers.
type Router struct {
	mailboxes       *repository.MailboxRepository
	runs            *repository.RunRepository
	validationRuns  *repository.ValidationRunRepository
	stats           *repository.StatsRepository
	crawler         *crawler.Service
	validationSvc   *validation.ValidationService
	revalidationChk *validation.RevalidationChecker
	origins         string
}

func NewRouter(
	mailboxes *repository.MailboxRepository,
	runs *repository.RunRepository,
	validationRuns *repository.ValidationRunRepository,
	stats *repository.StatsRepository,
	crawlerSvc *crawler.Service,
	validationSvc *validation.ValidationService,
	revalidationChk *validation.RevalidationChecker,
	allowedOrigins string,
) *gin.Engine {
	r := &Router{
		mailboxes:       mailboxes,
		runs:            runs,
		validationRuns:  validationRuns,
		stats:           stats,
		crawler:         crawlerSvc,
		validationSvc:   validationSvc,
		revalidationChk: revalidationChk,
		origins:         allowedOrigins,
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), r.corsMiddleware())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api")
	{
		api.GET("/mailboxes", r.listMailboxes)
		api.GET("/mailboxes/export", r.exportMailboxes)
		api.GET("/stats", r.getStats)
		api.POST("/stats/refresh", r.refreshStats)
		api.POST("/crawl/run", r.startCrawl)
		api.POST("/crawl/reprocess", r.reprocessMailboxes)
		api.GET("/crawl/status", r.getCrawlStatus)
		api.GET("/crawl/runs", r.listCrawlRuns)
		api.POST("/crawl/runs/:runId/cancel", r.cancelCrawlRun)

		// iPost1 specific endpoints
		api.POST("/crawl/ipost1/run", r.startIPost1Crawl)

		// Validation endpoints
		api.POST("/validation/run", r.runValidation)
		api.GET("/validation/stats", r.getValidationStats)
		api.GET("/validation/runs", r.listValidationRuns)
		api.GET("/validation/runs/:runId", r.getValidationRun)
		api.POST("/validation/revalidation/check", r.checkRevalidation)
	}

	return router
}

func (r *Router) corsMiddleware() gin.HandlerFunc {
	origins := strings.Split(r.origins, ",")
	trimmed := make([]string, 0, len(origins))
	for _, o := range origins {
		if t := strings.TrimSpace(o); t != "" {
			trimmed = append(trimmed, t)
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := "*"
		for _, o := range trimmed {
			if o == "*" || o == origin {
				allowed = origin
				break
			}
		}
		c.Header("Access-Control-Allow-Origin", allowed)
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (r *Router) listMailboxes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	activeParam := c.Query("active")
	var activePtr *bool
	if activeParam != "" {
		val := activeParam == "true"
		activePtr = &val
	}

	items, total, err := r.mailboxes.List(c.Request.Context(), repository.MailboxQuery{
		State:    c.Query("state"),
		CMRA:     c.Query("cmra"),
		RDI:      c.Query("rdi"),
		Source:   c.Query("source"),
		Active:   activePtr,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
	})
}

// generateExportFilename creates a descriptive CSV filename based on active filters.
// Format: mailbox-{filters}-{timestamp}.csv
// If only default filters (active=true), returns: mailbox-{timestamp}.csv
func generateExportFilename(query repository.MailboxQuery) string {
	parts := []string{"mailbox"}

	// Add filter segments in order: state, source, cmra, rdi
	if query.State != "" {
		parts = append(parts, query.State)
	}
	if query.Source != "" {
		parts = append(parts, query.Source)
	}
	if query.CMRA != "" {
		parts = append(parts, query.CMRA)
	}
	if query.RDI != "" {
		parts = append(parts, query.RDI)
	}

	// Add timestamp in RFC3339 basic format (UTC)
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	parts = append(parts, timestamp)

	return strings.Join(parts, "-") + ".csv"
}

func (r *Router) exportMailboxes(c *gin.Context) {
	// Parse query parameters for filtering
	activePtr := func() *bool { v := true; return &v }() // default to active only
	if activeParam := c.Query("active"); activeParam != "" {
		val := activeParam == "true"
		activePtr = &val
	}

	query := repository.MailboxQuery{
		State:  c.Query("state"),
		CMRA:   c.Query("cmra"),
		RDI:    c.Query("rdi"),
		Source: c.Query("source"),
		Active: activePtr,
	}

	// Generate dynamic filename based on filters
	filename := generateExportFilename(query)

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	if err := writer.Write([]string{"name", "street", "city", "state", "zip", "price", "link", "cmra", "rdi", "source"}); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	err := r.mailboxes.StreamWithQuery(c.Request.Context(), query, func(mb model.Mailbox) error {
		row := []string{
			mb.Name,
			mb.AddressRaw.Street,
			mb.AddressRaw.City,
			mb.AddressRaw.State,
			mb.AddressRaw.Zip,
			fmt.Sprintf("%.2f", mb.Price),
			mb.Link,
			mb.CMRA,
			mb.RDI,
			mb.Source,
		}
		return writer.Write(row)
	})
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
}

func (r *Router) getStats(c *gin.Context) {
	stats, err := r.stats.GetSystemStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (r *Router) refreshStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Fetch all mailboxes
	all, err := r.mailboxes.FetchAllMap(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch mailboxes: " + err.Error()})
		return
	}

	// Convert map to slice
	var list []model.Mailbox
	for _, m := range all {
		list = append(list, m)
	}

	// Aggregate stats
	sysStats := crawler.AggregateSystemStats(list)

	// Save stats
	if err := r.stats.SaveSystemStats(ctx, sysStats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save stats: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, sysStats)
}

type startCrawlReq struct {
	Links []string `json:"links"`
}

func (r *Router) startCrawl(c *gin.Context) {
	var req startCrawlReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	runID, err := r.crawler.Start(c.Request.Context(), req.Links)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"runId": runID})
}

func (r *Router) getCrawlStatus(c *gin.Context) {
	runID := c.Query("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}
	run, err := r.runs.GetRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (r *Router) listCrawlRuns(c *gin.Context) {
	runs, err := r.runs.ListRuns(c.Request.Context(), 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs})
}

func (r *Router) cancelCrawlRun(c *gin.Context) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}

	// Cancel the running goroutine (if still running)
	wasRunning := r.crawler.CancelJob(runID)

	// Update database status
	if err := r.runs.CancelRun(c.Request.Context(), runID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Run cancelled successfully",
		"runId":      runID,
		"wasRunning": wasRunning,
	})
}

type reprocessReq struct {
	TargetVersion   string `json:"targetVersion"`   // Optional: parser version to update to (defaults to current)
	OnlyOutdated    bool   `json:"onlyOutdated"`    // Optional: only reprocess records with different parser version
	ForceRevalidate bool   `json:"forceRevalidate"` // Optional: force Smarty re-validation even if data unchanged (for mock->real API switch)
}

func (r *Router) reprocessMailboxes(c *gin.Context) {
	var req reprocessReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	opts := crawler.ReprocessOptions{
		TargetVersion:   req.TargetVersion,
		OnlyOutdated:    req.OnlyOutdated,
		ForceRevalidate: req.ForceRevalidate,
	}

	runID, err := r.crawler.Reprocess(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runId":   runID,
		"message": "Reprocessing started. Check status with GET /api/crawl/status?runId=" + runID,
	})
}

func (r *Router) startIPost1Crawl(c *gin.Context) {
	runID, err := r.crawler.StartIPost1Crawl(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"runId":   runID,
		"message": "iPost1 crawl started. Check status with GET /api/crawl/status?runId=" + runID,
	})
}

// Validation endpoints

func (r *Router) runValidation(c *gin.Context) {
	if r.validationSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Validation service not available"})
		return
	}

	stats, err := r.validationSvc.ProcessPendingValidations(c.Request.Context(), "manual")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Validation completed",
		"stats":   stats,
	})
}

func (r *Router) getValidationStats(c *gin.Context) {
	stats, err := r.mailboxes.GetValidationStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (r *Router) checkRevalidation(c *gin.Context) {
	if r.revalidationChk == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Revalidation checker not available"})
		return
	}

	stats, err := r.revalidationChk.CheckRevalidationNeeded(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Revalidation check completed",
		"stats":   stats,
	})
}

func (r *Router) listValidationRuns(c *gin.Context) {
	runs, err := r.validationRuns.ListRuns(c.Request.Context(), 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs})
}

func (r *Router) getValidationRun(c *gin.Context) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}

	run, err := r.validationRuns.GetRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, run)
}
