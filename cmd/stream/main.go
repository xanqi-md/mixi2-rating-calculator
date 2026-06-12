// cmd/stream/main.go - gRPC Stream mode (for local development)
package main

import (
	"context"
	"crypto/tls"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	"github.com/mixigroup/mixi2-application-sdk-go/event/stream"
	application_streamv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_stream/v1"
	"github.com/yourusername/openskill-rating-bot/internal/config"
	"github.com/yourusername/openskill-rating-bot/internal/db"
	"github.com/yourusername/openskill-rating-bot/internal/handler"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting OpenSkill Rating Bot (gRPC Stream mode)...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Admin user ID: %s", cfg.AdminUserID)

	// Initialize database
	store, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// Initialize authenticator
	authenticator, err := auth.NewAuthenticator(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.TokenURL,
	)
	if err != nil {
		log.Fatalf("Failed to create authenticator: %v", err)
	}

	// Initialize event handler
	h, err := handler.New(cfg, store, authenticator)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	// Connect to Stream server
	conn, err := grpc.NewClient(
		cfg.StreamAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		log.Fatalf("Failed to connect to stream server: %v", err)
	}
	defer conn.Close()

	client := application_streamv1.NewApplicationServiceClient(conn)
	watcher := stream.NewStreamWatcher(client, authenticator)

	log.Println("Connecting to mixi2 event stream... Press Ctrl+C to stop.")
	if err := watcher.Watch(context.Background(), h); err != nil {
		log.Fatalf("Stream watcher failed: %v", err)
	}
}
