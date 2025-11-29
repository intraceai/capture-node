package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/intraceai/capture-node/internal/api"
	"github.com/intraceai/capture-node/internal/manifest"
	"github.com/intraceai/capture-node/internal/orchestrator"
	"github.com/intraceai/capture-node/internal/storage"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := getEnv("MINIO_ACCESS_KEY", "intrace")
	minioSecretKey := getEnv("MINIO_SECRET_KEY", "intrace123")
	minioBucket := getEnv("MINIO_BUCKET", "intrace")
	minioUseSSL := getEnv("MINIO_USE_SSL", "false") == "true"
	minioPublicURL := getEnv("MINIO_PUBLIC_URL", "http://localhost:9000")

	eventLogURL := getEnv("EVENT_LOG_URL", "http://localhost:8081")
	publicHost := getEnv("PUBLIC_HOST", "localhost")
	viewerURL := getEnv("VIEWER_URL", "http://localhost:3000")
	dockerNetwork := getEnv("DOCKER_NETWORK", "")
	listenAddr := getEnv("LISTEN_ADDR", ":8080")

	store, err := storage.NewMinIOStorage(
		minioEndpoint,
		minioAccessKey,
		minioSecretKey,
		minioBucket,
		minioUseSSL,
		minioPublicURL,
	)
	if err != nil {
		log.Fatalf("failed to create storage: %v", err)
	}

	if err := store.EnsureBucket(ctx); err != nil {
		log.Fatalf("failed to ensure bucket: %v", err)
	}

	orch, err := orchestrator.NewOrchestrator(dockerNetwork)
	if err != nil {
		log.Fatalf("failed to create orchestrator: %v", err)
	}
	orch.Start(ctx)
	defer orch.Stop()

	manifestBuilder := manifest.NewBuilder()

	server := api.NewServer(api.ServerConfig{
		Storage:      store,
		Orchestrator: orch,
		Manifest:     manifestBuilder,
		EventLogURL:  eventLogURL,
		PublicHost:   publicHost,
		ViewerURL:    viewerURL,
	})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("shutting down...")
		cancel()
	}()

	log.Printf("starting capture-node server on %s", listenAddr)
	if err := server.Run(listenAddr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
