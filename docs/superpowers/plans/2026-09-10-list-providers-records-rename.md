# list/records 命令语义重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将「列 DNS 记录」命令从 `list` 重命名为 `records`，并新增 `list` 用于列出全部可用 DNS 运营商。

**Architecture:** 原 `cmd/list.go` 中的记录查询逻辑整体迁入 `cmd/records.go`（符号统一改名）；`cmd/list.go` 改为只读遍历 `providerFactories` 打印表格。注册、测试、README、相关注释与错误文案同步更新；不保留旧 `list` 查记录别名。

**Tech Stack:** Go、cobra、现有 `providerFactories` / `restrictedProviders`

## Global Constraints

- 硬切换：无 `list`→`records` 兼容别名或弃用警告
- `list` 不读配置、不访问网络；输出列：`NAME` / `RECORDS/CLEAN` / `DESCRIPTION`，顺序与 `providerFactories` 一致
- `RECORDS/CLEAN`：`noListClean == true` 为 `no`，否则 `yes`
- `DESCRIPTION` 使用工厂 `short`，整行输出不截断
- `list` / `version` / `init` 同类轻量命令：跳过 slog 文件初始化
- 注释中文；错误/日志英文；`gofmt` / `go vet` / `go test ./...` 通过
- 不改 `clean` 命令名；不引入 JSON 输出 flag

---

## File Structure

| 文件 | 职责 |
|------|------|
| `cmd/records.go` | Create：原 list 的 records 命令、`handleRecords`、`runRecordsWithConfig`、辅助函数 |
| `cmd/list.go` | Rewrite：运营商列表命令与 `formatProviderList` |
| `cmd/root.go` | 注册 `recordsCmd`/`listCmd`；帮助文案；PersistentPreRun 跳过 list 日志 |
| `cmd/providers.go` | 注释 list→records；错误字符串随 `commandName` 参数已动态，确认调用方传 `"records"` |
| `cmd/*_test.go` | 测试路径与断言更新；新增 list 运营商测试 |
| `README.md` | 命令说明与示例；表头 list/clean → records/clean |
| `internal/config/config.go`、`internal/ddns/*.go` | 包/函数注释中的 list 列记录语义 → records |

---

### Task 1: 将列 DNS 记录命令迁移为 `records`

**Files:**
- Create: `cmd/records.go`
- Modify: `cmd/root.go`（注册与注释）
- Modify: `cmd/list.go`（本任务结束前可暂时保留旧命令，Task 2 覆盖）
- Modify: `cmd/cli_test.go`、`cmd/commands_test.go`、`cmd/handlers_test.go`
- Test: `cmd/*_test.go`

**Interfaces:**
- Produces: `var recordsCmd *cobra.Command`；`registerRecordsCommands()`；`handleRecords(cmd *cobra.Command, domains []*ddns.Domain, p ddns.DNSProvider) error`；`runRecordsWithConfig(cmd *cobra.Command) error`
- Consumes: `registerProviderSubCommands`、`runWithConfig`、`restrictedProviders`、`createDomainConfigs`（既有）

- [ ] **Step 1: 新增 `cmd/records.go`（由现 `list.go` 复制并改名）**

将下列符号替换：

| 旧 | 新 |
|----|----|
| `listCmd` | `recordsCmd` |
| `Use: "list [provider]"` | `Use: "records [provider]"` |
| `Short: "列出 DNS 记录"` | 保持 |
| Long/示例中的 `ddns6 list` | `ddns6 records` |
| `registerListCommands` | `registerRecordsCommands` |
| `handleList` | `handleRecords` |
| `runListWithConfig` | `runRecordsWithConfig` |
| `runWithConfig(..., "list", ...)` | `runWithConfig(..., "records", ...)` |
| `does not support 'list'` | `does not support 'records'` |
| `registerProviderSubCommands(listCmd, "list", ...)` | `registerProviderSubCommands(recordsCmd, "records", ...)` |

保留 `recordTypeDesc`、`buildFilterInfo`（仅一份，放在 `records.go`）。错误 `failed to list records` 可保留（英文「list records」指动作，非命令名）。

- [ ] **Step 2: 更新 `cmd/root.go` 注册**

```go
rootCmd.AddCommand(recordsCmd)
rootCmd.AddCommand(listCmd) // list 仍指向旧实现直到 Task 2；若同名冲突则先只注册 recordsCmd
registerRecordsCommands()
```

包注释命令树：

```
├── list                 列出可用 DNS 运营商
├── records [provider]   列出 DNS 记录
└── clean   [provider]   删除 DNS 记录
```

