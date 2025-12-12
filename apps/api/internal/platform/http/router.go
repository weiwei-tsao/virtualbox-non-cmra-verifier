package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/business/crawler"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/repository"
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/pkg/model"
)

// Router wires HTTP handlers.
type Router struct {
	mailboxes *repository.MailboxRepository
	runs      *repository.RunRepository
	stats     *repository.StatsRepository
	crawler   *crawler.Service
	origins   string
}

func NewRouter(mailboxes *repository.MailboxRepository, runs *repository.RunRepository, stats *repository.StatsRepository, crawlerSvc *crawler.Service, allowedOrigins string) *gin.Engine {
	r := &Router{
		mailboxes: mailboxes,
		runs:      runs,
		stats:     stats,
		crawler:   crawlerSvc,
		origins:   allowedOrigins,
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

		// iPost1 specific endpoints
		api.POST("/crawl/ipost1/run", r.startIPost1Crawl)
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

func (r *Router) exportMailboxes(c *gin.Context) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=mailboxes.csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	if err := writer.Write([]string{"name", "street", "city", "state", "zip", "price", "link", "cmra", "rdi"}); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	err := r.mailboxes.StreamAll(c.Request.Context(), true, func(mb model.Mailbox) error {
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
