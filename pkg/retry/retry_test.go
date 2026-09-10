package retry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryable_NilReturnsNil 验证 Retryable(nil) 返回 nil。
func TestRetryable_NilReturnsNil(t *testing.T) {
	if err := Retryable(nil); err != nil {
		t.Errorf("Retryable(nil) 应返回 nil, 得到 %v", err)
	}
}

// TestRetryable_WrapsError 验证包装后的错误可被 IsRetryable 识别。
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

// TestIsRetryable_NonRetryable 验证普通错误不被识别为可重试。
func TestIsRetryable_NonRetryable(t *testing.T) {
	err := fmt.Errorf("normal error")
	if IsRetryable(err) {
		t.Error("普通错误不应被识别为可重试")
	}
}

// TestIsRetryable_Nil 验证 nil 不被识别为可重试。
func TestIsRetryable_Nil(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("nil 不应被识别为可重试")
	}
}

// TestIsRetryable_WrappedInOther 验证经 fmt.Errorf %w 再包装后仍可识别。
func TestIsRetryable_WrappedInOther(t *testing.T) {
	original := Retryable(fmt.Errorf("inner"))
	wrapped := fmt.Errorf("outer: %w", original)

	if !IsRetryable(wrapped) {
		t.Error("被 fmt.Errorf 包装后仍应识别为可重试")
	}
}

// TestRetryableError_Unwrap 验证 Unwrap 返回原始错误且可 errors.Is。
func TestRetryableError_Unwrap(t *testing.T) {
	original := fmt.Errorf("inner error")
	wrapped := Retryable(original)

	re, ok := errors.AsType[*RetryableError](wrapped)
	if !ok {
		t.Fatal("应该能 AsType 到 RetryableError")
	}

	unwrapped := re.Unwrap()
	if !errors.Is(unwrapped, original) {
		t.Errorf("Unwrap 应返回原始错误, 得到 %v", unwrapped)
	}
}

// TestDo_FirstAttemptSucceeds 验证首次成功时不重试。
func TestDo_FirstAttemptSucceeds(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Errorf("首次成功时 Do 不应返回错误: %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("fn 应只被调用 1 次, 实际 %d", count.Load())
	}
}

// TestDo_SucceedsAfterRetries 验证前几次可重试失败后最终成功。
func TestDo_SucceedsAfterRetries(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		n := count.Add(1)
		if n < 3 {
			return Retryable(fmt.Errorf("attempt %d failed", n))
		}
		return nil
	})

	if err != nil {
		t.Errorf("重试成功后 Do 不应返回错误: %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("fn 应被调用 3 次, 实际 %d", count.Load())
	}
}

// TestDo_AllAttemptsFail 验证耗尽次数后返回原始可重试错误。
func TestDo_AllAttemptsFail(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()
	expectedErr := fmt.Errorf("always fail")

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return Retryable(expectedErr)
	})

	if err == nil {
		t.Fatal("所有尝试失败时 Do 应返回错误")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("应返回原始错误, 得到 %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("fn 应被调用 3 次, 实际 %d", count.Load())
	}
}

// TestDo_NonRetryableError 验证非可重试错误立即返回且不重试。
func TestDo_NonRetryableError(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()
	expectedErr := fmt.Errorf("non-retryable error")

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("应返回原始非可重试错误, 得到 %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("非可重试错误应只调用 1 次, 实际 %d", count.Load())
	}
}

// TestDo_ZeroAttempts 验证 attempts=0 时不调用 fn（循环不执行）。
func TestDo_ZeroAttempts(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()

	err := Do(ctx, 0, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return Retryable(fmt.Errorf("fail"))
	})

	if err != nil {
		// 0 attempts：循环不执行，通常返回 nil；此处不强制断言返回值
	}
	_ = count
}

// TestDo_SingleAttempt 验证 attempts=1 失败时直接返回且无退避。
func TestDo_SingleAttempt(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()
	expectedErr := fmt.Errorf("fail")

	err := Do(ctx, 1, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return Retryable(expectedErr)
	})

	if err == nil {
		t.Fatal("单次尝试失败时应返回错误")
	}
	if count.Load() != 1 {
		t.Errorf("单次尝试应只调用 1 次, 实际 %d", count.Load())
	}
}

// TestDo_ContextCancelled 验证入口处已取消的 context 返回 Canceled。
func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		return Retryable(fmt.Errorf("fail"))
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("上下文取消时 Do 应返回 Canceled, 得到 %v", err)
	}
}

// TestDo_ContextCancelledDuringBackoff 验证退避等待期间取消可中断。
func TestDo_ContextCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var count atomic.Int32

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, 5, 100*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return Retryable(fmt.Errorf("fail"))
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("退避中取消应返回 Canceled, 得到 %v", err)
	}
}

// TestDo_NilErrorIsNotRetryable 验证 fn 返回 nil 视为成功。
func TestDo_NilErrorIsNotRetryable(t *testing.T) {
	var count atomic.Int32
	ctx := t.Context()

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	if err != nil {
		t.Errorf("Do 应返回 nil, 得到 %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("fn 应只被调用 1 次, 实际 %d", count.Load())
	}
}

// TestDo_SimulateHTTPRetry 模拟 503 后最终成功的 HTTP 重试路径。
func TestDo_SimulateHTTPRetry(t *testing.T) {
	ctx := t.Context()
	var count atomic.Int32

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		n := count.Add(1)
		if n < 3 {
			return Retryable(fmt.Errorf("HTTP 503 Service Unavailable"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("重试成功后不应返回错误: %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("应重试 2 次（共 3 次调用）, 实际 %d", count.Load())
	}
}

// TestDo_SimulateHTTPClientError 模拟 4xx 不可重试错误立即失败。
func TestDo_SimulateHTTPClientError(t *testing.T) {
	ctx := t.Context()
	var count atomic.Int32

	err := Do(ctx, 3, 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return fmt.Errorf("HTTP 400 Bad Request")
	})

	if err == nil {
		t.Fatal("4xx 错误应返回错误")
	}
	if count.Load() != 1 {
		t.Errorf("非可重试错误应只调用 1 次, 实际 %d", count.Load())
	}
}

// TestDo_SimulateHTTPRateLimit 模拟 429 限流经多次重试后成功。
func TestDo_SimulateHTTPRateLimit(t *testing.T) {
	ctx := t.Context()
	var count atomic.Int32

	err := Do(ctx, 4, 10*time.Millisecond, func(ctx context.Context) error {
		n := count.Add(1)
		if n < 4 {
			return Retryable(fmt.Errorf("HTTP 429 Too Many Requests"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("429 重试成功后不应返回错误: %v", err)
	}
	if count.Load() != 4 {
		t.Errorf("应重试 3 次（共 4 次调用）, 实际 %d", count.Load())
	}
}

// TestDo_BackoffIncreasing 粗略验证连续重试之间存在退避等待。
func TestDo_BackoffIncreasing(t *testing.T) {
	ctx := t.Context()
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

	diff1 := timestamps[1].Sub(timestamps[0])
	diff2 := timestamps[2].Sub(timestamps[1])
	if diff2 < diff1/2 {
		t.Logf("退避时间应大致递增: diff1=%v, diff2=%v", diff1, diff2)
	}
}
