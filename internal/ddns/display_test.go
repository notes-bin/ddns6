package ddns

import (
	"strings"
	"testing"
)

// TestFormatRecords_Empty 验证空切片返回固定提示文案。
func TestFormatRecords_Empty(t *testing.T) {
	result := FormatRecords([]RecordInfo{})
	if result != "No records found." {
		t.Errorf("空记录集应返回 'No records found.', 得到 %q", result)
	}
}

// TestFormatRecords_Nil 验证 nil 切片与空切片行为一致。
func TestFormatRecords_Nil(t *testing.T) {
	result := FormatRecords(nil)
	if result != "No records found." {
		t.Errorf("nil 应返回 'No records found.', 得到 %q", result)
	}
}

// TestFormatRecords_SingleRecord 验证单条记录输出含表头与字段。
func TestFormatRecords_SingleRecord(t *testing.T) {
	records := []RecordInfo{
		{ID: "12345678", Name: "www.example.com", Zone: "example.com", Type: "AAAA", Value: "240e:1234::1", TTL: 600},
	}
	result := FormatRecords(records)

	if !strings.Contains(result, "ID") || !strings.Contains(result, "Name") ||
		!strings.Contains(result, "Type") || !strings.Contains(result, "Value") ||
		!strings.Contains(result, "TTL") {
		t.Error("结果应包含表头 (ID, Name, Type, Value, TTL)")
	}

	if !strings.Contains(result, "12345678") {
		t.Error("结果应包含记录 ID")
	}
	if !strings.Contains(result, "www.example.com") {
		t.Error("结果应包含域名")
	}
	if !strings.Contains(result, "240e:1234::1") {
		t.Error("结果应包含 IPv6 地址")
	}
	if !strings.Contains(result, "600") {
		t.Error("结果应包含 TTL")
	}
}

// TestFormatRecords_MultipleRecords 验证多条记录均出现在输出中。
func TestFormatRecords_MultipleRecords(t *testing.T) {
	records := []RecordInfo{
		{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "::1", TTL: 600},
		{ID: "2", Name: "api.example.com", Type: "AAAA", Value: "::2", TTL: 300},
	}
	result := FormatRecords(records)

	if !strings.Contains(result, "www.example.com") {
		t.Error("结果应包含第一条记录域名")
	}
	if !strings.Contains(result, "api.example.com") {
		t.Error("结果应包含第二条记录域名")
	}
}

// TestFormatRecords_ContainsRecordType 验证输出包含记录类型字段。
func TestFormatRecords_ContainsRecordType(t *testing.T) {
	records := []RecordInfo{
		{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "::1", TTL: 600},
	}
	result := FormatRecords(records)
	if !strings.Contains(result, "AAAA") {
		t.Error("结果应包含记录类型 AAAA")
	}
}

// TestFormatRecords_EmptyID 验证 ID 为空时仍正常渲染有记录输出。
func TestFormatRecords_EmptyID(t *testing.T) {
	records := []RecordInfo{
		{ID: "", Name: "www.example.com", Type: "AAAA", Value: "::1", TTL: 600},
	}
	result := FormatRecords(records)
	if result == "No records found." {
		t.Error("即使 ID 为空，有记录时也不应返回 'No records found.'")
	}
	if !strings.Contains(result, "www.example.com") {
		t.Error("结果应包含域名")
	}
}

// TestFormatRecords_DifferentTypes 验证混合记录类型均可输出。
func TestFormatRecords_DifferentTypes(t *testing.T) {
	records := []RecordInfo{
		{ID: "1", Name: "www.example.com", Type: "AAAA", Value: "::1", TTL: 600},
		{ID: "2", Name: "example.com", Type: "MX", Value: "mail.example.com", TTL: 300},
	}
	result := FormatRecords(records)
	if !strings.Contains(result, "AAAA") {
		t.Error("结果应包含 AAAA 类型")
	}
	if !strings.Contains(result, "MX") {
		t.Error("结果应包含 MX 类型")
	}
}
