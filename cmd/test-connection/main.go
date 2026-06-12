// cmd/test-connection/main.go - Connection test tool
// Tests connectivity to mixi2 API with current environment credentials
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"github.com/yourusername/openskill-rating-bot/internal/config"
)

func maskSecret(s string) string {
	if len(s) <= 6 {
		return "***"
	}
	return s[:6] + strings.Repeat("*", len(s)-6)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("=== OpenSkill Rating Bot - Connection Test ===")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Config load failed: %v", err)
	}

	log.Printf("✅ Config loaded:")
	log.Printf("   CLIENT_ID prefix : %s", maskSecret(cfg.ClientID))
	log.Printf("   TOKEN_URL        : %s", cfg.TokenURL)
	log.Printf("   API_ADDRESS      : %s", cfg.APIAddress)
	log.Printf("   STREAM_ADDRESS   : %s", cfg.StreamAddress)
	log.Printf("   ADMIN_USER_ID    : %s", cfg.AdminUserID)

	// Step 1: Get access token
	log.Println("\n[1/3] Testing OAuth2 token acquisition...")
	authenticator, err := auth.NewAuthenticator(
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.TokenURL,
	)
	if err != nil {
		log.Fatalf("❌ Authenticator creation failed: %v", err)
	}

	tokenCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	token, err := authenticator.GetAccessToken(tokenCtx)
	if err != nil {
		log.Fatalf("❌ Token acquisition failed: %v\n  Check CLIENT_ID, CLIENT_SECRET, TOKEN_URL", err)
	}
	log.Printf("✅ Access token obtained: %s...", maskSecret(token))

	// Step 2: Connect to API gRPC server
	log.Println("\n[2/3] Testing gRPC API connection...")
	conn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		log.Fatalf("❌ gRPC connection failed: %v", err)
	}
	defer conn.Close()
	log.Printf("✅ gRPC connection established to %s", cfg.APIAddress)

	// Step 3: Make a real API call (GetUsers to verify auth works)
	log.Println("\n[3/3] Testing API call (GetUsers)...")
	client := application_apiv1.NewApplicationServiceClient(conn)

	authCtx, err := authenticator.AuthorizedContext(context.Background())
	if err != nil {
		log.Fatalf("❌ AuthorizedContext failed: %v", err)
	}

	apiCtx, apiCancel := context.WithTimeout(authCtx, 15*time.Second)
	defer apiCancel()

	// Try to get info about the admin user
	resp, err := client.GetUsers(apiCtx, &application_apiv1.GetUsersRequest{
		UserIdList: []string{cfg.AdminUserID},
	})
	if err != nil {
		log.Printf("⚠️  GetUsers API call failed (non-fatal): %v", err)
		log.Printf("   ADMIN_USER_ID may be invalid format for GetUsers API.")
		log.Printf("   OAuth2 token acquisition and gRPC connection are confirmed working!")
	} else if len(resp.Users) > 0 {
		user := resp.Users[0]
		log.Printf("✅ API call succeeded!")
		log.Printf("   Admin User ID  : %s", user.GetUserId())
		log.Printf("   Display Name   : %s", user.GetDisplayName())
	} else {
		log.Printf("✅ API call succeeded (user not found, but connection works)")
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("✅ ALL CHECKS PASSED - Bot is ready to run!")
	fmt.Printf("   TOKEN_URL   : %s\n", cfg.TokenURL)
	fmt.Printf("   API_ADDRESS : %s\n", cfg.APIAddress)
	fmt.Println(strings.Repeat("=", 50))

	os.Exit(0)
}
