package cloudflare

import (
	"testing"

	"github.com/notes-bin/ddns6/utils"
)

var token, _ = utils.GetEnvSafe("CLOUDFLARE_AUTH_TOKEN")
var c = NewCloudflare(token["CLOUDFLARE_AUTH_TOKEN"])

func TestListRecords(t *testing.T) {
	resp := new(cloudflareResponse)
	if err := c.ListRecords("notes-bin.top", "1234567890", resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("获取域名列表失败")
	}
}

func TestGetZones(t *testing.T) {
	resp := new(cloudflareZoneResponse)
	if err := c.getZones("notes-bin.top", resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("获取域名列表失败")
	}
}

func TestValidateToken(t *testing.T) {
	if err := c.validateToken(); err != nil {
		t.Error(err)
	}
}
