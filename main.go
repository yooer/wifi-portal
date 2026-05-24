package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

//go:embed web
var embeddedWebFS embed.FS

func main() {
	// 支持后台 -d 守护进程运行模式
	Daemonize()

	log.Println("🚀 正在启动 WiFi Captive Portal SaaS 系统...")

	// 1. 初始化鉴权密钥系统
	InitAuth()

	// 2. 加载 YAML 配置文件 (若不存在则自动以 tools/ikuai-portal/config.yaml 为模板释放)
	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("❌ 严重错误: 无法加载或自动释放 config.yaml 配置文件 (%v)", err)
	}
	log.Printf("⚙️ 配置加载成功，监听端口: %d\n", cfg.Port)

	// 3. 初始化底座连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 连接 MongoDB
	err = InitMongoDB(ctx, cfg.MongoDB.URI, cfg.MongoDB.DBName)
	if err != nil {
		log.Fatalf("❌ MongoDB 连接初始化失败: %v", err)
	}

	// 连接 Redis
	err = InitRedis(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.Fatalf("❌ Redis 连接初始化失败: %v", err)
	}

	// 4. 注册 API 业务网关路由
	
	// Portal 终端网关访客接口 (无鉴权，高防刷防爬限流控制)
	http.HandleFunc("/portal/config", HandlePortalConfig)
	http.HandleFunc("/portal/sms/send", HandleSMSReleaseSend)
	http.HandleFunc("/portal/sms/verify", HandleSMSReleaseVerify)
	http.HandleFunc("/portal/sms/bypass", HandleSMSReleaseBypass)
	http.HandleFunc("/hotel/", HandleHotelRedirect)


	// SaaS 运营后台会话接口
	http.HandleFunc("/api/admin/login", HandleAdminLogin)
	http.HandleFunc("/api/admin/logout", HandleAdminLogout)
	http.HandleFunc("/api/admin/register/send-sms", HandleAdminRegisterSendSMS)
	http.HandleFunc("/api/admin/register", HandleAdminRegister)
	http.HandleFunc("/api/admin/profile", GlobalAuth.RequireAuth(HandleAdminProfile))

	// SaaS 超级管理员独占接口 (RequireAdmin 拦截)
	http.HandleFunc("/api/admin/users", GlobalAuth.RequireAdmin(HandleSuperUsers))
	http.HandleFunc("/api/admin/hotels", GlobalAuth.RequireAdmin(HandleSuperHotels))
	http.HandleFunc("/api/admin/recharge/manual", GlobalAuth.RequireAdmin(HandleSuperRechargeManual))
	http.HandleFunc("/api/admin/sms-providers", GlobalAuth.RequireAdmin(HandleSuperSMSProviders))

	// 套餐包接口特殊路由 (GET 允许商户查看以购买，POST/PUT/DELETE 限超管)
	http.HandleFunc("/api/admin/packages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			GlobalAuth.RequireAuth(HandleSuperPackages)(w, r)
		} else {
			GlobalAuth.RequireAdmin(HandleSuperPackages)(w, r)
		}
	})

	// SaaS 酒店商户专属接口 (RequireAuth 拦截)
	http.HandleFunc("/api/merchant/hotels", GlobalAuth.RequireAuth(HandleMerchantHotels))
	http.HandleFunc("/api/merchant/buy-package", GlobalAuth.RequireAuth(HandleMerchantBuyPackage))
	http.HandleFunc("/api/merchant/auth-logs", GlobalAuth.RequireAuth(HandleMerchantAuthLogs))
	http.HandleFunc("/api/merchant/sms-logs", GlobalAuth.RequireAuth(HandleMerchantSMSLogs))
	http.HandleFunc("/api/merchant/recharges", GlobalAuth.RequireAuth(HandleMerchantRechargeRecords))

	// 5. 挂载已嵌入二进制包的静态资源文件
	subFS, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		log.Fatalf("❌ 解析静态嵌入资源失败: %v", err)
	}
	
	fileServer := http.FileServer(http.FS(subFS))
	http.Handle("/portal/", fileServer)
	http.Handle("/admin/", fileServer)

	// 提供便捷的根路径与目录非斜线请求 301 重定向跳转
	http.HandleFunc("/portal", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/portal/", http.StatusMovedPermanently)
	})
	http.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusFound)
	})

	// 6. 开启 HTTP 守护监听服务进程
	serverAddr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🌐 WiFi Captive Portal SaaS 服务已就绪，服务网关地址: http://127.0.0.1%s/admin\n", serverAddr)
	
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("❌ HTTP 服务监听异常宕机: %v", err)
	}
}

// Daemonize 处理后台 -d 守护进程运行模式 (脱离终端控制)
func Daemonize() {
	daemonMode := false
	for _, arg := range os.Args[1:] {
		if arg == "-d" {
			daemonMode = true
			break
		}
	}

	if daemonMode {
		// 移除 -d 参数，防子进程循环自我克隆
		var args []string
		for _, arg := range os.Args[1:] {
			if arg != "-d" {
				args = append(args, arg)
			}
		}

		cmd := exec.Command(os.Args[0], args...)
		
		// 重定向 Stdin, Stdout, Stderr 彻底分离父进程终端
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		
		err := cmd.Start()
		if err != nil {
			log.Fatalf("❌ 无法以守护进程模式启动: %v\n", err)
		}
		
		fmt.Printf("🚀 成功在后台守护进程模式运行！PID: %d\n", cmd.Process.Pid)
		os.Exit(0)
	}
}
