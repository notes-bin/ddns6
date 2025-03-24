package cloudflare

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 测试 New 函数
func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Errorf("New() 返回了 nil，期望一个有效的 *cloudflare 实例")
	}
	// 在检查 c.Client 之前，先检查 c 是否为 nil
	if c != nil && c.Client == nil {
		t.Errorf("New() 创建的客户端为 nil，期望一个有效的 *http.Client 实例")
	}
}

// 测试 validateToken 函数
func TestValidateToken(t *testing.T) {
	// 创建一个模拟的 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟成功的响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"result": {
				"id": "test-id",
				"status": "active",
				"not_before": "2025-01-01T00:00:00Z",
				"expires_on": "2026-01-01T00:00:00Z"
			},
			"success": true,
			"errors": [],
			"messages": []
		}`))
	}))
	defer server.Close()

	c := New()
	c.Token = "test-token"
	// 替换 endpoint 为模拟服务器的地址
	// endpoint = server.URL

	err := c.validateToken()
	if err != nil {
		t.Errorf("validateToken() 返回错误: %v，期望 nil", err)
	}
}

// 测试 getZones 函数
func TestGetZones(t *testing.T) {
	// 创建一个模拟的 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟成功的响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"result": [
				{
					"id": "test-zone-id",
					"name": "test-domain.com",
					"status": "active"
				}
			],
			"result_info": {
				"count": 1,
				"page": 1,
				"per_page": 50,
				"total_count": 1
			},
			"success": true,
			"errors": [],
			"messages": []
		}`))
	}))
	defer server.Close()

	c := New()
	c.Token = "test-token"
	// 替换 endpoint 为模拟服务器的地址
	// endpoint = server.URL

	response := new(cloudflareZoneResponse)
	err := c.getZones("test-domain.com", response)
	if err != nil {
		t.Errorf("getZones() 返回错误: %v，期望 nil", err)
	}
	if len(response.Result) == 0 {
		t.Errorf("getZones() 返回的区域信息为空，期望至少有一个区域信息")
	}
}

// 可以继续为其他函数编写类似的测试，如 listRecords, createRecord, modifyRecord, deleteRecord, Task 等
