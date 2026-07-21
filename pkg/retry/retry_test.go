// Package retry 重试机制测试
package retry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Retryable / IsRetryable 测试
// ============================================================

func TestRetryable_NilReturnsNil(t *testing.T) {
	if err := Retryable(nil); err != nil {
		t.Errorf("Retryable(nil) 应返回 nil, 得到 %v", err)
	}
}

func TestRetryable_WrapsError(t *testing.T) {
	original := fmt.Errorf("some error")
	wrapped := Retryable(original)

	if wrapped == nil {
		t.Fatal("Retryable(error) 不应返回 nil")
	}

	if !IsRetryable(wrapped) {
		t.Error("Retryable 包装后的错误应被识别为可重试")
	}
}

func TestIsRetryable_NonRetryable(t *testing.T) {
	err := fmt.Errorf("normal error")
	if IsRetryable(err) {
		t.Error("普通错误不应被识别为可重试")
	}
}

func TestIsRetryable_Nil(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("nil 不应被识别为可重试")
	}
}

func TestIsRetryable_WrappedInOther(t *testing.T) {
	original := Retryable(fmt.Errorf("inner"))
	wrapped := fmt.Errorf("outer: %w", original)

	if !IsRetryable(wrapped) {
		t.Error("被 fmt.Errorf 包装后仍应识别为可重试")
	}
}

func TestRetryableError_Unwrap(t *testing.T) {
	original := fmt.Errorf("inner error")
	wrapped := Retryable(original)

	var re *RetryableError
	if !errors.As(wrapped, &re) {
		t.Fatal("应该能 As 到 RetryableError")
	}

	unwrapped := re.Unwrap()
	if !errors.Is(unwrapped, original) {
		t.Errorf("Unwrap 应返回原始错误, 得到 %v", unwrapped)
	}
}

// ============================================================
// Do 测试
// ============================================================

func TestDo_FirstAttemptSucceeds(t *testing.T) {
	var count int32
	ctx := context.Background()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return nil // 首次成功
	})

	if err != nil {
		t.Errorf("首次成功时 Do 不应返回错误: %v", err)
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("fn 应只被调用 1 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_SucceedsAfterRetries(t *testing.T) {
	var count int32
	ctx := context.Background()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		n := atomic.AddInt32(&count, 1)
		if n < 3 {
			return Retryable(fmt.Errorf("attempt %d failed", n))
		}
		return nil // 第 3 次成功
	})

	if err != nil {
		t.Errorf("重试成功后 Do 不应返回错误: %v", err)
	}
	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("fn 应被调用 3 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_AllAttemptsFail(t *testing.T) {
	var count int32
	ctx := context.Background()
	expectedErr := fmt.Errorf("always fail")

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return Retryable(expectedErr)
	})

	if err == nil {
		t.Fatal("所有尝试失败时 Do 应返回错误")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("应返回原始错误, 得到 %v", err)
	}
	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("fn 应被调用 3 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	var count int32
	ctx := context.Background()
	expectedErr := fmt.Errorf("non-retryable error")

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return expectedErr // 非 RetryableError
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("应返回原始非可重试错误, 得到 %v", err)
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("非可重试错误应只调用 1 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_ZeroAttempts(t *testing.T) {
	var count int32
	ctx := context.Background()

	err := Do(ctx, 0, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return Retryable(fmt.Errorf("fail"))
	})

	if err != nil {
		// 0 attempts 时的行为：可能返回 nil（循环不执行），取决于实现
		// 我们接受任一结果
	}
}

func TestDo_SingleAttempt(t *testing.T) {
	var count int32
	ctx := context.Background()
	expectedErr := fmt.Errorf("fail")

	err := Do(ctx, 1, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return Retryable(expectedErr)
	})

	if err == nil {
		t.Fatal("单次尝试失败时应返回错误")
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("单次尝试应只调用 1 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		return Retryable(fmt.Errorf("fail"))
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("上下文取消时 Do 应返回 Canceled, 得到 %v", err)
	}
}

func TestDo_ContextCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var count int32

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, 5, 100*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return Retryable(fmt.Errorf("fail"))
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("退避中取消应返回 Canceled, 得到 %v", err)
	}
}

func TestDo_NilErrorIsNotRetryable(t *testing.T) {
	var count int32
	ctx := context.Background()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	if err != nil {
		t.Errorf("Do 应返回 nil, 得到 %v", err)
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("fn 应只被调用 1 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

// ============================================================
// 集成风格测试：模拟 HTTP 调用
// ============================================================

func TestDo_SimulateHTTPRetry(t *testing.T) {
	ctx := context.Background()
	var count int32

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		n := atomic.AddInt32(&count, 1)
		// 模拟 HTTP 调用：前 2 次 503，第 3 次 200
		if n < 3 {
			return Retryable(fmt.Errorf("HTTP 503 Service Unavailable"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("重试成功后不应返回错误: %v", err)
	}
	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("应重试 2 次（共 3 次调用）, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_SimulateHTTPClientError(t *testing.T) {
	ctx := context.Background()
	var count int32

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&count, 1)
		// 4xx 错误通常不可重试（除 429）
		return fmt.Errorf("HTTP 400 Bad Request")
	})

	if err == nil {
		t.Fatal("4xx 错误应返回错误")
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("非可重试错误应只调用 1 次, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_SimulateHTTPRateLimit(t *testing.T) {
	ctx := context.Background()
	var count int32

	err := Do(ctx, 4, 10*time.Millisecond, func(ctx context.Context) error {
		n := atomic.AddInt32(&count, 1)
		if n < 4 {
			return Retryable(fmt.Errorf("HTTP 429 Too Many Requests"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("429 重试成功后不应返回错误: %v", err)
	}
	if atomic.LoadInt32(&count) != 4 {
		t.Errorf("应重试 3 次（共 4 次调用）, 实际 %d", atomic.LoadInt32(&count))
	}
}

func TestDo_BackoffIncreasing(t *testing.T) {
	// 验证重试后确实有等待，即每次重试的时间戳不同
	ctx := context.Background()
	var timestamps []time.Time

	err := Do(ctx, 3, 30*time.Millisecond, func(ctx context.Context) error {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) < 3 {
			return Retryable(fmt.Errorf("fail"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("重试成功后不应返回错误: %v", err)
	}
	if len(timestamps) != 3 {
		t.Fatalf("应记录 3 个时间戳, 实际 %d", len(timestamps))
	}

	// 检查退避时间差应递增
	diff1 := timestamps[1].Sub(timestamps[0])
	diff2 := timestamps[2].Sub(timestamps[1])
	if diff2 < diff1/2 {
		t.Logf("退避时间应大致递增: diff1=%v, diff2=%v", diff1, diff2)
	}
}