若 Task 1 与 Task 2 同会话执行：可在 Task 1 末尾删除旧 `listCmd` 定义，避免两个 `list`；推荐 **本任务先只加 records 并改测试指向 records，下一任务再重写 list.go**。

临时策略（避免双 list）：在 Task 1 将 `list.go` 内原命令改名为 records（文件重命名为 `records.go`），`list` 空缺到 Task 2。即：

```bash
# 概念步骤：mv list.go → records.go 后批量改名；Task 2 新建 list.go
```

- [ ] **Step 3: 更新测试中的命令路径**

`cmd/cli_test.go`：

```go
{"records", "cloudflare"},
{"records", "duckdns"},
// ...
duck := findCommand(findCommand(rootCmd, "records"), "duckdns")
```

`cmd/commands_test.go`：所有 `"list"` 查记录路径改为 `"records"`；`runListWithConfig` → `runRecordsWithConfig`。

`cmd/handlers_test.go`：`handleList` → `handleRecords`；`requireErrContains(..., "does not support 'records'")`。

- [ ] **Step 4: 运行测试**

Run: `go test ./cmd/ -count=1`

Expected: PASS（此时尚无新 `list` 运营商命令亦可；若 root 仍注册旧 list 名需与实现一致）

- [ ] **Step 5: Commit**

```bash
git add cmd/records.go cmd/list.go cmd/root.go cmd/cli_test.go cmd/commands_test.go cmd/handlers_test.go
git commit -m "$(cat <<'EOF'
refactor(cmd): 将列 DNS 记录命令由 list 重命名为 records

硬切换语义，为 list 专用于运营商列表腾出名额。
EOF
)"
```

---

### Task 2: 实现 `ddns6 list` 运营商列表

**Files:**
- Create/Rewrite: `cmd/list.go`
- Modify: `cmd/root.go`（注册、`PersistentPreRun` 跳过 list 日志、`Long` 引导）
- Test: `cmd/list_test.go`（新建）或写入既有 `commands_test.go`

**Interfaces:**
- Consumes: `providerFactories []providerFactory`（`name`、`short`、`noListClean`）
- Produces: `var listCmd *cobra.Command`；`formatProviderList(factories []providerFactory) string`（便于单测）

- [ ] **Step 1: 写失败测试**

在 `cmd/list_test.go`：

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestFormatProviderList 验证运营商表格含全部工厂名且受限为 no。
func TestFormatProviderList(t *testing.T) {
	out := formatProviderList(providerFactories)
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "RECORDS/CLEAN") {
		t.Fatalf("missing header: %s", out)
	}
	for _, p := range providerFactories {
		if !strings.Contains(out, p.name) {
			t.Errorf("missing provider %q", p.name)
		}
	}
	if !strings.Contains(out, "duckdns") || !strings.Contains(out, "no") {
		t.Error("expected duckdns marked no for RECORDS/CLEAN")
	}
	if !strings.Contains(out, fmt.Sprintf("Total: %d providers", len(providerFactories))) {
		t.Errorf("missing total line: %s", out)
	}
}

// TestListCmd_Execute 验证 ddns6 list 成功且输出含 tencent。
func TestListCmd_Execute(t *testing.T) {
	initRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"list"})
	// 若命令打印走 fmt.Println 而非 cmd.OutOrStdout，改为捕获 os.Stdout 或只测 formatProviderList
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
}
```

若项目测试习惯用 `withArgs`，对齐 `commands_test.go` 模式；**核心断言必须覆盖 `formatProviderList`**。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./cmd/ -run TestFormatProviderList -count=1`

Expected: FAIL（`formatProviderList` undefined）

- [ ] **Step 3: 实现 `cmd/list.go`**

```go
package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// listCmd 列出项目内全部可用 DNS 运营商（不读配置、不访问网络）。
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出可用的 DNS 运营商",
	Long: `列出 ddns6 当前支持的全部 DNS 运营商。

输出包含 CLI 名称、是否支持 records/clean，以及简要说明。
数据来自内置注册表，无需配置文件或网络。

示例:
  ddns6 list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(formatProviderList(providerFactories))
		return nil
	},
}

