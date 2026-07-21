// Package huaweicloud 实现华为云 DNS API 服务
//
// signer.go 实现华为云 SDK-HMAC-SHA256 签名算法
// 参考：https://support.huaweicloud.com/api-dns/dns_api_64001.html
// 基于 ddns-go 的 huawei_signer.go 移植
package huaweicloud

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"github.com/notes-bin/ddns6/internal/crypto"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	basicDateTimeFormat = "20060102T150405Z"
	sdkAlgorithm        = "SDK-HMAC-SHA256"
	headerXDate         = "X-Sdk-Date"
	headerHost          = "host"
	headerAuthorization = "Authorization"
	headerContentSha256 = "X-Sdk-Content-Sha256"
)

// hmacsha256 计算 HMAC-SHA256
func hmacsha256(key []byte, data string) ([]byte, error) {
	h := hmac.New(sha256.New, key)
	if _, err := h.Write([]byte(data)); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// canonicalRequest 构建规范化请求字符串
//
// CanonicalRequest =
//
//	HTTPRequestMethod + '\n' +
//	CanonicalURI + '\n' +
//	CanonicalQueryString + '\n' +
//	CanonicalHeaders + '\n' +
//	SignedHeaders + '\n' +
//	HexEncode(Hash(RequestPayload))
func canonicalRequest(r *http.Request, signedHeaders []string) (string, error) {
	var hexencode string
	var err error
	if hex := r.Header.Get(headerContentSha256); hex != "" {
		hexencode = hex
	} else {
		data, err := requestPayload(r)
		if err != nil {
			return "", err
		}
		hexencode, err = hexEncodeSHA256Hash(data)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		r.Method,
		canonicalURI(r),
		canonicalQueryString(r),
		canonicalHeaders(r, signedHeaders),
		strings.Join(signedHeaders, ";"),
		hexencode,
	), err
}

// canonicalURI 返回规范化的 URI
func canonicalURI(r *http.Request) string {
	patterns := strings.Split(r.URL.Path, "/")
	var uri []string
	for _, v := range patterns {
		uri = append(uri, url.PathEscape(v))
	}
	urlpath := strings.Join(uri, "/")
	if len(urlpath) == 0 || urlpath[len(urlpath)-1] != '/' {
		urlpath += "/"
	}
	return urlpath
}

// canonicalQueryString 返回规范化的查询字符串（按键排序）
func canonicalQueryString(r *http.Request) string {
	var keys []string
	query := r.URL.Query()
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var a []string
	for _, key := range keys {
		k := url.QueryEscape(key)
		sort.Strings(query[key])
		for _, v := range query[key] {
			kv := fmt.Sprintf("%s=%s", k, url.QueryEscape(v))
			a = append(a, kv)
		}
	}
	queryStr := strings.Join(a, "&")
	r.URL.RawQuery = queryStr
	return queryStr
}

// canonicalHeaders 返回规范化请求头字符串
func canonicalHeaders(r *http.Request, signerHeaders []string) string {
	var a []string
	header := make(map[string][]string)
	for k, v := range r.Header {
		header[strings.ToLower(k)] = v
	}
	for _, key := range signerHeaders {
		value := header[key]
		if strings.EqualFold(key, headerHost) {
			value = []string{r.Host}
		}
		sort.Strings(value)
		for _, v := range value {
			a = append(a, key+":"+strings.TrimSpace(v))
		}
	}
	return fmt.Sprintf("%s\n", strings.Join(a, "\n"))
}

// signedHeaders 返回排序后的签名头列表
func signedHeaders(r *http.Request) []string {
	var a []string
	for key := range r.Header {
		a = append(a, strings.ToLower(key))
	}
	sort.Strings(a)
	return a
}

// requestPayload 读取请求体并重置 Body
func requestPayload(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte(""), nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return []byte(""), err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(b))
	return b, nil
}

// stringToSign 构建待签名字符串
func stringToSign(canonicalRequest string, t time.Time) (string, error) {
	hash := sha256.New()
	_, err := hash.Write([]byte(canonicalRequest))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s\n%s\n%x",
		sdkAlgorithm, t.UTC().Format(basicDateTimeFormat), hash.Sum(nil)), nil
}

// signStringToSign 签名待签名字符串
func signStringToSign(stringToSign string, signingKey []byte) (string, error) {
	hm, err := hmacsha256(signingKey, stringToSign)
	return fmt.Sprintf("%x", hm), err
}

// hexEncodeSHA256Hash 计算 SHA256 并返回十六进制字符串
func hexEncodeSHA256Hash(body []byte) (string, error) {
	if body == nil {
		body = []byte("")
	}
	return crypto.SHA256Hex(body), nil
}

// authHeaderValue 生成 Authorization 头的值
func authHeaderValue(signature, accessKey string, signedHeaders []string) string {
	return fmt.Sprintf("%s Access=%s, SignedHeaders=%s, Signature=%s",
		sdkAlgorithm, accessKey, strings.Join(signedHeaders, ";"), signature)
}

// Signer 华为云 SDK-HMAC-SHA256 签名器
type Signer struct {
	Key    string
	Secret string
}

// Sign 为 HTTP 请求添加 SDK-HMAC-SHA256 签名
func (s *Signer) Sign(r *http.Request) error {
	var t time.Time
	var err error
	var dt string
	if dt = r.Header.Get(headerXDate); dt != "" {
		t, err = time.Parse(basicDateTimeFormat, dt)
	}
	if err != nil || dt == "" {
		t = time.Now()
		r.Header.Set(headerXDate, t.UTC().Format(basicDateTimeFormat))
	}
	signedHeaders := signedHeaders(r)
	canonicalReq, err := canonicalRequest(r, signedHeaders)
	if err != nil {
		return err
	}
	stringToSign, err := stringToSign(canonicalReq, t)
	if err != nil {
		return err
	}
	signature, err := signStringToSign(stringToSign, []byte(s.Secret))
	if err != nil {
		return err
	}
	authValue := authHeaderValue(signature, s.Key, signedHeaders)
	r.Header.Set(headerAuthorization, authValue)
	return nil
}
