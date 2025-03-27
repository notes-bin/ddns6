package cloudflare_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/notes-bin/ddns6/internal/providers/cloudflare"
)

func TestMain(t *testing.T) {
	// Create client with API token (recommended)
	client := cloudflare.NewClient(
		cloudflare.WithAPIToken("your-api-token"),
		cloudflare.WithAccountID("your-account-id"),
		// Optional: cloudflare.WithZoneID("your-zone-id"),
	)

	// Or with legacy API key and email
	// client := cloudflare.NewClient(
	//     cloudflare.WithAPIKey("your-api-key", "your-email@example.com"),
	// )

	// Add TXT record
	err := client.AddTxtRecord("_acme-challenge.www.example.com", "XKrxpRBosdIKFzxW_CT3KLZNf6q0HG9i01zxXp5CPBs")
	if err != nil {
		log.Fatalf("Error adding TXT record: %v", err)
	}
	fmt.Println("TXT record added successfully")

	// Remove TXT record
	err = client.RemoveTxtRecord("_acme-challenge.www.example.com", "XKrxpRBosdIKFzxW_CT3KLZNf6q0HG9i01zxXp5CPBs")
	if err != nil {
		log.Fatalf("Error removing TXT record: %v", err)
	}
	fmt.Println("TXT record removed successfully")
}
