//go:build !flexible
// +build !flexible

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/configrpc"
	"fault-diagnosis/pkg/models"
	"fault-diagnosis/pkg/receiver"
	"fault-diagnosis/pkg/utils"
	"go.uber.org/zap"
)

var (
	configPath      = flag.String("config", "./configs/fault_trees_multi_template.json", "故障树配置文件路径")
	enableConfigRPC = flag.Bool("enable-config-rpc", true, "是否启用VSOA配置RPC服务")
	configRPCAddr   = flag.String("config-rpc-addr", "127.0.0.1:3001", "VSOA配置RPC服务监听地址")
	logLevel        = flag.String("log-level", "info", "日志级别 (debug/info/warn/error)")
	outputPath      = flag.String("output", "", "诊断结果输出路径（为空则输出到stdout）")
)

func main() {
	flag.Parse()

	// 创建日志记录器
	logger, err := utils.NewLogger(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建日志记录器失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	config.SetFaultTreeConfigPath(*configPath)
	diagnosisEngine, err := newReloadableDiagnosisEngine(*configPath, logger, func(diagnosis *models.DiagnosisResult) {
		handleDiagnosisResult(diagnosis, logger)
	})
	if err != nil {
		logger.Fatal("初始化诊断引擎失败", zap.Error(err))
	}

	var configRPCServer *configrpc.VSOAServer
	if *enableConfigRPC {
		service := configrpc.NewRuntimeConfigServiceWithReload(func() error {
			return diagnosisEngine.Reload()
		})
		configRPCServer = configrpc.NewVSOAServer(*configRPCAddr, service)
		if err := configRPCServer.Start(); err != nil {
			logger.Fatal("启动配置RPC服务失败", zap.Error(err))
		}
		defer configRPCServer.Close()
		go func() {
			if err := <-configRPCServer.Errors(); err != nil {
				logger.Error("配置RPC服务异常退出", zap.Error(err))
			}
		}()
	}

	// 使用内存通道接收器（当前receiver包可用实现）
	alertReceiver := receiver.NewChannelReceiver(500, logger)
	defer alertReceiver.Stop()

	// 设置告警处理函数
	alertReceiver.SetHandler(func(alert *models.AlertEvent) {
		diagnosisEngine.ProcessAlert(alert)
	})

	// 启动接收器
	if err := alertReceiver.Start(); err != nil {
		logger.Fatal("启动告警接收器失败", zap.Error(err))
	}

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

// handleDiagnosisResult 处理诊断结果
func handleDiagnosisResult(diagnosis *models.DiagnosisResult, logger *zap.Logger) {
	logger.Info("===== 故障诊断报告 =====",
		zap.String("诊断ID", diagnosis.DiagnosisID),
		zap.String("故障树ID", diagnosis.FaultTreeID),
		zap.String("顶层事件", diagnosis.TopEventName),
		zap.String("故障码", diagnosis.FaultCode),
		zap.String("故障原因", diagnosis.FaultReason),
		zap.String("诊断源", diagnosis.Source),
		zap.Time("诊断时间", diagnosis.Timestamp),
		zap.Strings("触发路径", diagnosis.TriggerPath),
		zap.Strings("基本事件", diagnosis.BasicEvents),
	)

	// 如果指定了输出路径，将诊断结果写入文件
	if *outputPath != "" {
		writeToFile(diagnosis, logger)
	}

	// TODO: 将诊断结果发送到故障修复模块
	// 可以通过消息队列或HTTP API发送
}

// writeToFile 将诊断结果写入文件
func writeToFile(diagnosis *models.DiagnosisResult, logger *zap.Logger) {
	data, err := json.MarshalIndent(diagnosis, "", "  ")
	if err != nil {
		logger.Error("序列化诊断结果失败", zap.Error(err))
		return
	}

	// 追加写入文件
	f, err := os.OpenFile(*outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("打开输出文件失败", zap.Error(err))
		return
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		logger.Error("写入诊断结果失败", zap.Error(err))
		return
	}

	if _, err := f.WriteString("\n"); err != nil {
		logger.Error("写入换行符失败", zap.Error(err))
		return
	}

}
