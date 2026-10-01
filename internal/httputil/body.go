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

// LimitBody 返回以 MaxResponseBytes 为上限的 Reader；超出时 Read 返回错误。
func LimitBody(r io.Reader) io.Reader {
	return &limitedReader{r: r, rem: MaxResponseBytes}
}

// limitedReader 在超过 max 字节后再读时返回错误（与 ReadBody 语义一致）。
type limitedReader struct {
	r   io.Reader
	rem int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.rem <= 0 {
		// 探测是否还有数据
		var b [1]byte
		n, err := l.r.Read(b[:])
		if n > 0 {
			return 0, fmt.Errorf("response body exceeds %d bytes", MaxResponseBytes)
		}
		return 0, err
	}
	if int64(len(p)) > l.rem {
		p = p[:l.rem]
	}
	n, err := l.r.Read(p)
	l.rem -= int64(n)
	if l.rem == 0 && err == nil {
		var b [1]byte
		if n2, _ := l.r.Read(b[:]); n2 > 0 {
			l.rem = -1
			return n, fmt.Errorf("response body exceeds %d bytes", MaxResponseBytes)
		}
	}
	return n, err
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
