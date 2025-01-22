// Tencent DDNS 客户端库，用于与腾讯云 DNSPod 服务进行交互。
// 提供了创建、修改、删除和读取 DNS 记录的功能，以及列出和描述域名的功能。
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
	"net/http"
	"time"
)

// Record 结构体表示一个 DNS 记录的详细信息。
// 它包含了记录的唯一标识符、名称、值、状态、更新时间、线路、线路ID、类型、TTL（生存时间）以及是否为默认命名服务器。
type Record struct {
	RecordId  string    `json:"RecordId"`  // 记录的唯一标识符
	Name      string    `json:"Name"`      // 记录的名称
	Value     string    `json:"Value"`     // 记录的值
	Status    string    `json:"Status"`    // 记录的状态
	UpdatedOn time.Time `json:"UpdatedOn"` // 记录的最后更新时间
	Line      string    `json:"Line"`      // 记录所属的线路
	LineId    string    `json:"LineId"`    // 线路的唯一标识符
	Type      string    `json:"Type"`      // 记录的类型，如 A、CNAME 等
	TTL       int       `json:"TTL"`       // 记录的生存时间
	DefaultNS bool      `json:"DefaultNS"` // 是否为默认命名服务器
}

// Response 结构体表示来自腾讯 DDNS 服务的响应。
// 它包含有关记录计数的信息以及记录列表。
//
// 字段:
//   - RecordCountInfo: 包含记录总数的结构体。
//   - TotalCount: 表示记录的总数。
//   - RecordList: 是一个 Record 类型的切片，包含了所有的记录信息。
type Response struct {
	RecordCountInfo struct {
		TotalCount int `json:"TotalCount"`
	} `json:"RecordCountInfo"`
	RecordList []Record `json:"RecordList"`
}

// TencentDomain 结构体表示腾讯云 DNS 记录的相关信息。
// 它包含了域名、记录 ID、子域名、记录类型、记录线路、记录线路 ID、值以及 TTL 等字段。
// 这些字段用于描述和管理 DNS 记录的各种属性。
type tencentDomain struct {
	Domain       string `json:"Domain"`                 // 域名
	DomainId     int    `json:"DomainId,omitempty"`     // 域名 ID
	RecordId     int    `json:"RecordId,omitempty"`     // 记录 ID
	SubDomain    string `json:"SubDomain,omitempty"`    // 子域名
	RecordType   string `json:"RecordType,omitempty"`   // 记录类型
	RecordLine   string `json:"RecordLine,omitempty"`   // 记录线路
	RecordLineId string `json:"RecordLineId,omitempty"` // 记录线路 ID
	Value        string `json:"Value"`                  // 记录值
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
)

func New(secretId, secretKey string) *tencent {
	return &tencent{
		secretId:  secretId,
		secretKey: secretKey,
	}
}

func (tc *tencent) ListDomain(domain string, response Response) error {
	opt := tencentDomain{Domain: domain}
	return tc.request(service, "DescribeDomainList", version, &opt, &response)
}

func (tc *tencent) DescribeDomain(domain string, response Response) error {
	opt := tencentDomain{Domain: domain}
	return tc.request(service, "DescribeDomain", version, &opt, &response)
}

func (tc *tencent) ReadRecord(domain string, recordId int, response Response) error {
	opt := tencentDomain{Domain: domain, RecordId: recordId}
	return tc.request(service, "DescribeRecord", version, &opt, &response)
}

func (tc *tencent) CreateRecord(domain, subDomain, value string) error {
	opt := tencentDomain{Domain: domain, SubDomain: subDomain, RecordType: "AAAA", RecordLine: "默认", Value: value}
	return tc.request(service, "CreateRecord", version, &opt, nil)
}

func (tc *tencent) ModfiyRecord(domain string, recordId int, subDomain, recordLine, value string) error {
	opt := tencentDomain{Domain: domain, SubDomain: "@", RecordId: recordId, RecordType: "AAAA", RecordLine: "默认", Value: value}

	if subDomain != "" {
		opt.SubDomain = subDomain
	}
	if recordLine != "" {
		opt.RecordLine = recordLine
	}
	return tc.request(service, "ModifyRecord", version, &opt, nil)

}

func (tc *tencent) DeleteRecord(Domain string, RecordId int) error {
	opt := tencentDomain{Domain: Domain, RecordId: RecordId}
	return tc.request(service, "DeleteRecord", version, &opt, nil)
}

// request 向腾讯云服务发送HTTP POST请求以执行特定操作。
// service: 服务名称，例如"cdn"。
// action: 要执行的操作，例如"DescribeDomains"。
// version: API版本，例如"2018-06-06"。
// params: 包含请求参数的结构体，不能为空。
// result: 用于接收响应数据的结构体指针。
// 返回值: 如果请求成功，返回nil；否则返回相应的错误。
//
// 该函数首先将请求参数序列化为JSON格式，然后创建一个HTTP POST请求。
// 请求URL由服务名称和端点组成。接着，使用secretId和secretKey对请求进行签名。
// 使用HTTP客户端发送请求，并设置超时时间为30秒。读取响应体并将其反序列化为result参数指定的结构体。
// 如果在任何步骤中遇到错误，函数将返回相应的错误。
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
	if err := signature(tc.secretId, tc.secretKey, service, action, version, req); err != nil {
		return ErrGenerateSignature
	}
	cli := http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	slog.Debug("http response", "response", resp)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return err
	}

	return nil
}
