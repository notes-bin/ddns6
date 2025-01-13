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
	RecordId  string
	Value     string
	Status    string
	UpdatedOn time.Time
	Name      string
	Line      string
	LineId    string
	Type      string
	TTL       int
	DefaultNS bool
}

type Response struct {
	RecordCountInfo struct{ TotalCount int }
	RecordList      []Record
}

type dns struct {
	Domain string
	Name   string
	Type   string
	Addr   *net.IP
}

type Domain struct {
	DomainName string `json:"domain_name"`
	SubDomain  string
	RecordType string `json:"record_type"`
	RecordLine string `json:"record_line"`
	Value      string `json:"value"`
	TTL        int
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

func (tc *tencent) RecordList() {
	opt := map[string]any{"Type": "ALL", "Keyword": "dnspod"}
	tc.request(service, "DescribeDomainList", version, &opt, result)

}
func (tc *tencent) RecordCreate() {

}
func (tc *tencent) RecordModfiy() {

}

func (tc *tencent) RecordDelete() {

}

func (tc *tencent) request(service, action, version string, params, result *any) error {
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
