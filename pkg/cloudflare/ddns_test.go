package cloudflare

import (
	"testing"

	"github.com/notes-bin/ddns6/utils"
)

var token, _ = utils.GetEnvSafe("CLOUDFLARE_AUTH_TOKEN")

func TestGetZones(t *testing.T) {
	c := NewCloudflare(token["CLOUDFLARE_AUTH_TOKEN"])
	resp := new(cloudflareZoneResponse)
	if err := c.getZones("notes-bin.top", resp); err != nil {
		t.Error(err)
	}
	t.Logf("%+v", resp)
}
