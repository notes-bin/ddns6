// Package aws 实现 Amazon Route 53 DNS API 服务。
//
// 对应 acme.sh dns_aws，使用 AWS Signature Version 4 认证。
package aws

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	serviceName = "route53"
	region      = "us-east-1"
)

// signRequest 为 Route 53 API 请求附加 SigV4 认证头。
func signRequest(req *http.Request, accessKey, secretKey, sessionToken string, body []byte) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	payloadHash := hexSHA256(body)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("host", req.URL.Host)
	if len(body) > 0 {
		req.Header.Set("content-type", "application/x-www-form-urlencoded; charset=utf-8")
	}
	if sessionToken != "" {
		req.Header.Set("x-amz-security-token", sessionToken)
	}

	signedHeaders := []string{"host", "x-amz-date"}
	if sessionToken != "" {
		signedHeaders = append(signedHeaders, "x-amz-security-token")
	}
	if len(body) > 0 {
		signedHeaders = append(signedHeaders, "content-type")
	}
	sort.Strings(signedHeaders)

	canonicalHeaders := strings.Builder{}
	for _, h := range signedHeaders {
		canonicalHeaders.WriteString(h)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(req.Header.Get(h)))
		canonicalHeaders.WriteByte('\n')
	}

	canonicalQuery := req.URL.RawQuery
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, serviceName)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(secretKey, dateStamp, region, serviceName)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	auth := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, strings.Join(signedHeaders, ";"), signature,
	)
	req.Header.Set("Authorization", auth)
	return nil
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func requestPayload(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
