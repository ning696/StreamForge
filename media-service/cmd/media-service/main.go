// media-service 的可执行程序入口。
//
// 【Go 项目布局】
// 按 Go 社区惯例，可执行程序的 main 包放在 cmd/<binary-name>/ 目录下。
// 目录名 "media-service" 就是最终编译出来的二进制名字。
// internal/ 下的包只能被本项目 import，是一种"物理级别"的封装。
package main

import (
	"context"    // 上下文，用于传超时/取消信号
	"log"        // 简单日志
	"net/http"   // 标准 HTTP 服务
	"os"         // 进程环境（获取信号）
	"os/signal"  // 监听系统信号
	"strconv"    // int -> string
	"syscall"    // SIGTERM 常量在这里
	"time"       // 超时时长

	// 本项目内部包。import 路径的第一段是 go.mod 里的 module 名。
	"streamforge/media-service/internal/config"
	"streamforge/media-service/internal/gateway"
	// 别名 livekittoken：避免和目录里可能出现的其他 livekit 变量重名，
	// 也让代码读起来更清楚（"这是签 Token 的包"）。
	livekittoken "streamforge/media-service/internal/livekit"
	"streamforge/media-service/internal/room"
	"streamforge/media-service/internal/state"
)

// main 是每个 Go 可执行程序的起点。
//
// 【整体流程】
//   1. 加载配置
//   2. 初始化各依赖：Redis、房间存储、LiveKit 签发器、内存 Hub
//   3. 用它们组装出 gateway.Server
//   4. 起一个 http.Server 监听端口
//   5. 单独起一个 goroutine 等待 Ctrl-C / SIGTERM，收到就优雅关机
//   6. 阻塞在 ListenAndServe，直到服务停止
func main() {
	// 1) 加载配置（会自动读 .env + 环境变量）。
	cfg := config.Load()

	// 2) signal.NotifyContext 返回一个 ctx，当进程收到 os.Interrupt（Ctrl-C）
	//    或 SIGTERM（k8s / docker stop 用的信号）时会自动取消。
	//    我们随后用这个 ctx 触发"优雅关机"。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// defer stop() 在 main 退出时释放 signal 监听。虽然进程都要死了，但这是好习惯。
	defer stop()

	// 3) 组装依赖。这里能明显看出"上层依赖下层，下层不知道上层"的依赖方向。
	redisClient := state.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	store := state.NewRedisStore(redisClient)
	issuer := livekittoken.NewIssuer(livekittoken.FromConfig(cfg))
	server := gateway.NewServer(cfg, store, issuer, room.NewHub())

	// 4) 用 http.Server 而不是 http.ListenAndServe(...) 直接启动。
	//    是为了拿到 Server 引用，好在关机时调用 Shutdown()。
	httpServer := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.ServerPort), // ":8080" 这样的字符串
		Handler: server.Handler(),
		// ReadHeaderTimeout 防止"慢速攻击"：客户端故意慢慢发 header 拖住连接。
		// 强烈建议给任何暴露到公网的 HTTP 服务加上这个超时。
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 5) 优雅关机：单独起一个 goroutine 监听退出信号。
	//    go func() { ... }() 是"启动一个新 goroutine 并执行匿名函数"的语法。
	go func() {
		<-ctx.Done() // 阻塞直到收到信号（ctx 被取消）
		// 给 Shutdown 一个 5 秒的时限，防止某些请求死活不返回，卡住关机流程。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// http.Server.Shutdown 会：
		//   - 停止接受新连接
		//   - 等待正在处理中的请求跑完（或直到 shutdownCtx 到期）
		// 之后 ListenAndServe 会返回 http.ErrServerClosed，主 goroutine 就能退出了。
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("media-service room/token service listening on :%d", cfg.ServerPort)

	// 6) 阻塞式启动 HTTP 服务。
	//    正常关机时返回 ErrServerClosed，我们把这个错误当"正常退出"处理。
	//    其他错误（比如端口被占用）走 log.Fatal，会立即打日志然后 os.Exit(1)。
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
