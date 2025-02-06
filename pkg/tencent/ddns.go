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

// Record 结构体表示一个 DNS 记录的详细信息。
// 它包含了记录的唯一标识符、名称、值、状态、更新时间、线路、线路ID、类型、TTL（生存时间）以及是否为默认命名服务器。
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

type TencentCloudResponse struct {
	Response struct {
		RecordCountInfo struct {
			TotalCount int `json:"TotalCount"`
		} `json:"RecordCountInfo"`
		RecordList []Record `json:"RecordList"`
	}
}

type TencentCloudStatus struct {
	Error struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	} `json:"Error"`
}

func (e *TencentCloudStatus) ErrMessage() error {
	if e.Error.Code != "" {
		return fmt.Errorf("code: %s, message: %s", e.Error.Code, e.Error.Message)
	}
	return nil
}

// TencentDomain 结构体表示腾讯云 DNS 记录的相关信息。
// 它包含了域名、记录 ID、子域名、记录类型、记录线路、记录线路 ID、值以及 TTL 等字段。
// 这些字段用于描述和管理 DNS 记录的各种属性。
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

// Tencent 结构体包含了用于腾讯云 API 调用的认证信息。
// secretId 是用户的唯一标识符，用于身份验证。
// secretKey 是与 secretId 配对使用的密钥，用于加密签名验证。
type tencent struct {
	secretId  string
	secretKey string
}

const (
	service = "dnspod"
	version = "2021-03-23"
)

var (
	ErrGenerateSignature     = errors.New("failed to generate signature")
	ErrNotEmptyRequestParams = errors.New("not empty request params")
	ErrIPv6NotChanged        = errors.New("ipv6 address not changed")
)

func New(secretId, secretKey string) *tencent {
	return &tencent{
		secretId:  secretId,
		secretKey: secretKey,
	}
}

func (tc *tencent) Task(domain, subdomain, ipv6addr string) error {
	response, status := new(TencentCloudResponse), new(TencentCloudStatus)
	tc.ListRecords(domain, response)
	if response.Response.RecordCountInfo.TotalCount == 0 {
		return tc.CreateRecord(domain, subdomain, ipv6addr, status)

	}
	record := response.Response.RecordList[0]
	if net.ParseIP(record.Value).Equal(net.ParseIP(ipv6addr)) {
		return ErrIPv6NotChanged
	}
	return tc.ModfiyRecord(domain, record.RecordId, record.Name, record.Line, ipv6addr, status)
}

func (tc *tencent) ListRecords(domain string, response *TencentCloudResponse) error {
	opt := tencentRequest{Domain: domain, RecordType: "AAAA"}
	return tc.request(service, "DescribeRecordList", version, &opt, &response)
}

func (tc *tencent) CreateRecord(domain, subDomain, value string, status *TencentCloudStatus) error {
	opt := tencentRequest{Domain: domain, SubDomain: subDomain, RecordType: "AAAA", RecordLine: "默认", Value: value}
	if err := tc.request(service, "CreateRecord", version, &opt, &status); err != nil {
		return err
	}
	return status.ErrMessage()
}

func (tc *tencent) ModfiyRecord(domain string, recordId int, subDomain, recordLine, value string, status *TencentCloudStatus) error {
	opt := tencentRequest{Domain: domain, SubDomain: subDomain, RecordId: recordId, RecordType: "AAAA", RecordLine: "默认", Value: value}

	if recordLine != "" {
		opt.RecordLine = recordLine
	}

	if err := tc.request(service, "ModifyRecord", version, &opt, &status); err != nil {
		return err
	}
	return status.ErrMessage()
}

func (tc *tencent) DeleteRecord(Domain string, RecordId int, status *TencentCloudStatus) error {
	opt := tencentRequest{Domain: Domain, RecordId: RecordId}
	if err := tc.request(service, "DeleteRecord", version, &opt, &status); err != nil {
		return err
	}
	return status.ErrMessage()
}

func (tc *tencent) request(service, action, version string, params, result any) error {
	if params == nil {
		return ErrNotEmptyRequestParams
	}

	jsonStr, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("https://%s.%s", service, endpoint), bytes.NewBuffer(jsonStr))
	slog.Debug("create http request", "request", req, "error", err)
	if err != nil {
		return err
	}
	if err := signature(tc.secretId, tc.secretKey, service, action, version, string(jsonStr), req); err != nil {
		return ErrGenerateSignature
	}
	cli := http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	slog.Debug("http response", "response", raw, "error", err)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}

	return nil
}
