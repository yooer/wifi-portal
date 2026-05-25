# tools/ikuai-portal/ Agent 指南

本目录维护 WiFi 认证 Portal SaaS 系统的 Go 后端核心逻辑与编译资产。

## 职责

- 负责访客连网请求、超管多通道短信池管理和双层扣减账单的安全执行。
- 处理路由器（iKuai / Panabit / MikroTik）签名、重定向放行及时间同步。

## 约束

- **不提交非白名单可执行文件**：确保 `ikuai-portal.exe` 和 `check-build.exe` 被 git 忽略，只有用户要求的 `wifi` 和 `wifi.exe` 可以被提交。
- **本地敏感信息**：千万不要在 `config.yaml` 提交真实的云服务 AccessKey/Secret；使用 MongoDB 的掩码模式保护。
- **原子扣费**：任何涉及到扣减 `sms_count` 或 `balance` 的改动，必须使用 MongoDB 的原子操作（如 `$inc` 加以比较过滤），严防高并发下的扣费透支与 Race Condition。

## 常用命令

```bash
# 1. 本地 Windows 编译
go build -o wifi.exe .

# 2. 本地 Linux 交叉编译
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o wifi .

# 3. 本地原生直接运行
.\wifi.exe

# 4. 运行安全引导程序生成随机凭证并自动生成 Docker 配置
python bootstrap.py

# 5. Docker Compose 一键启动生态 (MongoDB + Redis + App)
docker-compose up -d

# 6. 查看 Docker App 运行及验证码日志
docker-compose logs -f wifi-portal

# 7. ⚠️ 针对 WiFi 业务服务单独重启（数据库不受干扰，会话不丢失）
docker-compose restart wifi-portal

# 8. 停止 Docker Compose 生态
docker-compose down
```
