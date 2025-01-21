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

type Record struct {
	RecordId  string    `json:"RecordId"`
	Name      string    `json:"Name"`
	Value     string    `json:"Value"`
	Status    string    `json:"Status"`
	UpdatedOn time.Time `json:"UpdatedOn"`
	Line      string    `json:"Line"`
	LineId    string    `json:"LineId"`
	Type      string    `json:"Type"`
	TTL       int       `json:"TTL"`
	DefaultNS bool      `json:"DefaultNS"`
}

type Response struct {
	RecordCountInfo struct {
		TotalCount int `json:"TotalCount"`
	} `json:"RecordCountInfo"`
	RecordList []Record `json:"RecordList"`
}

type dns struct {
	Domain string
	Name   string
	Type   string
	Addr   *net.IP
}

type tencentDomain struct {
	Domain       string `json:"Domain"`
	DomainId     int    `json:"DomainId,omitempty"`
	SubDomain    string `json:"SubDomain,omitempty"`
	RecordType   string `json:"RecordType,omitempty"`
	RecordLine   string `json:"RecordLine,omitempty"`
	RecordLineId string `json:"RecordLineId,omitempty"`
	Value        string `json:"Value,omitempty"`
	TTL          int    `json:"TTL,omitempty"`
}

type tencent struct {
	secretId  string
	secretKey string
	dns
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

func (tc *tencent) RecordList(opt tencentDomain, resp Response) error {
	// `{"Domain": "notes-bin.top"}`
	return tc.request(service, "DescribeDomainList", version, &opt, &resp)
}

func (tc *tencent) RecordRead() {
	// record := `{"Domain": "notes-bin.top","RecordId": 1342341821}`
}

func (tc *tencent) RecordCreate() {
	// record := `{
	// 	"Domain": "notes-bin.top",
	// 	"SubDomain": "www",
	// 	"RecordType": "AAAA",
	// 	"RecordLine": "默认",
	// 	"RecordLineId": "0",
	// 	"Value": "129.23.32.32"
	// 	}`

}
func (tc *tencent) RecordModfiy() {
	// record := `{
	// 	"Domain": "notes-bin.top",
	// 	"SubDomain": "www",
	// 	"RecordType": "AAAA",
	// 	"RecordLine": "默认",
	// 	"Value": "129.23.32.32"
	// 	"RecordId":1342341821,
	// }`

}

func (tc *tencent) RecordDelete() {
	// Record := `{"Domain": "notes-bin.top","RecordId": 1342341821}`

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

func (tc *tencent) String() string {
	return "tencent"
}
