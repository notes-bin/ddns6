package alicloud_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/notes-bin/ddns6/internal/providers/alicloud"
)

func TestMain(t *testing.T) {
	// Create client with API credentials
	client := alicloud.NewClient("your-access-key-id", "your-access-key-secret")

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
