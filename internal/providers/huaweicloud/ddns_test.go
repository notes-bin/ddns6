package huaweicloud_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/notes-bin/ddns6/internal/providers/huaweicloud"
)

func TestMain(t *testing.T) {
	// Create client with credentials
	client := huaweicloud.NewClient(
		"your-username",
		"your-password",
		"your-domain-name",
	)

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
