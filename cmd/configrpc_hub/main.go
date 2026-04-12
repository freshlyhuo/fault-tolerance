package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"fault-tolerance/pkg/confighub"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:3001", "统一配置RPC服务监听地址")
	flag.Parse()

	hub := confighub.NewConfigHubServer(*addr)
	if err := hub.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start config hub failed: %v\n", err)
		os.Exit(1)
	}
	defer hub.Close()

	fmt.Printf("config hub started on %s\n", *addr)
	fmt.Println("registered URLs:")
	fmt.Println("- /fault_tolerance/health_monitor/update_config_rpc")
	fmt.Println("- /fault_tolerance/health_monitor/get_status_rpc")
	fmt.Println("- /fault_tolerance/fault_diagnosis/update_config_rpc")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("shutdown signal received")
	case err := <-hub.Errors():
		fmt.Fprintf(os.Stderr, "config hub exited with error: %v\n", err)
		os.Exit(1)
	}
}
