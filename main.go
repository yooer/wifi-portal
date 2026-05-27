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
	"strconv"
	"strings"
	"time"
)

//go:embed web
var embeddedWebFS embed.FS

func main() {
	// 支持 Linux 服务命令控制 start, stop, restart, status
	ProcessCommand()
	defer os.Remove(pidFile)

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
	http.HandleFunc("/api/admin/sms-providers/balance", GlobalAuth.RequireAdmin(HandleSMSProviderBalance))

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
	http.HandleFunc("/api/merchant/hotels/allocate-sms", GlobalAuth.RequireAuth(HandleMerchantAllocateSMS))
	http.HandleFunc("/api/merchant/buy-package", GlobalAuth.RequireAuth(HandleMerchantBuyPackage))
	http.HandleFunc("/api/merchant/auth-logs", GlobalAuth.RequireAuth(HandleMerchantAuthLogs))
	http.HandleFunc("/api/merchant/sms-logs", GlobalAuth.RequireAuth(HandleMerchantSMSLogs))
	http.HandleFunc("/api/merchant/recharges", GlobalAuth.RequireAuth(HandleMerchantRechargeRecords))

	// 5. 挂载已嵌入二进制包的静态资源文件
	subPortalFS, err := fs.Sub(embeddedWebFS, "web/portal")
	if err != nil {
		log.Fatalf("❌ 解析静态 Portal 嵌入资源失败: %v", err)
	}
	subAdminFS, err := fs.Sub(embeddedWebFS, "web/admin")
	if err != nil {
		log.Fatalf("❌ 解析静态管理嵌入资源失败: %v", err)
	}

	http.Handle("/portal/", http.StripPrefix("/portal/", http.FileServer(http.FS(subPortalFS))))
	http.Handle("/manage/", http.StripPrefix("/manage/", http.FileServer(http.FS(subAdminFS))))

	// 提供便捷的根路径与目录非斜线请求 301 重定向跳转
	http.HandleFunc("/portal", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/portal/", http.StatusMovedPermanently)
	})
	http.HandleFunc("/manage", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/manage/", http.StatusMovedPermanently)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// 首页完全静默，不输出任何内容
		w.WriteHeader(http.StatusOK)
	})

	// 6. 开启 HTTP 守护监听服务进程
	serverAddr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🌐 WiFi Captive Portal SaaS 服务已就绪，服务网关地址: http://127.0.0.1%s/manage\n", serverAddr)
	
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("❌ HTTP 服务监听异常宕机: %v", err)
	}
}

// ProcessCommand 解析并处理 Linux 服务管理子命令 (start, stop, restart, status)
func ProcessCommand() {
	if len(os.Args) < 2 {
		return
	}

	// 如果第一个参数不是 "server"，说明可能是普通运行或者不符合新子命令，直接返回
	if os.Args[1] != "server" {
		// 为了向下兼容，如果有人误输了 ./wifi -d，我们予以友好提示
		for _, arg := range os.Args[1:] {
			if arg == "-d" {
				fmt.Println("⚠️ 提示: 旧有启动方式 'wifi -d' 已废弃。请使用 'wifi server start -d' 启动服务。")
				os.Exit(1)
			}
		}
		printUsage()
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	action := os.Args[2]
	switch action {
	case "start":
		handleStart()
	case "stop":
		handleStop()
		os.Exit(0)
	case "restart":
		handleRestart()
		os.Exit(0)
	case "status":
		handleStatus()
		os.Exit(0)
	case "log", "logs":
		handleLog()
		os.Exit(0)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("⚡ WiFi Captive Portal SaaS 计费运营系统\n")
	fmt.Printf("用法:\n")
	fmt.Printf("  %s server start [-d]   - 启动服务 (带 -d 标志以守护进程在后台静默高可用运行)\n", os.Args[0])
	fmt.Printf("  %s server stop         - 停止服务\n", os.Args[0])
	fmt.Printf("  %s server restart      - 重启服务 (默认在后台启动)\n", os.Args[0])
	fmt.Printf("  %s server status       - 检查服务运行状态\n", os.Args[0])
	fmt.Printf("  %s server log          - 实时查看服务运行日志 (相当于 tail -f wifi.log)\n", os.Args[0])
}

const pidFile = "wifi.pid"

// 检查 PID 是否正在活跃运行
func isProcessRunning(pid int) bool {
	// 在 Linux/Unix 环境下，kill -0 是最轻量标准检测
	cmd := exec.Command("kill", "-0", strconv.Itoa(pid))
	err := cmd.Run()
	return err == nil
}

func handleStart() {
	// 1. 检查是否已经在运行
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, errConv := strconv.Atoi(strings.TrimSpace(string(data))); errConv == nil {
			if isProcessRunning(pid) {
				fmt.Printf("⚠️ 警告: 服务已在运行中 (PID: %d)，请勿重复启动。\n", pid)
				os.Exit(1)
			}
		}
	}

	// 2. 检查是否需要守护进程后台启动
	daemonMode := false
	for _, arg := range os.Args[3:] {
		if arg == "-d" {
			daemonMode = true
			break
		}
	}

	if daemonMode {
		// 克隆自身，但移除 -d 确保子进程进入正常启动逻辑
		var args []string
		for _, arg := range os.Args[1:] {
			if arg != "-d" {
				args = append(args, arg)
			}
		}
		cmd := exec.Command(os.Args[0], args...)
		cmd.Stdin = nil
		
		// 重定向 Stdout 和 Stderr 写入 wifi.log 文件，提供完善的物理日志支持
		logFile, errFile := os.OpenFile("wifi.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if errFile == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
		
		err := cmd.Start()
		if err != nil {
			log.Fatalf("❌ 无法以守护进程模式启动: %v\n", err)
		}
		
		fmt.Printf("🚀 成功在后台守护进程模式运行！PID: %d\n", cmd.Process.Pid)
		os.Exit(0)
	}

	// 3. 前台启动或子进程启动，写入 PID 文件
	myPid := os.Getpid()
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(myPid)), 0644)
	if err != nil {
		log.Printf("⚠️ 警告: 无法写入 PID 文件 %s: %v\n", pidFile, err)
	}
}

