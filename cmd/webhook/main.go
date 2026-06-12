// cmd/webhook/main.go - HTTP Webhook mode (for production deployment)
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"log"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	"github.com/mixigroup/mixi2-application-sdk-go/event/webhook"
	"github.com/yourusername/openskill-rating-bot/internal/config"
	"github.com/yourusername/openskill-rating-bot/internal/db"
	"github.com/yourusername/openskill-rating-bot/internal/handler"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting OpenSkill Rating Bot (Webhook mode)...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.ValidateWebhook(); err != nil {
		log.Fatalf("Webhook config validation failed: %v", err)
	}

	log.Printf("Admin user ID: %s", cfg.AdminUserID)
	log.Printf("Webhook port: %s", cfg.Port)

	// Decode Ed25519 public key for signature verification
	publicKeyBytes, err := base64.StdEncoding.DecodeString(cfg.SignaturePublicKey)
	if err != nil {
		log.Fatalf("Failed to decode SIGNATURE_PUBLIC_KEY: %v", err)
	}
	publicKey := ed25519.PublicKey(publicKeyBytes)

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

	// Start webhook server
	// SDK handles: Ed25519 signature verification, ping responses, health check at /healthz
	addr := ":" + cfg.Port
	server := webhook.NewServer(addr, publicKey, h)

	log.Printf("Webhook server listening on %s", addr)
	log.Println("Endpoints: POST /events (events), GET /healthz (health)")
	if err := server.Start(); err != nil {
		log.Fatalf("Webhook server failed: %v", err)
	}
}
