package domain

import (
	"context"
	"errors"
	"net"
	"testing"
)

// MockIPv6Getter 是 IPv6Getter 接口的模拟实现
type MockIPv6Getter struct {
	Addr net.IP
	Err  error
}

// GetIPv6Addr 实现 IPv6Getter 接口的方法
func (m *MockIPv6Getter) GetIPv6Addr(ctx context.Context) (net.IP, error) {
	return m.Addr, m.Err
}

// MockTasker 是 Tasker 接口的模拟实现
type MockTasker struct {
	Err error
}

// Task 实现 Tasker 接口的方法
func (m *MockTasker) Task(domain, subdomain, ipv6addr string) error {
	return m.Err
}

// TestDomain_UpdateRecord 测试 Domain 结构体的 UpdateRecord 方法
func TestDomain_UpdateRecord(t *testing.T) {
	tests := []struct {
		name         string
		domain       *Domain
		ipv6Getter   IPv6Getter
		tasker       Tasker
		expectedErr  bool
		expectedInfo string
	}{
		{
			name: "更新任务被取消",
			domain: &Domain{
				Domain:    "example.com",
				SubDomain: "sub",
			},
			ipv6Getter: &MockIPv6Getter{},
			tasker:     &MockTasker{},
			expectedErr: func() bool {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx.Err() != nil
			}(),
			expectedInfo: "更新任务被取消",
		},
		{
			name: "获取 IPv6 地址失败",
			domain: &Domain{
				Domain:    "example.com",
				SubDomain: "sub",
			},
			ipv6Getter: &MockIPv6Getter{
				Err: errors.New("模拟获取 IPv6 地址失败"),
			},
			tasker:      &MockTasker{},
			expectedErr: true,
		},
		{
			name: "IPv6 地址未改变",
			domain: &Domain{
				Domain:    "example.com",
				SubDomain: "sub",
				Addr:      net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
			},
			ipv6Getter: &MockIPv6Getter{
				Addr: net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
			},
			tasker:       &MockTasker{},
			expectedErr:  false,
			expectedInfo: "IPv6 地址未改变，无需更新",
		},
		{
			name: "IPv6 地址改变，更新成功",
			domain: &Domain{
				Domain:    "example.com",
				SubDomain: "sub",
				Addr:      net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7333"),
			},
			ipv6Getter: &MockIPv6Getter{
				Addr: net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
			},
			tasker:       &MockTasker{},
			expectedErr:  false,
			expectedInfo: "IPv6 地址发生变化，DDNS 配置完成",
		},
		{
			name: "IPv6 地址改变，更新失败",
			domain: &Domain{
				Domain:    "example.com",
				SubDomain: "sub",
				Addr:      net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7333"),
			},
			ipv6Getter: &MockIPv6Getter{
				Addr: net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
			},
			tasker: &MockTasker{
				Err: errors.New("模拟更新 DNS 记录失败"),
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.expectedInfo == "更新任务被取消" {
				_, cancel := context.WithCancel(ctx)
				cancel()
			}
			err := tt.domain.UpdateRecord(ctx, tt.ipv6Getter, tt.tasker)
			if (err != nil) != tt.expectedErr {
				t.Errorf("Domain.UpdateRecord() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

// TestDomain_String 测试 Domain 结构体的 String 方法
func TestDomain_String(t *testing.T) {
	domain := &Domain{
		Domain:    "example.com",
		SubDomain: "sub",
		Type:      "AAAA",
		Addr:      net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
	}
	expected := "fullDomain: sub.example.com, type: AAAA, addr: 2001:db8:85a3::8a2e:370:7334"
	result := domain.String()
	if result != expected {
		t.Errorf("Domain.String() = %q, want %q", result, expected)
	}
}

// TestDomain_hasAddressChanged 测试 Domain 结构体的 hasAddressChanged 方法
func TestDomain_hasAddressChanged(t *testing.T) {
	domain := &Domain{
		Addr: net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7333"),
	}
	newAddr := net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
	changed := domain.hasAddressChanged(newAddr)
	if !changed {
		t.Errorf("Domain.hasAddressChanged() = %v, want true", changed)
	}

	sameAddr := net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7333")
	changed = domain.hasAddressChanged(sameAddr)
	if changed {
		t.Errorf("Domain.hasAddressChanged() = %v, want false", changed)
	}
}
