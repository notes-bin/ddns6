package tencent

import (
	"testing"

	"github.com/notes-bin/ddns6/utils/common"
	"github.com/notes-bin/ddns6/utils/network"
)

// 需要设置环境变量 TENCENTCLOUD_SECRET_ID
var tc = New()
var domain, _ = common.EnvToString("DOMAIN")

func TestListRecords(t *testing.T) {
	common.EnvToStruct(tc, true)
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
	ipv6method := network.NewPublicDNS()
	common.EnvToStruct(ipv6method, true)
	ipv6, _ := ipv6method.GetIPV6Addr()
	err := tc.createRecord(domain, "www", ipv6[0].String(), response)
	if err != nil {
		t.Error(err)
	} else {
		t.Logf("response -> %#v\n", response)
	}
}

func TestModifyRecord(t *testing.T) {
	response := new(tencentCloudStatus)
	ipv6method := network.NewPublicDNS()
	common.EnvToStruct(ipv6method, true)
	ipv6, _ := ipv6method.GetIPV6Addr()
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
	ipv6method := network.NewPublicDNS()
	common.EnvToStruct(ipv6method, true)
	ipv6, _ := ipv6method.GetIPV6Addr()
	err := tc.Task(domain, "www", ipv6[0].String())
	if err != nil {
		t.Error(err)
	}
}
