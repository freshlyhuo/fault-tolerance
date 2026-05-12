package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fault-tolerance/pkg/confighub"
	"health-monitor/pkg/configrpc"
	"health-monitor/pkg/state"
)

func main() {
	// 命令行参数
	enableConfigRPC := flag.Bool("enable-config-rpc", true, "是否启用VSOA配置RPC服务")
	configRPCAddr := flag.String("config-rpc-addr", "127.0.0.1:3001", "VSOA配置RPC服务监听地址")
	flag.Parse()

	fmt.Printf("========== 健康监控系统启动 ==========\n")
	fmt.Println("存储模式: 纯内存缓存")
	if *enableConfigRPC {
		fmt.Printf("配置RPC服务: 已启用 (%s)\n", *configRPCAddr)
	} else {
		fmt.Println("配置RPC服务: 未启用")
	}
	fmt.Println("硬件指标采集: 发布订阅方式（旧RPC采样入口已移除）")
	fmt.Println("ECSM微服务监测: 已移除")
	fmt.Println("======================================")
	fmt.Println()

	// 创建 context，用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 0. 初始化状态管理器
	fmt.Println("初始化状态管理器...")
	sm, err := state.NewStateManager()
	if err != nil {
		fmt.Printf("❌ 初始化状态管理器失败: %v\n", err)
		os.Exit(1)
	}
	defer sm.Close()

	// 1. 启动配置RPC服务。硬件指标采集改由发布订阅链路接入，不再注册旧RPC采样路由。
	var hubRPC *confighub.ConfigHubServer
	if *enableConfigRPC {
		fmt.Printf("启动配置RPC服务 (%s)...\n", *configRPCAddr)
		hubRPC = confighub.NewConfigHubServerWithServices(*configRPCAddr, configrpc.NewRuntimeConfigService(), nil, nil)
		if err := hubRPC.Start(); err != nil {
			fmt.Printf("❌ 配置RPC服务启动失败: %v\n", err)
			os.Exit(1)
		}
		go func() {
			select {
			case err := <-hubRPC.Errors():
				fmt.Printf("⚠️  配置RPC服务异常退出: %v\n", err)
			case <-ctx.Done():
			}
		}()
	}

	fmt.Println()

	// 2. 监听系统信号，优雅退出
	fmt.Println("✅ 系统运行中，按 Ctrl+C 停止")
	fmt.Println()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n收到退出信号，正在关闭...")
	cancel()
	if hubRPC != nil {
		if err := hubRPC.Close(); err != nil {
			fmt.Printf("⚠️  配置RPC服务关闭失败: %v\n", err)
		}
	}
	time.Sleep(time.Second)
	fmt.Println("系统已停止")
}
