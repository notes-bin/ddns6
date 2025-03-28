// Tencent DDNS 客户端库，用于与腾讯云 DNSPod 服务进行交互。
// 提供了创建、修改、删除和读取 DNS 记录的功能。
// 使用时需要提供 secretId 和 secretKey 进行身份验证。
//
// 示例用法：
// tc := tencent.New("your-secret-id", "your-secret-key")
// err := tc.CreateRecord("example.com", "sub", "123.123.123.123")
//
//	if err != nil {
//	    log.Fatal(err)
//	}
package tencent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type tencentCloudStatus struct {
	Response struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	}
}

func (e *tencentCloudStatus) Error() string {
	return fmt.Sprintf("code: %s, message: %s", e.Response.Error.Code, e.Response.Error.Message)
}

type Record struct {
	RecordId  int    `json:"RecordId"`  // 记录的唯一标识符
	Name      string `json:"Name"`      // 记录的名称
	Value     string `json:"Value"`     // 记录的值
	Status    string `json:"Status"`    // 记录的状态
	UpdatedOn string `json:"UpdatedOn"` // 记录的最后更新时间
	Line      string `json:"Line"`      // 记录所属的线路
	LineId    string `json:"LineId"`    // 线路的唯一标识符
	Type      string `json:"Type"`      // 记录的类型，如 A、CNAME 等
	TTL       int    `json:"TTL"`       // 记录的生存时间
	DefaultNS bool   `json:"DefaultNS"` // 是否为默认命名服务器
}

func (r *Record) String() string {
	return fmt.Sprintf("id: %d, name: %s, type: %s, value: %s", r.RecordId, r.Name, r.Type, r.Value)
}

type tencentCloudResponse struct {
	Response struct {
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
		RecordCountInfo struct {
			TotalCount int `json:"TotalCount"`
		} `json:"RecordCountInfo"`
		RecordList []Record `json:"RecordList"`
	}
}

type tencentRequest struct {
	Domain       string `json:"Domain"`                 // 域名
	DomainId     int    `json:"DomainId,omitempty"`     // 域名 ID
	RecordId     int    `json:"RecordId,omitempty"`     // 记录 ID
	SubDomain    string `json:"SubDomain,omitempty"`    // 子域名
	RecordType   string `json:"RecordType,omitempty"`   // 记录类型
	RecordLine   string `json:"RecordLine,omitempty"`   // 记录线路
	RecordLineId string `json:"RecordLineId,omitempty"` // 记录线路 ID
	Value        string `json:"Value,omitempty"`        // 记录值
	TTL          int    `json:"TTL,omitempty"`          // TTL 值
}

func (t *tencentRequest) String() string {
	return fmt.Sprintf("domain: %s, subdomain: %s, type: %s, value: %s", t.Domain, t.SubDomain, t.RecordType, t.Value)
}

type tencent struct {
	secretId  string `env:"Tencent_SecretId" required:"true"`
	secretKey string `env:"Tencent_SecretKey" required:"true"`
	*http.Client
}

const (
	service = "dnspod"
	version = "2021-03-23"
)

var (
	ErrGenerateSignature = errors.New("failed to generate signature")
	ErrIPv6NotChanged    = errors.New("ipv6 address not changed")
	ErrInvalidResponse   = errors.New("invalid response from Tencent API")
	ErrRecordNotFound    = errors.New("record not found")
)

func New() *tencent {
	return &tencent{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (tc *tencent) Task(domain, subdomain, ipv6addr string) error {
	response, status := new(tencentCloudResponse), new(tencentCloudStatus)

	if err := tc.listRecords(domain, response); err != nil {
		return err
	}
	if response.Response.RecordCountInfo.TotalCount == 0 {
		return tc.createRecord(domain, subdomain, ipv6addr, status)
	}
	record := response.Response.RecordList[0]
	if net.ParseIP(record.Value).Equal(net.ParseIP(ipv6addr)) {
		return ErrIPv6NotChanged
	}
	return tc.modfiyRecord(domain, record.RecordId, record.Name, record.Line, ipv6addr, status)
}

func (tc *tencent) listRecords(domain string, response *tencentCloudResponse) error {
	opt := tencentRequest{Domain: domain, RecordType: "AAAA"}
	if err := tc.request(service, "DescribeRecordList", version, &opt, &response); err != nil {
		slog.Debug("获取域名记录列表", "params", fmt.Sprint(opt), "response", response)
		return err
	}

	if response.Response.Error.Code != "" {
		return fmt.Errorf("code: %s, message: %s", response.Response.Error.Code, response.Response.Error.Message)
	}
	return nil
}

func (tc *tencent) createRecord(domain, subDomain, value string, status *tencentCloudStatus) error {
	opt := tencentRequest{Domain: domain, SubDomain: subDomain, RecordType: "AAAA", RecordLine: "默认", Value: value}
	if err := tc.request(service, "CreateRecord", version, &opt, &status); err != nil {
		return err
	}
	if status.Response.Error.Code != "" {
		return status
	}
	return nil
}

func (tc *tencent) modfiyRecord(domain string, recordId int, subDomain, recordLine, value string, status *tencentCloudStatus) error {
	opt := tencentRequest{Domain: domain, SubDomain: subDomain, RecordId: recordId, RecordType: "AAAA", RecordLine: "默认", Value: value}

	if recordLine != "" {
		opt.RecordLine = recordLine
	}

	if err := tc.request(service, "ModifyRecord", version, &opt, &status); err != nil {
		return err
	}
	if status.Response.Error.Code != "" {
		return status
	}
	return nil
}

func (tc *tencent) deleteRecord(Domain string, RecordId int, status *tencentCloudStatus) error {
	opt := tencentRequest{Domain: Domain, RecordId: RecordId}
	if err := tc.request(service, "DeleteRecord", version, &opt, &status); err != nil {
		return err
	}
	if status.Response.Error.Code != "" {
		return status
	}
	return nil
}

func (tc *tencent) request(service, action, version string, params, result any) (err error) {
	jsonStr, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s.tencentcloudapi.com", service), bytes.NewBuffer(jsonStr))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	if err := signature(tc.secretId, tc.secretKey, service, action, version, string(jsonStr), req); err != nil {
		return fmt.Errorf("%w: %v", ErrGenerateSignature, err)
	}

	resp, err := tc.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}
