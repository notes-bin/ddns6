package ddns

import (
	"context"
	"uuid"
)

// syncIDKey 是 context 中存放 sync_id 的键类型。
type syncIDKey struct{}

// WithSyncID 为本轮同步注入 sync_id，便于并发 zone 日志对齐。
func WithSyncID(ctx context.Context) context.Context {
	return context.WithValue(ctx, syncIDKey{}, uuid.New().String())
}

// SyncIDFrom 取出 ctx 中的 sync_id；无则返回空串。
func SyncIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(syncIDKey{}).(string); ok {
		return id
	}
	return ""
}
