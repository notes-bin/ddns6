package configs

import (
	_ "embed"
	"log/slog"
	"os"
	"strings"
	"text/template"
)

//go:embed systemd-service.tpl
var content string

type ServiceConfig struct {
	Description      string
	Environment      string
	ExecStart        string
	WorkingDirectory string
}

func GenerateService(params ...string) {
	// 获取当前目录
	dir, err := os.Getwd()
	if err != nil {
		slog.Error("获取当前目录失败", "err", err)
		os.Exit(1)
	}

	// 解析模板
	tmpl, err := template.New("service").Parse(content)
	if err != nil {
		slog.Error("解析模板失败", "err", err)
		os.Exit(1)
	}

	// 定义服务配置
	config := ServiceConfig{
		Description:      "ddns6 auto update service",
		ExecStart:        strings.Join(params, " "),
		WorkingDirectory: dir,
	}

	// 创建输出文件
	outputFile, err := os.Create("ddns6-update.service")
	if err != nil {
		slog.Error("创建输出文件失败", "err", err)
		os.Exit(1)
	}
	defer outputFile.Close()

	// 执行模板并写入文件
	err = tmpl.Execute(outputFile, config)
	if err != nil {
		slog.Error("执行模板失败", "err", err)
		os.Exit(1)
	}
}
