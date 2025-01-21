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

type tencentDomain struct {
	Domain       string `json:"Domain"`
	DomainId     int    `json:"DomainId,omitempty"`
	RecordId     int    `json:"RecordId,omitempty"`
	SubDomain    string `json:"SubDomain,omitempty"`
	RecordType   string `json:"RecordType,omitempty"`
	RecordLine   string `json:"RecordLine,omitempty"`
	RecordLineId string `json:"RecordLineId,omitempty"`
	Value        string `json:"Value"`
	TTL          int    `json:"TTL,omitempty"`
}

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

func (tc *tencent) RecordList(domain string, response Response) error {
	opt := tencentDomain{Domain: domain}
	return tc.request(service, "DescribeDomainList", version, &opt, &response)
}

func (tc *tencent) RecordRead(domain string, recordId int, response Response) error {
	opt := tencentDomain{Domain: domain, RecordId: recordId}
	return tc.request(service, "DescribeRecord", version, &opt, &response)
}

func (tc *tencent) RecordCreate(domain, subdomain, value string) error {

	opt := tencentDomain{Domain: domain, SubDomain: subdomain, RecordType: "AAAA", RecordLine: "默认", Value: value}
	return tc.request(service, "CreateDomainRecord", version, &opt, nil)
}

func (tc *tencent) RecordModfiy() {

}

func (tc *tencent) RecordDelete() {

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
