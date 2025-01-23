package tencent

import (
	"os"
	"testing"
)

// 需要设置环境变量 TENCENTCLOUD_SECRET_ID
var secretId = os.Getenv("TENCENTCLOUD_SECRET_ID")

// 需要设置环境变量 TENCENTCLOUD_SECRET_KEY
var secretKey = os.Getenv("TENCENTCLOUD_SECRET_KEY")

// 需要设置域名变量 Domain
var domain = os.Getenv("DOMAIN")

var tc = New(secretId, secretKey)

func TestRequest(t *testing.T) {

}

func TestListRecords(t *testing.T) {
	var response = new(TencentCloudResponse)
	err := tc.ListRecords(domain, response)
	if err != nil {
		t.Error(err)
	}
	t.Logf("response -> %#v\n", response)
}

func TestReadRecord(t *testing.T) {
	var response = new(TencentCloudResponse)
	err := tc.ReadRecord(domain, 123456, response)
	if err != nil {
		t.Error(err)
	}
	t.Logf("response -> %#v\n", response)
}

func TestModifyRecord(t *testing.T) {

}

func TestDeleteRecord(t *testing.T) {

}
