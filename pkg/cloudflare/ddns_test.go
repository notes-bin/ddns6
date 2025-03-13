package cloudflare

import (
	"testing"

	"github.com/notes-bin/ddns6/utils/common"
	"github.com/notes-bin/ddns6/utils/network"
)

var c = New()
var domain, _ = common.EnvToString("DOMAIN")

func TestModfiyRecord(t *testing.T) {
	common.EnvToStruct(c, true)
	resp := new(cloudflareResponse)
	ipv6method := network.NewPublicDNS()
	common.EnvToStruct(ipv6method, true)
	ipv6, _ := ipv6method.GetIPV6Addr()
	if err := c.modifyRecord(domain, "6ea09c33602945f8bc582f9bab3646cb", "", ipv6[0].String(), resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("修改记录失败")
	}
}

func TestCreateRecord(t *testing.T) {
	resp := new(cloudflareResponse)
	ipv6method := network.NewPublicDNS()
	common.EnvToStruct(ipv6method, true)
	ipv6, _ := ipv6method.GetIPV6Addr()
	if err := c.createRecord(domain, ipv6[0].String(), "6ea09c33602945f8bc582f9bab3646cb", resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("创建记录失败")
	}
}

func TestListRecords(t *testing.T) {
	resp := new(cloudflareResponse)
	if err := c.listRecords(domain, "6ea09c33602945f8bc582f9bab3646cb", resp); err != nil {
		t.Error(err)
	}
	if len(resp.Result) == 0 {
		t.Error("获取域名列表失败")
	}
}

func TestGetZones(t *testing.T) {
	resp := new(cloudflareZoneResponse)
	if err := c.getZones(domain, resp); err != nil {
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
