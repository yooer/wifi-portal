# ⚡ WiFi Captive Portal SaaS 计费运营系统 (Wifi-Portal)

本系统是一个基于 **Go (Golang) + MongoDB + Redis** 开发的高可用、高性能商业级多商户 WiFi 热点短信认证 Portal SaaS 平台。支持对接 **爱快 (iKuai)、Panabit、MikroTik** 等主流网络硬件网关，并配备了双层短信计费引擎、多短信通道加权调度与一键级联销户数据清理机制。

---

## 🎨 核心界面展示

### 1. 现代苹果风系统登录与自助商户注册

采用高斯模糊（Glassmorphism）与微动画反馈的登录界面，支持短信验证码免密注册与账户自激活。

![系统登录与商户注册](images/login.png)

### 2. 商户业务大屏 (Dashboard)

实时汇总展现酒店节点总数、WiFi 连网放行次数与累计发送验证码指标，配备详细的设备对接引导与计费透明说明。

![商户业务大屏](images/dashboard.png)

### 3. 酒店及认证网关极简配置

支持图形化创建、修改对接酒店，参数直观配置。全新升级的**二次免登快捷上网（免验证码认证）**功能与短信冷却、天发送上限等参数直接落库控制，彻底摆脱静态配置文件束缚。

![新建/修改对接酒店](images/add_hotel.png)

### 4. 访客连网授权认证页面 (手机端 Captive Portal)

自适应弹出式连网认证页面，支持中国大陆 11 位手机号快速获取验证码，支持免登快捷连网，界面极致简约优美。

| 明亮模式 | 暗黑模式 |
| :---: | :---: |
| ![授权页明亮](images/portal_light.png) | ![授权页暗黑](images/portal_dark.png) |

---

## 🚀 核心技术与商业特性

* **多通道加权高可用**：原生集成 **阿里云短信 (Aliyun)**、**腾讯云短信 (Tencent)**、**互亿无线 (Ihuyi)**、**短信精灵 (SmsJingling)** 与 **本地模拟 (Mock)** 五大短信提供商。支持后台 1-10 阶梯式发送权重分配，实现多通道高容灾负载均衡与分流。
* **实时通道余量查询**：支持超级管理员在后台一键安全地查询**短信精灵**、**互亿无线**及**模拟通道**的实时短信余额，极简交互、数据透明，具备完善的防爆防挂鉴权保护。
* **双层短信扣费引擎**：优先扣减商户拥有的【短信套餐包】剩余条数，套餐消耗殆尽后自动无缝切换至扣除【账户余额】。余额不足时提供优雅的页面拦截提示。
* **物理级联销户 (Cascading Delete)**：支持超管一键安全物理删除商户主体，自动触发净化链条，级联抹除其名下全部酒店、上网审计日志 (`auth_logs`)、短信流水账单 (`sms_logs`)、充值购买流水 (`recharge`)。同时，**秒级注销被删商户在 Redis 中的所有活跃 Token**，保障极速安全下线。
* **酒店配置及审计日志级联删除**：商户与超管可在后台一键删除特定酒店网关配置，**同步级联物理清理该酒店产生的所有访客上网审计日志 (`auth_logs`)**，保持数据库数据净化度。接口搭载严格的多租户水平权限防越权鉴权锁。
* **全网财务账单对账大盘**：全新的充值与对账记录模块。商户端可追踪历史消费，**超级管理员（Level >= 50）可一键拉取并实时展示全网最新 200 条充值与财务对账流水**，提供全网统一的高可靠 ULID 对账大盘，防 `null` 空值崩溃设计。
* **首页静默化与隐藏后台挂载 (/manage)**：首页 `/` 彻底实现完全静默化零响应。彻底废弃并屏蔽了外界常用的 `/admin` 后台登录路径扫描，升级为全新隐藏入口 **/manage**，极大提高了平台的防扫描安全防线。
* **苹果级 Captive Portal 上网认证**：访客终端连接 WiFi 时自适应弹出明亮/深色模式认证页，完美兼容 iOS / Android 原生弹窗，并提供手机号本地存储免验证码免密重连。
* **守护进程及命令行服务管理**：针对 Linux 环境原生内置完备的服务子命令（`start [-d]`、`stop`、`restart`、`status`、`log`），启动服务支持以 `-d` 模式在后台静默运行并**自动将标准日志重定向输出至同级 `wifi.log` 中**。

---

## 📂 项目结构

