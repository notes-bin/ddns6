package tencent

import (
	"testing"

	"github.com/notes-bin/ddns6/utils"
)

// 需要设置环境变量 TENCENTCLOUD_SECRET_ID
var secret, _ = utils.GetEnvSafe("TENCENTCLOUD_SECRET_ID", "TENCENTCLOUD_SECRET_KEY", "DOMAIN")
var tc = New(secret["TENCENTCLOUD_SECRET_ID"], secret["TENCENTCLOUD_SECRET_KEY"])
var domain = secret["DOMAIN"]

func TestListRecords(t *testing.T) {
	t.Logf("secret -> %#v\n", secret)
	response := new(tencentCloudResponse)
	err := tc.listRecords(domain, response)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("response -> %#v\n", response)
	}

}

func TestCreateRecord(t *testing.T) {
	response := new(tencentCloudStatus)
	ipv6, _ := utils.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	err := tc.createRecord(domain, "www", ipv6[0].String(), response)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("response -> %#v\n", response)
	}
}

func TestModifyRecord(t *testing.T) {
	response := new(tencentCloudStatus)
	ipv6, _ := utils.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	err := tc.modfiyRecord(domain, 1956278994, "www", "默认", ipv6[0].String(), response)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("response -> %#v\n", response)
	}
}

func TestDeleteRecord(t *testing.T) {
	response := new(tencentCloudStatus)
	err := tc.deleteRecord(domain, 1959780499, response)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("response -> %#v\n", response)
	}
}

func TestTask(t *testing.T) {
	ipv6, _ := utils.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	err := tc.Task(domain, "www", ipv6[0].String())
	if err != nil {
		t.Error(err)
	}
}
