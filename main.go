package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"home-broadband/internal/config"
	"home-broadband/internal/panel"
	"home-broadband/internal/tunnel"
	"home-broadband/internal/web"
)

// version 由构建时通过 -ldflags 注入。
var version = "dev"

func main() {
	var (
		webPort  = flag.Int("web", 8899, "Web 管理端口")
		maxSlots = flag.Int("max", 20, "最多同时运行的隧道数")
		workDir  = flag.String("dir", "/var/lib/home-broadband", "工作目录")
	)
	panelMode := flag.String("panel", "", "节点链接后端: 留空按界面设置/自动探测, 3x-ui, native")
	publicIP := flag.String("ip", "", "母机公网 IPv4，用于分享链接/SOCKS5 地址；留空则自动探测")
	showVersion := flag.Bool("version", false, "显示版本后退出")
	flag.Parse()

	// web 包里自更新等逻辑要显示版本号。
	web.Version = version

	if *publicIP == "" {
		*publicIP = os.Getenv("HOMEBROADBAND_PUBLIC_IP")
	}

	if *showVersion {
		fmt.Println("home-broadband", version)
		return
	}

	if os.Geteuid() != 0 {
		log.Fatal("需要 root 权限（要创建 netns 和改 iptables）")
	}
	if err := os.MkdirAll(*workDir, 0700); err != nil {
		log.Fatalf("创建工作目录失败: %v", err)
	}

	// 打开统一配置存储（老版本散落的小文件会在这里一次性迁入 config.json）。
	// 之后所有无 dir 参数的配置访问都走这份全局存储。
	if _, err := config.Init(*workDir); err != nil {
		log.Fatalf("打开配置存储失败: %v", err)
	}

	// 先记下母机的网络命名空间，后面所有子进程都从这里起。
	// 必须赶在建任何隧道之前，那之后线程就可能被带进隧道里了
	if err := tunnel.InitMainNetns(); err != nil {
		log.Fatal(err)
	}

	// 同一个工作目录只许跑一个实例：两份会共用 state.json 互相覆盖，隧道记录直接丢
	unlock, err := tunnel.LockWorkDir(*workDir)
	if err != nil {
		log.Fatal(err)
	}
	defer unlock()

	// 定下这台机器上属于本实例的 netns 名与网段。默认目录沿用老名字，
	// 换了目录就自动隔离，免得两个实例互相拆隧道（见 internal/tunnel/instance.go）
	if err := tunnel.InitInstance(*workDir); err != nil {
		log.Fatalf("初始化实例标识失败: %v", err)
	}
	if tunnel.InstanceTag() != "" {
		log.Printf("非默认工作目录，本实例用 netns hb%s* 与网段 10.%d.x", tunnel.InstanceTag(), tunnel.InstanceBase())
	}

	tunnel.SetPublicIPOverride(*publicIP)
	go tunnel.HostPublicIP() // 预热探测，别让首个请求阻塞
	if err := tunnel.PrepareHost(); err != nil {
		log.Fatal(err)
	}

	panel.ConfigurePanel(*workDir, *panelMode)

	mgr := tunnel.NewManager(*maxSlots, *workDir)
	if p, err := panel.OpenPanel(); err != nil {
		log.Printf("节点链接后端暂不可用（可在 Web 界面查看原因）: %v", err)
	} else {
		log.Printf("节点链接后端: %s", p.Describe())
		mgr.SetBackend(p)
	}
	log.Printf("正在拉取节点列表...")
	if n, err := mgr.RefreshNodes(); err != nil {
		log.Printf("拉取失败（可在 Web 界面重试）: %v", err)
	} else {
		log.Printf("已获取 %d 个节点", n)
	}

	if n, err := mgr.RestoreState(); err != nil {
		log.Printf("恢复上次状态失败: %v", err)
	} else if n > 0 {
		log.Printf("正在恢复上次的 %d 条隧道", n)
		// 3x-ui 模式重启不会自动重写面板出站，旧版本升上来时面板里的 socks
		// 出站没有认证字段，端口一旦要认证就连不上，这里恢复后对账一次
		go mgr.ReconcileOutbounds()
	}

	go mgr.WatchHealth()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("正在清理所有隧道...")
		mgr.Shutdown()
		panel.ClosePanel()
		unlock() // os.Exit 会绕过 defer，这里手动放锁
		os.Exit(0)
	}()

	mux := http.NewServeMux()

	auth, created, err := web.NewAuth(*workDir)
	if err != nil {
		log.Fatalf("初始化访问口令失败: %v", err)
	}
	if created {
		log.Printf("已生成访问口令，见 %s", filepath.Join(*workDir, "password"))
	}

	bpCreated, err := config.InitBasePath(*workDir)
	if err != nil {
		log.Fatalf("初始化访问路径失败: %v", err)
	}
	if bpCreated {
		log.Printf("已生成访问路径，见 %s", filepath.Join(*workDir, "basepath"))
	}

	// 用户显式给了 -web 就以命令行为准，否则沿用界面上存过的端口
	portExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "web" {
			portExplicit = true
		}
	})
	webCfg, err := config.LoadWebSettings(*workDir, *webPort, portExplicit)
	if err != nil {
		log.Fatalf("加载 Web 设置失败: %v", err)
	}

	srv := web.NewWebServer(config.StripBasePath(auth.Wrap(mux)))
	web.RegisterRoutes(mux, mgr, *workDir, auth, srv)

	log.Printf("管理界面: http://<本机IP>%s%s/", webCfg.ListenAddrString(), config.CurrentBasePath())
	log.Printf("SOCKS5 端口在 %d-%d 之间随机分配", tunnel.RandPortMin, tunnel.RandPortMax)
	if err := srv.Serve(); err != nil {
		log.Fatal(err)
	}
}
