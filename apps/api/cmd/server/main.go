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
	"github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/config"
	firestoreclient "github.com/weiwei-tsao/virtualbox-verifier/apps/api/internal/platform/firestore"
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
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	firestoreClient, credsSource, err := firestoreclient.New(ctx, cfg)
	if err != nil {
		log.Fatalf("firestore init: %v", err)
	}
	defer firestoreClient.Close()

	if err := firestoreclient.Ping(ctx, firestoreClient); err != nil {
		log.Fatalf("firestore ping: %v", err)
	}
	log.Printf("connected to Firestore project %s using %s credentials", cfg.FirebaseProjectID, credsSource)

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
