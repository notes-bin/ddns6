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

func TestListDomain(t *testing.T) {
	var response = new(Response)
	err := tc.ListDomain(domain, response)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}

func TestDescribeDomain(t *testing.T) {
	var response = new(Response)
	err := tc.DescribeDomain(domain, response)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}

func TestReadRecord(t *testing.T) {
	var response = new(Response)
	err := tc.ReadRecord(domain, 123456, response)
	if err != nil {
		t.Error(err)
	}
	t.Log(response)
}

func TestCreateRecord(t *testing.T) {

}

func TestModifyRecord(t *testing.T) {

}

func TestDeleteRecord(t *testing.T) {

}
