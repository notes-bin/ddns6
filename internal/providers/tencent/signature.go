package tencent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	Algorithm     = "TC3-HMAC-SHA256"
	RequestMethod = "POST"
	CanonicalURI  = "/"
	ContentType   = "application/json; charset=utf-8"
)

func sha256hex(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}

func hmacsha256(s, key string) string {
	hashed := hmac.New(sha256.New, []byte(key))
	hashed.Write([]byte(s))
	return string(hashed.Sum(nil))
}

func signature(secretId, secretKey, service, action, version, payload string, r *http.Request) error {
	host := fmt.Sprintf("%s.tencentcloudapi.com", service)
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)

	// step 1: build canonical request string
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		ContentType, host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedRequestPayload := sha256hex(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		RequestMethod,
		CanonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)

	// step 2: build string to sign
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%d\n%s\n%s",
		Algorithm,
		timestamp,
		credentialScope,
		hashedCanonicalRequest)

	// step 3: sign string
	secretDate := hmacsha256(date, "TC3"+secretKey)
	secretService := hmacsha256(service, secretDate)
	secretSigning := hmacsha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacsha256(string2sign, secretSigning)))

	// step 4: build authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		Algorithm,
		secretId,
		credentialScope,
		signedHeaders,
		signature)

	r.Header.Set("Authorization", authorization)
	r.Header.Set("Content-Type", ContentType)
	r.Header.Set("Host", host)
	r.Header.Set("X-TC-Action", action)
	r.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	r.Header.Set("X-TC-Version", version)

	// curl := fmt.Sprintf(`curl -X POST https://%s\
	// 	 -H "Authorization: %s"\
	// 	 -H "Content-Type: application/json; charset=utf-8"\
	// 	 -H "Host: %s" -H "X-TC-Action: %s"\
	// 	 -H "X-TC-Timestamp: %d"\
	// 	 -H "X-TC-Version: %s"\
	// 	 -d '%s'`, host, authorization, host, action, timestamp, version, payload)
	// fmt.Println(curl)
	return nil
}