```
wifi-portal/
├── Dockerfile           # 多阶段构建镜像
├── docker-compose.yaml # MongoDB + Redis + App 完整部署
├── config.yaml.example # 配置示例
├── config.docker.yaml  # Docker 环境配置
├── go.mod / go.sum    # Go 依赖
├── .gitignore
├── images/             # 功能截图
│   ├── login.png
│   ├── dashboard.png
│   ├── add_hotel.png
│   └── portal_*.png
├── web/                # 前端静态资源
│   ├── admin/          # 商户控制台 + 超管后台
│   └── portal/         # Captive Portal 页面
├── main.go             # 主程序入口
├── auth.go             # Session 校验、权限中间件
├── database.go         # MongoDB 驱动、索引初始化
├── session.go          # Redis 限流、会话管理
├── sms.go              # 双层扣费、多通道调度
├── handler_admin.go    # 超管/商户 API
├── handler_portal.go   # Portal 重定向、OTP 校验
└── config.go           # 配置解析
```

---

## 🛠️ 快速部署与运行

### 环境要求

| 方式 | 要求 |
| :--- | :--- |
| **Docker (推荐)** | 仅需 Docker + Docker Compose |
| **二进制** | MongoDB ≥ 4.4, Redis ≥ 6.0 |

### 2. 配置文件说明

系统运行时若检测到本地目录缺失 `config.yaml`，将**自动以正确的结构释放默认配置文件**。您也可以手动将 `config.yaml.example` 复制为 `config.yaml` 进行自定义修改。正确的配置结构如下：

```yaml
# 服务器监听端口
port: 8080

# MongoDB 数据库配置
mongodb:
  uri: "mongodb://localhost:27017/wifi"
  db_name: "wifi"

# Redis 缓存配置 (防刷限流与验证码、会话存储)
redis:
  addr: "localhost:6379"
  password: ""
  db: 0

# 短信计费规则配置
sms:
  # 默认单条扣费价格 (单位: 分。此处 6 代表 0.06 元/条)
  price_per_sms: 6

# 安全与频率限制配置 (注: 酒店网关的冷却和上限均已改用数据库动态配置，此处仅作为全局默认备份)
security:
  # 单个手机号发送短信的冷却时间 (秒)
  sms_cooldown: 60
  # 单个 IP 发送短信的冷却时间 (秒)
  ip_cooldown: 60
  # 每个手机号/IP 每日最大允许发送短信次数 (0代表不限制)
  max_sends_per_day: 5
  # 验证码有效时长 (分钟，仍为全局配置)
  code_expire_minutes: 5
  # 验证码最大尝试匹配失败次数 (超过后验证码失效)
  max_attempts: 3
```

### 3. Windows 环境运行

```powershell
# 启动服务
.\wifi.exe server start
```

### 4. Linux 环境运行 (工业级命令行服务管理)

本系统内置了完善的服务控制管理机制，支持一键在 Linux 后台以守护进程（Daemon）静默运行，并能够持久化记录物理运行日志。

#### 1. 赋予程序可执行权限
```bash
chmod +x ./wifi
```

#### 2. 系统服务管理命令说明
| 指令命令 | 功能作用 | 说明 |
| :--- | :--- | :--- |
| `./wifi server start` | **前台启动服务** | 适合前台控制台调试使用。 |
| `./wifi server start -d` | **后台守护进程启动** | 在后台静默运行，**标准日志将自动输出重定向至同级 `wifi.log` 中**。 |
| `./wifi server stop` | **安全停止服务** | 自动向运行的 PID 发送终止信号，优雅释放资源并清理 PID 文件。若无响应会自动强制杀进程。 |
| `./wifi server restart` | **后台重启服务** | 自动结束运行实例并在后台重新拉起服务。 |
| `./wifi server status` | **服务运行状态检测** | 智能读取 `wifi.pid` 文件，实时检测进程状态，如果服务未正常运行将自动清理残留进程标记。 |
| `./wifi server log` | **实时追踪系统日志** | 相当于执行 `tail -f wifi.log`，支持实时滚屏查看系统运行及认证审计的详细日志。 |

### 5. Docker 部署

`docker-compose.yaml` 包含 MongoDB + Redis + App 完整依赖，开箱即用。

```bash
# 构建并启动全部服务 (MongoDB + Redis + App)
docker-compose up -d --build

# 查看日志
docker-compose logs -f wifi-portal

# 停止服务
docker-compose down
```

#### 单独构建

```bash
# 构建镜像
docker build -t wifi-portal .

# 运行容器 (需自行准备 MongoDB/Redis)
docker run -d -p 8080:8080 --name wifi-portal wifi-portal
```

#### 多平台镜像

```bash
# Linux amd64
$env:GOOS="linux"; $env:GOARCH="amd64"
docker build --platform linux/amd64 -t wifi-portal:amd64 .

# Linux arm64
$env:GOOS="linux"; $env:GOARCH="arm64"
docker build --platform linux/arm64 -t wifi-portal:arm64 .
```

---

## 🔒 系统初始化账号

* **超级管理员 (Super Admin)**  
  * 账号：`13703770377`  
  * 密码：`aa123456`
* **默认酒店商户 (Merchant)**  
  * 账号：`13803770377`  
  * 密码：`123456`
