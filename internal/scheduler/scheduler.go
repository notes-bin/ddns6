package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// TaskFunc 定义任务函数类型，支持上下文和错误返回
type TaskFunc func(ctx context.Context) error

// JobState 表示任务状态
type JobState int32

const (
	StateIdle JobState = iota
	StateRunning
	StateStopping
	StateStopped
)

// Job 表示一个调度任务
type Job struct {
	ID       string
	interval time.Duration
	task     TaskFunc
	ctx      context.Context
	cancel   context.CancelFunc
	state    atomic.Int32 // 使用原子操作保证状态一致性
}

// Scheduler 任务调度器
type Scheduler struct {
	jobs    sync.Map
	wg      sync.WaitGroup
	logger  *slog.Logger
	errCh   chan error
	stopCh  chan struct{}
	stopped atomic.Bool
}

// New 创建调度器实例
func New(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
		errCh:  make(chan error, 100),
		stopCh: make(chan struct{}),
	}
}

// Errors 返回错误通道
func (s *Scheduler) Errors() <-chan error {
	return s.errCh
}

// AddJob 添加新任务
func (s *Scheduler) AddJob(id string, interval time.Duration, task TaskFunc) error {
	if s.stopped.Load() {
		return fmt.Errorf("scheduler is stopped")
	}

	if _, loaded := s.jobs.LoadOrStore(id, nil); loaded {
		return fmt.Errorf("job with id %q already exists", id)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID:       id,
		interval: interval,
		task:     task,
		ctx:      ctx,
		cancel:   cancel,
	}
	job.state.Store(int32(StateIdle))

	s.jobs.Store(id, job)
	s.wg.Add(1)

	go s.runJob(job)
	return nil
}

// UpdateJob 更新现有任务
func (s *Scheduler) UpdateJob(id string, newInterval time.Duration, newTask TaskFunc) error {
	if s.stopped.Load() {
		return fmt.Errorf("scheduler is stopped")
	}

	value, ok := s.jobs.Load(id)
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	job := value.(*Job)
	if !job.state.CompareAndSwap(int32(StateIdle), int32(StateIdle)) {
		return fmt.Errorf("job %q is not idle", id)
	}

	// 停止当前任务
	job.cancel()

	// 创建新上下文
	ctx, cancel := context.WithCancel(context.Background())
	job.ctx = ctx
	job.cancel = cancel
	job.interval = newInterval
	if newTask != nil {
		job.task = newTask
	}

	// 重启任务
	s.wg.Add(1)
	go s.runJob(job)
	return nil
}

// runJob 实际执行任务
func (s *Scheduler) runJob(job *Job) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("job panic recovered",
				"job_id", job.ID,
				"panic", r,
			)
			s.sendError(fmt.Errorf("job %q panic: %v", job.ID, r))
		}
	}()

	ticker := time.NewTicker(job.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !job.state.CompareAndSwap(int32(StateIdle), int32(StateRunning)) {
				s.logger.Warn("job skipped: not idle",
					"job_id", job.ID,
					"state", JobState(job.state.Load()))
				continue
			}

			s.logger.Debug("job starting", "job_id", job.ID)
			err := job.task(job.ctx)
			if err != nil {
				s.sendError(fmt.Errorf("job %q failed: %w", job.ID, err))
			}

			if !job.state.CompareAndSwap(int32(StateRunning), int32(StateIdle)) {
				s.logger.Warn("job state inconsistency",
					"job_id", job.ID,
					"state", JobState(job.state.Load()))
			}
			s.logger.Debug("job completed", "job_id", job.ID)

		case <-job.ctx.Done():
			job.state.Store(int32(StateStopped))
			s.logger.Info("job stopped", "job_id", job.ID)
			return
		case <-s.stopCh:
			job.state.Store(int32(StateStopping))
			job.cancel()
			return
		}
	}
}

// sendError 发送错误到错误通道
func (s *Scheduler) sendError(err error) {
	select {
	case s.errCh <- err:
	default:
		s.logger.Error("error channel full, dropping error",
			"error", err)
	}
}

// StopJob 停止指定任务
func (s *Scheduler) StopJob(id string) error {
	if s.stopped.Load() {
		return fmt.Errorf("scheduler is stopped")
	}

	value, ok := s.jobs.Load(id)
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	job := value.(*Job)
	if !job.state.CompareAndSwap(int32(StateIdle), int32(StateStopping)) {
		return fmt.Errorf("job %q is not idle", id)
	}

	job.cancel()
	s.jobs.Delete(id)
	return nil
}

// GracefulShutdown 优雅停止所有任务
func (s *Scheduler) GracefulShutdown() {
	if !s.stopped.CompareAndSwap(false, true) {
		return
	}

	close(s.stopCh)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("all jobs stopped")
	case <-time.After(30 * time.Second):
		s.logger.Warn("graceful shutdown timed out")
	}

	close(s.errCh)
}
