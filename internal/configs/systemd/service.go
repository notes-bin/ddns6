package systemd

import (
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed systemd-service.tpl
var serviceTemplate string

// SystemdConfig 定义了 systemd 服务的配置参数
type SystemdConfig struct {
	Description      string // 服务描述
	Environment      string // 环境变量
	ExecStart        string // 启动命令
	WorkingDirectory string // 工作目录
}

// GenerateService 生成 systemd 服务文件
func GenerateService(params ...string) {
	// 获取当前工作目录
	dir, err := os.Getwd()
	if err != nil {
		slog.Error("获取当前工作目录失败", "error", err)
		os.Exit(1)
	}

	// 解析模板
	tmpl, err := template.New("systemd-service").Parse(serviceTemplate)
	if err != nil {
		slog.Error("解析模板失败", "error", err)
		os.Exit(1)
	}

	// 定义服务配置
	config := SystemdConfig{
		Description:      "DDNS6 Auto Update Service", // 服务描述
		ExecStart:        strings.Join(params, " "),   // 启动命令
		WorkingDirectory: dir,                         // 工作目录
	}

	// 创建输出文件
	outputFilePath := filepath.Join(dir, "ddns6-update.service")
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		slog.Error("创建输出文件失败", "error", err, "path", outputFilePath)
		os.Exit(1)
	}
	defer outputFile.Close()

	// 执行模板并写入文件
	if err := tmpl.Execute(outputFile, config); err != nil {
		slog.Error("执行模板失败", "error", err)
		os.Exit(1)
	}

	slog.Info("systemd 服务文件生成成功", "path", outputFilePath)
}
