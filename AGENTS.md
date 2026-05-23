# tools/ikuai-portal/ Agent 指南

本目录维护 WiFi 认证 Portal SaaS 系统的 Go 后端核心逻辑与编译资产。

## 职责

- 负责访客连网请求、超管多通道短信池管理和双层扣减账单的安全执行。
- 处理路由器（iKuai / Panabit / MikroTik）签名、重定向放行及时间同步。

## 约束

- **不提交可执行文件**：确保 `ikuai-portal.exe` 和 `check-build.exe` 被 git 忽略，或不将其添加至提交清单中。
- **本地敏感信息**：千万不要在 `config.yaml` 提交真实的云服务 AccessKey/Secret；使用 MongoDB 的掩码模式保护。
- **原子扣费**：任何涉及到扣减 `sms_count` 或 `balance` 的改动，必须使用 MongoDB 的原子操作（如 `$inc` 加以比较过滤），严防高并发下的扣费透支与 Race Condition。

## 常用命令

```bash
# 1. 本地编译
go build -o ikuai-portal.exe .

# 2. 运行服务
./ikuai-portal.exe
```
