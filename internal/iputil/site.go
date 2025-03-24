package iputil

import (
	"fmt"
	"io"
	"net"
	"net/http"
)

// SiteProvider 从网站获取 IPv6 地址
type SiteProvider struct {
	urls []string `env:"IPv6_URLS"`
}

// NewSiteProvider 创建一个新的 SiteProvider
func NewSiteProvider() *SiteProvider {
	return &SiteProvider{
		urls: []string{"http://ipv6.icanhazip.com/", "http://v6.ident.me/"},
	}
}

// getIPv6FromURL 尝试从指定 URL 获取 IPv6 地址
func getIPv6FromURL(url string) (net.IP, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", url, err)
	}

	ip := net.ParseIP(string(body))
	if ip != nil && ip.To16() != nil {
		return ip, nil
	}
	return nil, fmt.Errorf("no valid IPv6 address found from %s", url)
}

// GetIPv6Addr 获取网站的 IPv6 地址
func (s *SiteProvider) GetIPv6Addr() (net.IP, error) {
	for _, url := range s.urls {
		ip, err := getIPv6FromURL(url)
		if err == nil {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("failed to get IPv6 address from all sites")
}
