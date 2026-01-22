package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/engine"
	"fault-diagnosis/pkg/models"
	"fault-diagnosis/pkg/receiver"
	"fault-diagnosis/pkg/utils"
	"go.uber.org/zap"
)

var (
	configPath     = flag.String("config", "./configs/fault_tree_business.json", "故障树配置文件路径")
	etcdEndpoints  = flag.String("etcd", "localhost:2379", "etcd集群地址（逗号分隔）")
	watchPrefix    = flag.String("prefix", "/alerts/", "监听的etcd键前缀")
	logLevel       = flag.String("log-level", "info", "日志级别 (debug/info/warn/error)")
	outputPath     = flag.String("output", "", "诊断结果输出路径（为空则输出到stdout）")
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

	logger.Info("故障诊断模块启动",
		zap.String("config", *configPath),
		zap.String("etcd", *etcdEndpoints),
		zap.String("watch_prefix", *watchPrefix),
		zap.String("log_level", *logLevel),
	)

	// 加载故障树配置
	loader := config.NewLoader(*configPath)
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		logger.Fatal("加载故障树配置失败", zap.Error(err))
	}

	logger.Info("故障树配置加载成功",
		zap.String("fault_tree_id", faultTree.FaultTreeID),
		zap.Int("top_events", len(faultTree.TopEvents)),
		zap.Int("intermediate_events", len(faultTree.IntermediateEvents)),
		zap.Int("basic_events", len(faultTree.BasicEvents)),
	)

	// 创建诊断引擎
	diagnosisEngine, err := engine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		logger.Fatal("创建诊断引擎失败", zap.Error(err))
	}

	// 设置诊断回调函数
	diagnosisEngine.SetCallback(func(diagnosis *models.DiagnosisResult) {
		handleDiagnosisResult(diagnosis, logger)
	})

	// 创建告警接收器
	endpoints := strings.Split(*etcdEndpoints, ",")
	alertReceiver, err := receiver.NewAlertReceiver(endpoints, *watchPrefix, logger)
	if err != nil {
		logger.Fatal("创建告警接收器失败", zap.Error(err))
	}
	defer alertReceiver.Stop()

	// 设置告警处理函数
	alertReceiver.SetHandler(func(alert *models.AlertEvent) {
		diagnosisEngine.ProcessAlert(alert)
	})

	// 启动接收器
	if err := alertReceiver.Start(); err != nil {
		logger.Fatal("启动告警接收器失败", zap.Error(err))
	}

	logger.Info("故障诊断模块已启动，等待告警事件...")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("收到退出信号，正在关闭...")
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
	// 可以通过etcd、消息队列或HTTP API发送
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

	logger.Info("诊断结果已写入文件", zap.String("path", *outputPath))
}
