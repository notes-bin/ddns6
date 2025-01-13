package tencent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type tencent struct {
	secretId  string
	secretKey string
}

var ErrGenerateSignature = errors.New("failed to generate signature")

func (tc *tencent) request(service, action, version string, params any) error {
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

	return nil
}

func (tc *tencent) String() string {
	return "tencent"
}
