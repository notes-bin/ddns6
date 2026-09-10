# 设计：`list` 列运营商 + `records` 列 DNS 记录

日期：2026-09-10  
仓库：`github.com/notes-bin/ddns6`  
状态：待用户审阅 spec 后实施

## 背景与目标

当前 `ddns6 list` 语义为「列出 DNS 记录」，与「列出可用 DNS 运营商」易混淆；用户此前裸跑 `list` 时也易理解为后者。仓库已有 `providerFactories` 作为唯一运营商注册源，但缺少面向用户的查询命令，root 帮助里仍硬编码供应商名单。

成功标准：

1. `ddns6 list` 列出全部可用 DNS 运营商（不读配置、不访问网络）。
2. 原「列 DNS 记录」命令更名为 `ddns6 records`，行为与现 `list` 一致（含 provider 子命令与 `--type`）。
3. 硬切换：不保留旧 `list` 查记录的兼容别名或弃用警告。
4. README、包注释、帮助文案、错误提示、测试与命令名一致。
5. `gofmt` / `go vet` / `go test ./...` 通过。

## 方案选择

| 方案 | 说明 | 结论 |
|------|------|------|
| A | `list` 输出表格（NAME + RECORDS/CLEAN + DESCRIPTION） | **采用** |
| B | `list` 仅打印 CLI 名 | 信息过少，不采用 |
| C | JSON + 人类表格双模式 | 超出当前需求，不采用 |

命名已确认：

- 列运营商：`list`
- 列 DNS 记录：`records`
- 兼容策略：硬切换（破坏性变更，可接受）

## 命令设计

### `ddns6 list`

- 无子命令、无必填 flag。
- 数据源：`providerFactories`（与 `run`/`records`/`clean`/`init` 注册同源）。
- 输出（人类可读表格，注册顺序）：

```
NAME          RECORDS/CLEAN  DESCRIPTION
tencent       yes            Tencent Cloud DNS (DNSPod API v3) ...
duckdns       no             DuckDNS ...
...
Total: N providers
```

- `RECORDS/CLEAN`：`noListClean == true`（或等价 `restrictedProviders`）为 `no`，否则 `yes`。
- `DESCRIPTION`：使用工厂的 `short` 字段；过长时可在实现时按终端宽度截断或整行输出（优先整行，避免丢关键约束信息）。
- 不写日志文件依赖时的特殊行为：与 `version`/`init` 类似，可不初始化 JSON 日志（实现时对齐「无网络、只打印」的轻量命令）。

### `ddns6 records [provider]`

- 由现有 `list` 命令整体改名而来：`Use`/`变量名`/`文件名`/`注册函数`/`测试`/`文案` 统一改为 `records`。
- Flag、provider 子命令、配置文件模式、受限运营商拒绝逻辑保持不变；错误文案中的 `'list'` 改为 `'records'`。
- `clean` 命令名不变。

## 代码改动范围（预期）

| 区域 | 改动 |
|------|------|
| `cmd/list.go` | 改为运营商列表命令；原记录逻辑迁至 `cmd/records.go`（或同文件拆分后删除旧语义） |
| `cmd/providers.go` | `registerProviderSubCommands` 调用方、`restrictedProviders` 相关错误字符串、`noListClean` 注释中的 list → records |
| `cmd/root.go` | 命令树注释、`Long` 中硬编码名单可改为引导 `ddns6 list`；注册 `list`/`records` |
| `cmd/*_test.go` | 更新命令路径与断言 |
| `README.md` | 用法与命令说明：`list`/`records` 语义更新 |
| 其他文档/注释 | 凡写「list 列记录」处同步 |

不在范围内：

- 不改 `clean` 命名。
- 不保留 `list` → records 的隐藏别名。
- 不引入 JSON 输出 flag（除非后续单独需求）。

## 测试计划

1. `ddns6 list`：退出码 0；输出包含全部 `providerFactories` 的 `name`；受限运营商标记为 `no`。
2. `ddns6 records --help` / `ddns6 records <provider> --help` 可用。
3. 原 list 相关单测改为 records；新增 list（运营商）单测。
4. `go test ./...`、`go vet ./...` 通过。
5. README 中不再出现「list 列出 DNS 记录」的旧语义。

## 风险与兼容性

- **破坏性变更**：依赖脚本/文档中的 `ddns6 list ...` 查记录会失败，需改为 `records`。项目若已发版，属 semver minor/major 需按仓库惯例评估（命令面不兼容更接近 major；若尚未广泛依赖可在 changelog 明确说明）。
- `check` 等处若仍硬编码部分运营商名单，可顺带改为提示 `ddns6 list`，但不强制本次穷尽所有硬编码清理。
