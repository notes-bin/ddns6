package main

// import (
// 	"context"
// 	"os"
// 	"os/signal"
// 	"syscall"

// "github.com/notes-bin/ddns6/internal/domain"
// "github.com/notes-bin/ddns6/internal/providers/cloudflare"
// "github.com/notes-bin/ddns6/internal/providers/tencent"
// "github.com/notes-bin/ddns6/utils/cli"
// "github.com/notes-bin/ddns6/utils/env"
// "github.com/notes-bin/ddns6/utils/logging"

// )

// func main() {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	// 加载配置
// 	cfg, err := config.Load("config.yaml")
// 	if err != nil {
// 		logger.Fatal("Failed to load config", "error", err)
// 	}

// 	// 初始化提供商
// 	provider := providers.NewManager().GetProvider(cfg.Provider)
// 	if provider == nil {
// 		logger.Fatal("Provider not found", "name", cfg.Provider)
// 	}

// 	// 初始化IPv6获取器
// 	ipGetter := network.NewMultiSourceGetter(ctx)

// 	// 创建调度器
// 	sched := scheduler.New()

// 	// 添加任务
// 	for _, domainCfg := range cfg.Domains {
// 		task := domain.NewTask(domainCfg, provider, ipGetter)
// 		sched.AddJob(domainCfg.Name, domainCfg.Interval, task.Run)
// 	}

// 	// 信号处理
// 	sigCh := make(chan os.Signal, 1)
// 	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

// 	// 等待退出
// 	<-sigCh
// 	logger.Info("Received shutdown signal")
// 	sched.GracefulShutdown()
// }
