// Package retry 提供带指数退避的重试机制。
//
// 使用场景：HTTP API 调用遇到临时性错误（网络波动、服务端限流 429、5xx 等）
// 时自动重试，提升服务可靠性。
//
// 使用示例：
//
//	body, err := retry.Do(ctx, 3, 100*time.Millisecond, func(ctx context.Context) error {
//	    resp, err := http.Get(url)
//	    if err != nil { return retry.Retryable(err) }
//	    if resp.StatusCode == 429 { return retry.Retryable(fmt.Errorf("rate limited")) }
//	    if resp.StatusCode >= 500 { return retry.Retryable(fmt.Errorf("server error: %d", resp.StatusCode)) }
//	    return nil  // 成功，不重试
//	})
package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// RetryableError 标记一个错误是可重试的临时错误。
// Do 遇到 RetryableError 时按退避策略重试，遇到其他错误则立即返回。
type RetryableError struct {
	Err error
}

// Error 返回错误信息。
func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable: %v", e.Err)
}

// Unwrap 返回被包装的原始错误。
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// Retryable 将 err 包装为可重试错误。
// 如果 err 本身为 nil 则返回 nil。
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &RetryableError{Err: err}
}

// IsRetryable 判断错误是否为 RetryableError。
func IsRetryable(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}

// Do 执行 fn，遇到 RetryableError 时按指数退避重试。
//
// 参数：
//   - ctx: 上下文，取消时中止重试
//   - attempts: 最大尝试次数（包括首次调用）
//   - baseDelay: 基础延迟，每次重试的延迟为 baseDelay * 2^n + jitter
//   - fn: 要执行的函数，返回 RetryableError 时重试，其他错误或 nil 时停止
//
// 返回 fn 的最后一次返回值（成功时为 nil，超过重试次数时返回最后一次错误）。
func Do(ctx context.Context, attempts int, baseDelay time.Duration, fn func(context.Context) error) error {
	var lastErr error

	for i := range attempts {
		// 检查上下文是否已取消
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(ctx)
		if err == nil {
			return nil // 成功
		}

		// 非 RetryableError — 立即返回
		var re *RetryableError
		if !errors.As(err, &re) {
			return err
		}

		lastErr = re.Unwrap()

		// 最后一次尝试后不再等待
		if i == attempts-1 {
			break
		}

		// 指数退避 + 随机 jitter
		delay := baseDelay * (1 << i) // baseDelay * 2^i
		halfDelay := delay / 2
		// jitter 在 [0, halfDelay] 范围内
		jitter := time.Duration(rand.Int63n(int64(halfDelay + 1)))
		wait := halfDelay + jitter // halfDelay ~ 1.5*halfDelay

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return lastErr
}
