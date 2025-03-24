package tencent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

func SignRequest(secretId, secretKey, service, action, version string, payload []byte, req *http.Request) error {
	// 生成时间戳
	timestamp := time.Now().Unix()
	// 生成随机数
	nonce := fmt.Sprintf("%d", timestamp)

	// 计算签名
	hashed := hmac.New(sha256.New, []byte(secretKey))
	hashed.Write(fmt.Appendf(nil, "%s%d%s%s", secretId, timestamp, nonce, string(payload)))
	signature := hex.EncodeToString(hashed.Sum(nil))

	// 设置请求头
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Nonce", nonce)
	req.Header.Set("X-TC-Region", "ap-guangzhou")
	req.Header.Set("Authorization", fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=content-type;host, Signature=%s", secretId, service, "tc3_request", signature))

	return nil
}
