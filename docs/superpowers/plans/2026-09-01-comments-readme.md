# 注释与 README 全量更新 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按 `docs/superpowers/specs/2026-09-01-comments-readme-design.md`，全量统一中文注释并重写 README，且不改变运行时行为。

**Architecture:** 先分离工作区已有 simplify 改动；再按包分批重写注释（包注释唯一、导出 godoc、未导出意图注释、测试用途说明）；最后整体重写 README。每批 `gofmt` + 相关 `go test`。

**Tech Stack:** Go 1.25.2、cobra CLI、现有 providers / pkg / internal 结构。

## Global Constraints

- 注释语言：简体中文；标识符英文；错误日志/API 文案保持英文
- 不改函数签名、不改业务逻辑、不重构
- 每个包仅一处 `// Package`（非测试文件）
- 验证：`gofmt` + `go test ./...`
- Spec：`docs/superpowers/specs/2026-09-01-comments-readme-design.md`

---

### Task 0: 分离并提交 simplify 改动

**Files:**
- Modify: 工作区已改的 test / `test.yml`（先前 simplify）

- [ ] **Step 1:** `git status` / `git diff` 确认仅含 simplify
- [ ] **Step 2:** 提交 `refactor(test): 表驱动与 ApiError 断言简化`（或等价中文 conventional commit）
- [ ] **Step 3:** 确认工作区干净（或仅剩后续注释改动）

---

### Task 1: `main` + `cmd` 注释

**Files:**
- Modify: `main.go`, `cmd/*.go`（含 `*_test.go`）

- [ ] **Step 1:** 重写/补齐包注释、导出与未导出关键符号、命令注册相关注释
- [ ] **Step 2:** `gofmt` + `go test ./cmd ./...`（至少 `./cmd`）
- [ ] **Step 3:** Commit `docs(cmd): 全量更新 CLI 包中文注释`

---

### Task 2: `internal/ddns` + `config` + `crypto`

**Files:**
- Modify: `internal/ddns/*.go`, `internal/config/*.go`, `internal/crypto/*.go`（含测试）

- [ ] **Step 1:** 统一包/类型/方法注释；清理低信息行内注释
- [ ] **Step 2:** `go test ./internal/ddns ./internal/config ./internal/crypto`
- [ ] **Step 3:** Commit `docs(internal): 更新 ddns/config/crypto 中文注释`

---

### Task 3: `pkg/*`

**Files:**
- Modify: `pkg/domainutil`, `pkg/ipaddr`, `pkg/retry`（含测试与 example）

- [ ] **Step 1:** 重写包与公开 API 注释；测试用途说明
- [ ] **Step 2:** `go test ./pkg/...`
- [ ] **Step 3:** Commit `docs(pkg): 更新 domainutil/ipaddr/retry 中文注释`

---

### Task 4: `internal/providers`（可拆多 commit）

**Files:**
- Modify: `internal/providers/<name>/*.go` 全部 23 家（含测试）

分组建议：A 国内云 / B Cloudflare+国际常见 / C 免费 DDNS / D aws/gcloud/azure/namecheap/dpi

- [ ] **Step 1:** 每组按规范更新 Package、Client、CRUD、Option、签名辅助注释与测试注释
- [ ] **Step 2:** 每组 `go test` 对应包
- [ ] **Step 3:** 每组或整批 Commit `docs(providers): 更新 … 中文注释`

---

### Task 5: README 整体重写

**Files:**
- Modify: `README.md`

- [ ] **Step 1:** 按 spec 大纲重写；核对 23 providers、`test.yml`、Docker、Makefile
- [ ] **Step 2:** 人工核对运营商表与 `cmd/providers.go` 一致
- [ ] **Step 3:** Commit `docs(readme): 按现状整体重写使用说明`

---

### Task 6: 全量验收

- [ ] **Step 1:** `gofmt -l .` 无输出（或仅预期文件已格式化）
- [ ] **Step 2:** `go test ./... -count=1`
- [ ] **Step 3:** 快速扫包注释唯一性与 README 项目结构节
- [ ] **Step 4:** 向用户报告完成情况与 commit 列表（是否 push 由用户决定）
