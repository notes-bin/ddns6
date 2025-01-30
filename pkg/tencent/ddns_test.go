package tencent

import (
	"os"
	"testing"

	"github.com/notes-bin/ddns6/utils"
)

// 需要设置环境变量 TENCENTCLOUD_SECRET_ID
var secretId = os.Getenv("TENCENTCLOUD_SECRET_ID")

// 需要设置环境变量 TENCENTCLOUD_SECRET_KEY
var secretKey = os.Getenv("TENCENTCLOUD_SECRET_KEY")

// 需要设置域名变量 Domain
var domain = os.Getenv("DOMAIN")

var tc = New(secretId, secretKey)

func TestListRecords(t *testing.T) {
	response := new(TencentCloudResponse)
	err := tc.ListRecords(domain, response)
	if err != nil {
		t.Error(err)
	}
	t.Logf("response -> %#v\n", response)
}

func TestCreateRecord(t *testing.T) {
	response := new(TencentCloudStatus)
	ipv6, _ := utils.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	err := tc.CreateRecord(domain, "www", ipv6[0].String(), response)
	if err != nil {
		t.Error(err)
	}
	t.Logf("response -> %#v\n", response)
}

func TestModifyRecord(t *testing.T) {
	response := new(TencentCloudStatus)
	ipv6, _ := utils.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	err := tc.ModfiyRecord(domain, 1956278994, "www", "默认", ipv6[0].String(), response)
	if err != nil {
		t.Error(err)
	}
	t.Logf("response -> %#v\n", response)
}

func TestDeleteRecord(t *testing.T) {
	response := new(TencentCloudStatus)
	err := tc.DeleteRecord(domain, 1959780499, response)
	if err != nil {
		t.Error(err)
	}
	t.Logf("response -> %#v\n", response)
}

func TestTask(t *testing.T) {
	ipv6, _ := utils.NewPublicDNS("2400:3200:baba::1").GetIPV6Addr()
	err := tc.Task(domain, "www", ipv6[0].String())
	if err != nil {
		t.Error(err)
	}
}
