package httputil

import (
	"fmt"
	"io"
)

// MaxResponseBytes 为 DNS API 响应体默认上限（2 MiB）。
const MaxResponseBytes int64 = 2 << 20

// MaxIPResponseBytes 为公网 IP 探测端点响应体上限（4 KiB）。
const MaxIPResponseBytes int64 = 4 << 10

// ReadBody 读取 r 至多 MaxResponseBytes；超出则返回错误，避免无界缓冲。
func ReadBody(r io.Reader) ([]byte, error) {
	return readLimited(r, MaxResponseBytes)
}

// ReadIPBody 读取 IP 探测响应，上限为 MaxIPResponseBytes。
func ReadIPBody(r io.Reader) ([]byte, error) {
	return readLimited(r, MaxIPResponseBytes)
}

// LimitBody 返回以 MaxResponseBytes 为上限的 Reader，供 json.Decoder 等流式消费。
func LimitBody(r io.Reader) io.Reader {
	return io.LimitReader(r, MaxResponseBytes)
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("response body exceeds %d bytes", max)
	}
	return b, nil
}
