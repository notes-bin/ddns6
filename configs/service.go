package configs

import (
	_ "embed"
	"os"
	"strings"
	"text/template"
)

//go:embed systemd-service.tpl
var content string

type ServiceConfig struct {
	Description      string
	ExecStart        string
	WorkingDirectory string
}

func GenerateService(params ...string) {
	// 获取当前目录
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	paramsStr := strings.Join(params, " ")

	// 解析模板
	tmpl, err := template.New("service").Parse(content)
	if err != nil {
		panic(err)
	}

	// 定义服务配置
	config := ServiceConfig{
		Description:      "ddns6 auto update service",
		ExecStart:        paramsStr,
		WorkingDirectory: dir,
	}

	// 创建输出文件
	outputFile, err := os.Create("ddns6-update.service")
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()

	// 执行模板并写入文件
	err = tmpl.Execute(outputFile, config)
	if err != nil {
		panic(err)
	}
}
