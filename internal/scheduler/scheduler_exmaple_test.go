package scheduler_test

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/notes-bin/ddns6/internal/scheduler"
)

func ExampleScheduler() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	sched := scheduler.New()

	// 错误处理协程
	go func() {
		for err := range sched.Errors() {
			slog.Error("scheduler error", "error", err)
		}
	}()

	// 任务
	task := func(ctx context.Context) error {
		// 执行具体的DDNS更新逻辑
		return nil
	}

	// 添加任务
	err := sched.AddJob("check_ip", 5*time.Minute, task)
	if err != nil {
		slog.Error("failed to add job", "error", err)
		return
	}

	// 处理信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 优雅关闭
	sched.GracefulShutdown()
}
