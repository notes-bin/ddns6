// Package alicloud 提供阿里云 DNS API（Alibaba Cloud DNS）客户端实现。
//
// 本文件实现阿里云 V3 签名机制（ACS3-HMAC-SHA256），用于替代原有的 V1 签名。
package alicloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// V3Request V3 签名机制的请求参数。
type V3Request struct {
	// AccessKeyId 阿里云 AccessKey ID
	AccessKeyId string
	// AccessKeySecret 阿里云 AccessKey Secret
	AccessKeySecret string
	// SecurityToken 可选：STS 临时安全令牌（使用 RAM 角色时必填）
	SecurityToken string
	// Method HTTP 方法（GET / POST / PUT / DELETE）
	Method string
	// Host API 端点主机名（如 alidns.aliyuncs.com）
	Host string
	// Path 请求路径，默认为 "/"
	Path string
	// QueryParams 查询参数
	QueryParams map[string]string
	// Headers 请求头。
	// 必须包含 x-acs-action（API 名称）和 x-acs-version（API 版本）。
	// content-type 如未设置，默认为 application/x-www-form-urlencoded（有 body 时）。
	// x-acs-date、x-acs-content-sha256、x-acs-signature-nonce 由 SignV3 自动填充。
	Headers map[string]string
	// Body 请求体字节序列（可选）
	Body []byte
}

// SignV3 使用阿里云 V3 签名机制（ACS3-HMAC-SHA256）创建签名的 HTTP 请求。
//
// 签名流程：
//  1. 填充必要请求头（x-acs-date、x-acs-content-sha256、x-acs-signature-nonce）
//  2. 构建 CanonicalRequest（规范化请求）
//  3. 构建 StringToSign（待签字符串）
//  4. 计算 Signature（HMAC-SHA256）
//  5. 组装 Authorization 请求头
//  6. 返回完整的 *http.Request
func SignV3(ctx context.Context, req *V3Request) (*http.Request, error) {
	now := time.Now().UTC()

	// === 1. 填充请求头 ===
	headers := make(map[string]string, len(req.Headers)+6)
	for k, v := range req.Headers {
		headers[k] = v
	}
	headers["host"] = req.Host
	headers["x-acs-date"] = now.Format("2006-01-02T15:04:05Z")
	headers["x-acs-signature-nonce"] = fmt.Sprintf("%d", now.UnixNano())

	// 如果未设置 content-type 且 body 非空，使用默认值
	if _, ok := headers["content-type"]; !ok && len(req.Body) > 0 {
		headers["content-type"] = "application/x-www-form-urlencoded"
	}

	// 计算 body SHA-256 哈希
	bodyHash := sha256Hex(req.Body)
	headers["x-acs-content-sha256"] = bodyHash

	// 可选 STS token
	if req.SecurityToken != "" {
		headers["x-acs-security-token"] = req.SecurityToken
	}

	// === 2. 构建 CanonicalRequest ===
	canonicalURI := req.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := buildCanonicalQueryStringV3(req.QueryParams)
	canonicalHeaders, signedHeaders := buildCanonicalHeadersV3(headers)
	hashedPayload := bodyHash

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		strings.ToUpper(req.Method),
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	)

	// === 3. 构建 StringToSign ===
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("ACS3-HMAC-SHA256\n%s", hashedCanonicalRequest)

	// === 4. 计算 Signature ===
	mac := hmac.New(sha256.New, []byte(req.AccessKeySecret))
	mac.Write([]byte(stringToSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	// === 5. 构建 Authorization ===
	authorization := fmt.Sprintf(
		"ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		req.AccessKeyId, signedHeaders, signature,
	)
	headers["Authorization"] = authorization

	// === 6. 创建 HTTP 请求 ===
	scheme := "https"
	rawURL := fmt.Sprintf("%s://%s%s", scheme, req.Host, canonicalURI)
	if canonicalQueryString != "" {
		rawURL = fmt.Sprintf("%s?%s", rawURL, canonicalQueryString)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 设置请求头
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	// 设置请求体
	if len(req.Body) > 0 {
		httpReq.Body = io.NopCloser(bytes.NewReader(req.Body))
		httpReq.ContentLength = int64(len(req.Body))
	}

	return httpReq, nil
}

// sha256Hex 计算数据的 SHA-256 哈希并返回十六进制编码字符串。
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// buildCanonicalQueryStringV3 构建 V3 签名所需的规范化查询字符串。
//
// 按键名升序排列，键和值分别进行 RFC 3986 百分号编码。
func buildCanonicalQueryStringV3(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(percentEncode(k))
		buf.WriteByte('=')
		buf.WriteString(percentEncode(params[k]))
	}
	return buf.String()
}

// buildCanonicalHeadersV3 构建 V3 签名所需的规范化请求头。
//
// 仅保留需要签名的头：host、content-type 及所有 x-acs-* 前缀的头。
// 头名转小写并排序，值去除首尾空格。
// 返回规范头字符串（以 \n 结尾）和签名头列表（";" 分隔）。
func buildCanonicalHeadersV3(headers map[string]string) (canonicalHeaders, signedHeaders string) {
	// 筛选并规范化头
	normalized := make(map[string]string)
	for k, v := range headers {
		lowerKey := strings.ToLower(k)
		// V3 签名范围：host、content-type、所有 x-acs-* 头
		if lowerKey == "host" || lowerKey == "content-type" || strings.HasPrefix(lowerKey, "x-acs-") {
			normalized[lowerKey] = strings.TrimSpace(v)
		}
	}

	// 确保 host 存在
	if _, ok := normalized["host"]; !ok {
		normalized["host"] = ""
	}

	// 排序
	names := make([]string, 0, len(normalized))
	for k := range normalized {
		names = append(names, k)
	}
	sort.Strings(names)

	// 构建输出
	var cBuf, sBuf strings.Builder
	for i, n := range names {
		if i > 0 {
			sBuf.WriteByte(';')
		}
		sBuf.WriteString(n)

		cBuf.WriteString(n)
		cBuf.WriteByte(':')
		cBuf.WriteString(normalized[n])
		cBuf.WriteByte('\n')
	}

	return cBuf.String(), sBuf.String()
}

// percentEncode 实现 RFC 3986 百分号编码。
//
// Go 标准库 url.QueryEscape 使用 application/x-www-form-urlencoded 编码，
// 空格编码为 +（应编码为 %20），* 不编码（应编码为 %2A），
// ~ 编码为 %7E（实际上 unreserved 字符无需编码）。
// 本函数修正这三种差异以满足 RFC 3986。
func percentEncode(s string) string {
	if s == "" {
		return ""
	}
	result := url.QueryEscape(s)
	result = strings.ReplaceAll(result, "+", "%20")
	result = strings.ReplaceAll(result, "*", "%2A")
	result = strings.ReplaceAll(result, "%7E", "~")
	return result
}
