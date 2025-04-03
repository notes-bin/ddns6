package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockTask 是一个模拟的任务函数，用于测试
func MockTask(ctx context.Context) error {
	// 模拟任务执行
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// TestNew 测试 New 函数是否能正确创建调度器实例
func TestNew(t *testing.T) {
	scheduler := New()
	if scheduler == nil {
		t.Errorf("New() 返回了 nil，期望得到一个有效的调度器实例")
	}
}

// TestAddJob 测试 AddJob 方法是否能正确添加任务
func TestAddJob(t *testing.T) {
	scheduler := New()
	id := "test-job"
	interval := 1 * time.Second

	err := scheduler.AddJob(id, interval, MockTask)
	if err != nil {
		t.Errorf("AddJob() 返回错误: %v，期望成功添加任务", err)
	}

	// 检查任务是否已添加到调度器中
	_, loaded := scheduler.jobs.Load(id)
	if !loaded {
		t.Errorf("任务 %q 未成功添加到调度器中", id)
	}
}

// TestAddJobDuplicate 测试添加重复任务时是否返回错误
func TestAddJobDuplicate(t *testing.T) {
	scheduler := New()
	id := "test-job"
	interval := 1 * time.Second

	// 第一次添加任务
	err := scheduler.AddJob(id, interval, MockTask)
	if err != nil {
		t.Errorf("第一次调用 AddJob() 返回错误: %v，期望成功添加任务", err)
	}

	// 再次添加相同 ID 的任务
	err = scheduler.AddJob(id, interval, MockTask)
	if !errors.Is(err, ErrJobAlreadyExists) {
		t.Errorf("第二次调用 AddJob() 未返回预期的错误: %v，期望错误为 %v", err, ErrJobAlreadyExists)
	}
}

// TestUpdateJob 测试 UpdateJob 方法是否能正确更新任务
func TestUpdateJob(t *testing.T) {
	scheduler := New()
	id := "test-job"
	interval := 1 * time.Second
	newInterval := 2 * time.Second

	// 先添加任务
	err := scheduler.AddJob(id, interval, MockTask)
	if err != nil {
		t.Errorf("AddJob() 返回错误: %v，期望成功添加任务", err)
	}

	// 更新任务
	err = scheduler.UpdateJob(id, newInterval, MockTask)
	if err != nil {
		t.Errorf("UpdateJob() 返回错误: %v，期望成功更新任务", err)
	}

	// 检查任务的间隔是否已更新
	if job, ok := scheduler.jobs.Load(id); ok {
		if job.(*Job).interval != newInterval {
			t.Errorf("任务 %q 的间隔未更新为 %v，实际为 %v", id, newInterval, job.(*Job).interval)
		}
	} else {
		t.Errorf("任务 %q 未在调度器中找到，无法更新", id)
	}
}

// TestStopJob 测试 StopJob 方法是否能正确停止任务
func TestStopJob(t *testing.T) {
	scheduler := New()
	id := "test-job"
	interval := 1 * time.Second

	// 添加任务
	err := scheduler.AddJob(id, interval, MockTask)
	if err != nil {
		t.Errorf("AddJob() 返回错误: %v，期望成功添加任务", err)
	}

	// 停止任务
	err = scheduler.StopJob(id)
	if err != nil {
		t.Errorf("StopJob() 返回错误: %v，期望成功停止任务", err)
	}

	// 检查任务是否已从调度器中移除
	_, loaded := scheduler.jobs.Load(id)
	if loaded {
		t.Errorf("任务 %q 未成功从调度器中移除", id)
	}
}

// TestGracefulShutdown 测试 GracefulShutdown 方法是否能优雅关闭调度器
func TestGracefulShutdown(t *testing.T) {
	scheduler := New()
	id := "test-job"
	interval := 1 * time.Second

	// 添加任务
	err := scheduler.AddJob(id, interval, MockTask)
	if err != nil {
		t.Errorf("AddJob() 返回错误: %v，期望成功添加任务", err)
	}

	// 优雅关闭调度器
	scheduler.GracefulShutdown()

	// 检查调度器是否已停止
	if !scheduler.stopped.Load() {
		t.Errorf("GracefulShutdown() 后调度器未停止")
	}
}

// TestRunJobPanic 测试任务执行时发生 panic 的情况
func TestRunJobPanic(t *testing.T) {
	scheduler := New()
	id := "test-job"
	interval := 1 * time.Second

	// 定义一个会发生 panic 的任务
	panicTask := func(ctx context.Context) error {
		panic("模拟 panic")
	}

	// 添加任务
	err := scheduler.AddJob(id, interval, panicTask)
	if err != nil {
		t.Errorf("AddJob() 返回错误: %v，期望成功添加任务", err)
	}

	// 等待一段时间，让任务有机会执行
	time.Sleep(200 * time.Millisecond)

	// 检查错误通道是否收到了 panic 错误
	select {
	case err := <-scheduler.Errors():
		if !errors.Is(err, ErrJobPanic) {
			t.Errorf("错误通道收到的错误不是预期的 panic 错误: %v，期望错误为 %v", err, ErrJobPanic)
		}
	default:
		t.Errorf("错误通道未收到预期的 panic 错误")
	}
}

// 定义一些可能的错误类型，用于测试断言
var (
	ErrJobAlreadyExists = errors.New("job with id already exists")
	ErrJobPanic         = errors.New("job panic")
)
