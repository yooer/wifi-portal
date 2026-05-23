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

* **多商户 SaaS 架构**：超级管理员（Level 100）与普通酒店商户（Level 10）双层权限体系，超管专区支持全局商户增删改查、注资充值、短信通道调配。
* **双层短信扣费引擎**：优先扣减商户拥有的【短信套餐包】剩余条数，套餐消耗殆尽后自动无缝切换至扣除【账户余额】。余额不足时提供优雅的页面拦截提示。
* **物理级联销户 (Cascading Delete)**：支持超管一键安全物理删除商户主体，自动触发净化链条，级联抹除其名下全部酒店、上网审计日志 (`auth_logs`)、短信流水账单 (`sms_logs`)、充值购买流水 (`recharge`)。同时，**秒级注销被删商户在 Redis 中的所有活跃 Token**，保障极速安全下线。
* **苹果级 Captive Portal 上网认证**：访客终端连接 WiFi 时自适应弹出明亮/深色模式认证页，完美兼容 iOS / Android 原生弹窗，并提供手机号本地存储免验证码免密重连。
* **守护进程支持 (`-d`)**：针对 Linux 服务器环境，原生支持 `-d` 命令行 Flag，执行克隆分裂脱离控制终端，以守护进程（Daemon）模式在后台静默高可用运行。

---

## 📂 模块目录结构

```text
tools/ikuai-portal/
├── main.go               # 系统主程序入口与 -d 守护进程初始化
├── auth.go               # 基于 Session 的高并发登录态校验与权限中间件
├── database.go           # MongoDB 数据库驱动、索引检查与种子数据自动初始化
├── session.go            # Redis 短信频次限流、冷却控制与全局活跃会话管理器
├── sms.go                # 双层扣费原子交易控制与多通道加权调度发送引擎
├── handler_admin.go      # 超管/商户资料维护、注资充值、物理级联销户 API 控制器
├── handler_portal.go     # 网关重定向放行、OTP 验证码校验、快捷免认证 API 控制器
├── config.go             # YAML 配置文件高扩展解析器
├── config.yaml           # 本地数据库、Redis 与全局短信冷却参数配置文件
├── images/               # 核心功能界面高清截图目录
└── web/                  # 静态资源前端包
    ├── admin/            # 苹果风格 SaaS 商户控制台与超管运营后台
    └── portal/           # 访客 Captive Portal 原生连网适配页面
```

---

## 🛠️ 快速部署与运行

### 1. 前置环境要求

* **MongoDB**：用于存储系统持久化账户、酒店节点、财务及连网日志（支持单机或副本集）。
* **Redis**：用于存储高并发会话 Token、OTP 短信冷却频次记录与临时验证码。

### 2. 配置文件说明

将 `config.yaml.example` 复制为 `config.yaml` 并填写您的 MongoDB 及 Redis 连接串，以及全局短信防刷安全兜底参数：

```yaml
server:
  port: ":8080"            # 服务监听端口

database:
  mongo_uri: "mongodb://localhost:27017"
  db_name: "ikuai_portal"
  redis_addr: "localhost:6379"
  redis_pass: ""
  redis_db: 0

security:
  sms_cooldown: 60         # 手机号发送短信冷却时间（秒）
  ip_cooldown: 60          # 单IP发送短信冷却时间（秒）
  max_sends_day: 10        # 单手机号/IP每日验证码发送上限（次，0为不限）
```

### 3. Windows 环境运行

```bash
# 启动程序（自动执行 MongoDB 索引建立与超管种子数据生成）
.\wifi.exe
```

### 4. Linux 环境运行 (守护进程模式)

```bash
# 1. 赋予可执行权限
chmod +x ./wifi

# 2. 启用 -d 标志在后台静默运行
./wifi -d
# 控制台输出：🚀 成功在后台守护进程模式运行！PID: [子进程PID]

# 3. 验证运行状态
ps -ef | grep wifi
```

---

## 🔒 系统初始化账号

* **超级管理员 (Super Admin)**  
  * 账号：`13703770377`  
  * 密码：`aa123456`
* **默认酒店商户 (Merchant)**  
  * 账号：`13803770377`  
  * 密码：`123456`
