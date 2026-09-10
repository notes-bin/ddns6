// Package retry 提供带指数退避的可重试错误封装。
//
// 用于 HTTP API 等临时性失败（网络波动、限流 429、5xx）的自动重试。
// 入口为 Do；仅 RetryableError（经 Retryable 包装）会触发退避重试，
// 其他错误立即返回。
//
// 使用示例：
//
//	err := retry.Do(ctx, 3, 100*time.Millisecond, func(ctx context.Context) error {
//	    resp, err := http.Get(url)
//	    if err != nil {
//	        return retry.Retryable(err)
//	    }
//	    defer resp.Body.Close()
//	    if resp.StatusCode == 429 || resp.StatusCode >= 500 {
//	        return retry.Retryable(fmt.Errorf("HTTP %d", resp.StatusCode))
//	    }
//	    return nil
//	})
package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// RetryableError 标记可重试的临时错误。
// Do 仅对此类型按退避策略重试；其他错误立即返回。
type RetryableError struct {
	Err error
}

// Error 实现 error，前缀 "retryable:"。
func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable: %v", e.Err)
}

// Unwrap 返回被包装的原始错误，供 errors.Is / errors.As 使用。
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// Retryable 将 err 包装为 RetryableError；err 为 nil 时返回 nil。
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// IsRetryable 判断 err（含包装链）是否为 RetryableError。
func IsRetryable(err error) bool {
	_, ok := errors.AsType[*RetryableError](err)
	return ok
}

// Do 执行 fn，遇 RetryableError 时按指数退避重试。
//
// 参数：
//   - ctx: 取消时中止重试并返回 ctx.Err()
//   - attempts: 最大尝试次数（含首次）
//   - baseDelay: 基础延迟；第 i 次重试等待约为 baseDelay*2^i 的全 jitter
//   - fn: 返回 RetryableError 时重试，其他错误或 nil 时停止
//
// 返回最后一次结果：成功为 nil，耗尽次数则为最后一次可重试错误的 Unwrap 值。
func Do(ctx context.Context, attempts int, baseDelay time.Duration, fn func(context.Context) error) error {
	var lastErr error

	for i := range attempts {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		re, ok := errors.AsType[*RetryableError](err)
		if !ok {
			return err
		}

		lastErr = re.Unwrap()

		if i == attempts-1 {
			break
		}

		// 指数退避 + 全 jitter，范围 [0, delay)
		delay := baseDelay * (1 << i) // baseDelay * 2^i
		if delay <= 0 {
			delay = 1 // 避免 rand.Int63n 对非正参数 panic
		}
		wait := time.Duration(rand.Int63n(int64(delay)))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return lastErr
}
