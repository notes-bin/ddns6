package ipaddr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

type RequestPool struct {
	site []string
	dns  []string
}

type Option func(*RequestPool)

func WithSite(url string) Option {
	return func(r *RequestPool) {
		r.site = append(r.site, url)
	}
}

func WithDns(ipv6 string) Option {
	return func(r *RequestPool) {
		r.dns = append(r.dns, ipv6)
	}
}

func New(opts ...Option) RequestPool {
	req := RequestPool{
		site: []string{"https://ipv6.icanhazip.com/", "https://ifconfig.co", "https://v6.ident.me/"},
		dns:  []string{"2400:3200:baba::1", "2606:4700:4700::1111", "2001:4860:4860::8888"},
	}

	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func (req RequestPool) GetIPv6Addr(ctx context.Context) (net.IP, error) {
	// 设置默认超时
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	ipChan := make(chan net.IP, 2)
	errChan := make(chan error, 2)

	// 尝试从网站服务获取 IPv6 地址
	if len(req.site) > 0 {
		go func(ipChan chan<- net.IP, errChan chan<- error) {
			select {
			case <-ctx.Done():
				return
			default:
				ip, err := getIPv6FromSiteService(req.site[rnd.Intn(len(req.site))])
				if err != nil {
					errChan <- err
					return
				}
				ipChan <- ip
			}
		}(ipChan, errChan)
	}

	// 尝试从 DNS 服务获取 IPv6 地址
	if len(req.dns) > 0 {
		go func(ipChan chan<- net.IP, errChan chan<- error) {
			select {
			case <-ctx.Done():
				return
			default:
				ip, err := getIPv6FromDnsService(req.dns[rnd.Intn(len(req.dns))])
				if err != nil {
					errChan <- err
					return
				}
				ipChan <- ip
			}
		}(ipChan, errChan)
	}

	// 收集错误
	var errors []error

	// 等待结果或超时
	for i := 0; i < 2; i++ {
		select {
		case ip := <-ipChan:
			return ip, nil
		case err := <-errChan:
			errors = append(errors, err)
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		}
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("all attempts failed: %v", errors)
	}

	return nil, fmt.Errorf("unexpected error: no results or errors received")
}

// getIPv6FromSiteService 通过访问指定的 URL 获取本地 IPv6 地址
func getIPv6FromSiteService(url string) (net.IP, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", url, err)
	}

	// 清理响应内容
	body = bytes.TrimSpace(body)
	if bytes.Contains(body, []byte("%")) {
		body = bytes.Trim(body, "%")
	}

	ip := net.ParseIP(string(body))
	if ip != nil && ip.To16() != nil && ip.To4() == nil {
		// 确保是 IPv6 地址且不是 IPv4-mapped IPv6 地址
		return ip, nil
	}

	return nil, fmt.Errorf("no valid IPv6 address found from %s", url)
}

// getIPv6FromDnsService 通过连接到指定的 DNS 服务器获取本地 IPv6 地址
func getIPv6FromDnsService(server string) (net.IP, error) {
	conn, err := net.Dial("udp6", fmt.Sprintf("[%s]:53", server))
	if err != nil {
		return nil, fmt.Errorf("failed to dial DNS server %s: %w", server, err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	if localAddr.IP.To16() != nil && localAddr.IP.To4() == nil {
		// 确保是 IPv6 地址且不是 IPv4-mapped IPv6 地址
		return localAddr.IP, nil
	}
	return nil, fmt.Errorf("no valid IPv6 address found from server %s", server)
}
