package domain

import (
	"context"
	"errors"
	"net"
	"testing"
)

var ErrIPv6NotChanged = errors.New("IPv6 地址未改变")
var ErrIPv6Changed = errors.New("IPv6 地址已改变")

type mockIPv6Getter struct {
	addr net.IP
	err  error
}

func (m *mockIPv6Getter) GetIPv6Addr() (net.IP, error) {
	return m.addr, m.err
}

type mockTasker struct {
	err error
}

func (m *mockTasker) Task(domain, subdomain, ipv6addr string) error {
	return m.err
}

func TestDomain_UpdateRecord(t *testing.T) {
	ctx := context.Background()
	ip := net.ParseIP("2001:db8::1")

	tests := []struct {
		name        string
		ipv6Getter  IPv6Getter
		tasker      Tasker
		expectedErr error
	}{
		{
			name: "获取 IPv6 地址失败",
			ipv6Getter: &mockIPv6Getter{
				err: errors.New("failed to get IPv6 address"),
			},
			expectedErr: errors.New("failed to get IPv6 address"),
		},
		{
			name: "IPv6 地址未改变",
			ipv6Getter: &mockIPv6Getter{
				addr: ip,
			},
			tasker:      &mockTasker{err: ErrIPv6NotChanged},
			expectedErr: nil,
		},
		{
			name: "配置 DNS 失败",
			ipv6Getter: &mockIPv6Getter{
				addr: ip,
			},
			tasker:      &mockTasker{err: errors.New("failed to update DNS record")},
			expectedErr: errors.New("failed to update DNS record"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Domain{
				Domain:    "example.com",
				SubDomain: "sub",
				Type:      "AAAA",
			}
			d.UpdateRecord(ctx, tt.ipv6Getter, tt.tasker, ErrIPv6NotChanged)
			if d.Err != nil && d.Err.Error() != tt.expectedErr.Error() {
				t.Errorf("expected error: %v, got: %v", tt.expectedErr, d.Err)
			}
		})
	}
}
