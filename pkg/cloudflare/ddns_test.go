package cloudflare

import (
	"testing"

	"github.com/notes-bin/ddns6/utils/common"
	"github.com/notes-bin/ddns6/utils/network"
)

var token, _ = common.GetEnvSafe("CLOUDFLARE_AUTH_TOKEN")
var c = NewCloudflare(token["CLOUDFLARE_AUTH_TOKEN"])

func TestModfiyRecord(t *testing.T) {
	resp := new(cloudflareResponse)
	ipv6, _ := network.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	if err := c.modifyRecord("www.notes-bin.top", "6ea09c33602945f8bc582f9bab3646cb", "", ipv6[0].String(), resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("修改记录失败")
	}
}

func TestCreateRecord(t *testing.T) {
	resp := new(cloudflareResponse)
	ipv6, _ := network.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	if err := c.createRecord("www.notes-bin.top", ipv6[0].String(), "6ea09c33602945f8bc582f9bab3646cb", resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("创建记录失败")
	}
}

func TestListRecords(t *testing.T) {
	resp := new(cloudflareResponse)
	if err := c.listRecords("notes-bin.top", "6ea09c33602945f8bc582f9bab3646cb", resp); err != nil {
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
	} else {
		t.Logf("%+v", resp)
	}
}

func TestValidateToken(t *testing.T) {
	if err := c.validateToken(); err != nil {
		t.Error(err)
	}
}