func handleStop() {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("❌ 错误: 未找到 PID 文件 wifi.pid，服务可能未启动。")
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Println("❌ 错误: PID 文件损坏，无法解析进程 ID。正在自动清理 PID 文件...")
		os.Remove(pidFile)
		return
	}

	if !isProcessRunning(pid) {
		fmt.Printf("ℹ️ 提示: 服务未在运行 (发现残留 PID 文件 %d)，正在自动清理...\n", pid)
		os.Remove(pidFile)
		return
	}

	fmt.Printf("🔌 正在终止 WiFi 门户服务进程 (PID: %d)...\n", pid)
	// 发送 SIGTERM (15) 优雅停止信号
	exec.Command("kill", "-15", strconv.Itoa(pid)).Run()

	// 等待最多 5 秒让进程优雅退出
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if !isProcessRunning(pid) {
			os.Remove(pidFile)
			fmt.Println("❇️ 服务已成功停止！")
			return
		}
	}

	// 5 秒未退出，强制杀死 (SIGKILL -9)
	fmt.Println("⚠️ 进程未响应优雅停止，正在强制杀死...")
	exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
	os.Remove(pidFile)
	fmt.Println("❇️ 服务已强制停止！")
}

func handleRestart() {
	// 1. 停止原服务
	handleStop()
	time.Sleep(1 * time.Second)

	// 2. 以守护进程方式在后台重新拉起
	fmt.Println("🚀 正在后台重新拉起服务...")
	cmd := exec.Command(os.Args[0], "server", "start", "-d")
	cmd.Stdin = nil
	
	logFile, errFile := os.OpenFile("wifi.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if errFile == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	
	err := cmd.Start()
	if err != nil {
		log.Fatalf("❌ 无法重新拉起服务: %v\n", err)
	}
	fmt.Printf("🚀 服务重启成功！最新后台进程 PID: %d\n", cmd.Process.Pid)
}

func handleStatus() {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("🔴 服务状态: 未在运行")
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		fmt.Println("🔴 服务状态: 未在运行 (发现损坏的 PID 文件，已清理)")
		os.Remove(pidFile)
		return
	}

	if isProcessRunning(pid) {
		fmt.Printf("🟢 服务状态: 正在运行中 (PID: %d)\n", pid)
	} else {
		fmt.Printf("🔴 服务状态: 未在运行 (发现残留 PID 文件 %d，已自动清理)\n", pid)
		os.Remove(pidFile)
	}
}

func handleLog() {
	_, err := os.Stat("wifi.log")
	if os.IsNotExist(err) {
		fmt.Println("ℹ️ 提示: 尚未生成 wifi.log 日志文件。")
		return
	}

	fmt.Println("📋 正在实时追踪服务运行日志 (按 Ctrl+C 退出)...")
	// 在 Linux/Unix 下直接调用 tail -f 能获得完美原生响应和缓冲输出
	cmd := exec.Command("tail", "-n", "100", "-f", "wifi.log")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// 备用 fallback：若系统不支持 tail (如 Windows 调试)，直接输出最后 50 行
		data, _ := os.ReadFile("wifi.log")
		lines := strings.Split(string(data), "\n")
		start := 0
		if len(lines) > 50 {
			start = len(lines) - 50
		}
		for _, line := range lines[start:] {
			fmt.Println(line)
		}
	}
}
