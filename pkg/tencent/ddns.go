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

var service = "dnspod"

var ErrGenerateSignature = errors.New("failed to generate signature")

func (tc *tencent) RecordList() {

}
func (tc *tencent) RecordCreate() {

}
func (tc *tencent) RecordModfiy() {

}

func (tc *tencent) RecordDelete() {

}

func (tc *tencent) request(service, action, version string, params, result any) error {
	jsonStr := make([]byte, 0)
	if params != nil {
		jsonStr, _ = json.Marshal(params)
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

	result, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (tc *tencent) String() string {
	return "tencent"
}
