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
		site: []string{"http://ipv6.icanhazip.com/", "http://ifconfig.co", "http://v6.ident.me/"},
		dns:  []string{"2400:3200:baba::1", "2606:4700:4700::1111", "2001:4860:4860::8888"},
	}

	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func (req RequestPool) GetIPv6Addr(ctx context.Context) (net.IP, error) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	ipChan := make(chan net.IP, 1)
	errChan := make(chan error, 2)

	go func(ipChan chan<- net.IP, errChan chan<- error) {
		ip, err := getIPv6FromSiteService(req.site[rnd.Intn(3)])
		if err != nil {
			errChan <- err
			return
		}
		ipChan <- ip
	}(ipChan, errChan)

	go func(ipChan chan<- net.IP, errChan chan<- error) {
		ip, err := getIPv6FromDnsService(req.dns[rand.Intn(3)])
		if err != nil {
			errChan <- err
			return
		}
		ipChan <- ip
	}(ipChan, errChan)

	select {
	case ip := <-ipChan:
		return ip, nil
	case err := <-errChan:
		return nil, fmt.Errorf("failed to obtain local IPv6 address: %w", err)
	}

}

// getIPv6FromSiteService 通过访问指定的 URL 获取本地 IPv6 地址
func getIPv6FromSiteService(url string) (net.IP, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", url, err)
	}

	if bytes.Contains(body, []byte("%")) {
		body = bytes.Trim(body, "%")
	}

	ip := net.ParseIP(string(body))
	if ip != nil && ip.To16() != nil {
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
	if localAddr.IP.To16() != nil {

		return localAddr.IP, nil
	}
	return nil, fmt.Errorf("no valid IPv6 address found from server %s", server)
}