// formatProviderList 按注册顺序格式化运营商表格。
func formatProviderList(factories []providerFactory) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tRECORDS/CLEAN\tDESCRIPTION")
	for _, p := range factories {
		rc := "yes"
		if p.noListClean {
			rc = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.name, rc, p.short)
	}
	_ = w.Flush()
	fmt.Fprintf(&b, "\nTotal: %d providers\n", len(factories))
	return b.String()
}
```

注意：`noListClean` 须与 `restrictedProviders` 一致（duckdns/he/noip 工厂应已设 `noListClean: true`）。实现前核对三家工厂字段；若仅 map 有而字段未设，则以 `restrictedProviders[p.name]` 为准：

```go
rc := "yes"
if p.noListClean || restrictedProviders[p.name] {
	rc = "no"
}
```

- [ ] **Step 4: 更新 `root.go` PersistentPreRun**

```go
if cmd.Name() == "version" || cmd.Name() == "init" || cmd.Name() == "list" {
	return
}
```

`rootCmd.Long`：将硬编码 23 家名单改为引导，例如：

```
支持的 DNS 服务商: 运行 ddns6 list 查看完整列表与能力说明。
```

确保 `rootCmd.AddCommand(listCmd)` 与 `recordsCmd` 均已注册；**删除**旧的 `registerListCommands` 调用（已改为 `registerRecordsCommands`）。

- [ ] **Step 5: 运行测试**

Run: `go test ./cmd/ -count=1 && go vet ./cmd/...`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/list.go cmd/list_test.go cmd/root.go
git commit -m "$(cat <<'EOF'
feat(cmd): 新增 list 命令列出全部 DNS 运营商

以表格展示名称、records/clean 能力与说明，数据源自 providerFactories。
EOF
)"
```

---

### Task 3: 同步文档与周边注释

**Files:**
- Modify: `README.md`
- Modify: `cmd/providers.go`（文件头与 `noListClean` 等注释 list→records）
- Modify: `internal/config/config.go`、`internal/ddns/processor.go`、`internal/ddns/display.go`（注释）
- Modify: `cmd/check.go`（可选：硬编码 Available providers 提示改为 `ddns6 list`）

- [ ] **Step 1: 更新 README 命令参考**

- 将 `### ddns6 list [provider]` 整节改为 `### ddns6 records [provider]`，示例全部改 `records`
- 新增 `### ddns6 list`：说明列运营商、示例 `ddns6 list`
- 运营商表列名 `list/clean` → `records/clean`
- FAQ「不能 list 或 clean」→「不能 records 或 clean」
- 架构图中 `list.go` 说明与命令一致
- 保留已删除的 ACME 对照表变更（若工作区仍有未提交 README 改动，一并纳入本提交）

- [ ] **Step 2: 更新代码注释**

`cmd/providers.go`：

```go
// 本文件集中注册 23 家 DNS 运营商工厂，并挂载到 run / records / clean / init。
// noListClean 为 true 时 records/clean 仅注册提示命令
```

`internal/config/config.go`：`run/check/list/clean` → `run/check/records/list/clean`

`internal/ddns/processor.go` / `display.go`：`list/clean` → `records/clean`

- [ ] **Step 3: 可选精简 `check.go` 硬编码名单**

若存在 `Available providers: tencent, cloudflare, ...`，改为：

```go
fmt.Println("Available providers: run 'ddns6 list' to see the full list")
```

- [ ] **Step 4: 全量验证**

Run: `go test ./... -count=1 && go vet ./...`

Expected: PASS

手动：

```bash
go run . list
go run . records --help
```

Expected: `list` 打印表格含 Total；`records --help` 展示列记录说明

- [ ] **Step 5: Commit**

```bash
git add README.md cmd/providers.go cmd/check.go internal/config/config.go internal/ddns/processor.go internal/ddns/display.go
git commit -m "$(cat <<'EOF'
docs: 同步 list/records 命令语义到 README 与注释

list 列运营商，records 列 DNS 记录。
EOF
)"
```

---

## Spec Coverage Checklist

| Spec 要求 | Task |
|-----------|------|
| `ddns6 list` 列全部运营商 | Task 2 |
| 表格 NAME / RECORDS/CLEAN / DESCRIPTION | Task 2 |
| 注册顺序 / noListClean→no | Task 2 |
| 原 list → records，行为不变 | Task 1 |
| 硬切换无别名 | Task 1–2 |
| README/注释/测试一致 | Task 1、3 |
| list 跳过日志初始化 | Task 2 |
| go test / go vet | Task 2–3 |
| 不改 clean / 无 JSON flag | 全局约束 |

## Self-Review

- 无 TBD/占位步骤
- `handleRecords` / `runRecordsWithConfig` / `formatProviderList` 命名前后一致
- Task 1 文件迁移与 Task 2 新建 list 顺序已写明，避免双 `listCmd` 冲突
