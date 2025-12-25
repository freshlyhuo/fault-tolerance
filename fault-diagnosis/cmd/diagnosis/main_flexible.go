package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
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
	receiverType   = flag.String("receiver", "channel", "接收器类型 (channel/udp/etcd)")
	
	// Channel接收器参数
	channelBuffer  = flag.Int("channel-buffer", 100, "Channel缓冲大小")
	
	// UDP接收器参数
	udpAddress     = flag.String("udp-addr", ":9999", "UDP监听地址")
	
	// etcd接收器参数
	etcdEndpoints  = flag.String("etcd", "localhost:2379", "etcd集群地址（逗号分隔）")
	watchPrefix    = flag.String("prefix", "/alerts/", "监听的etcd键前缀")
	
	// 通用参数
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
		zap.String("receiver_type", *receiverType),
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

	// 根据类型创建接收器
	var alertReceiver receiver.Receiver
	
	switch *receiverType {
	case "channel":
		logger.Info("使用Channel接收器（内存队列）", zap.Int("buffer_size", *channelBuffer))
		alertReceiver = receiver.NewChannelReceiver(*channelBuffer, logger)
		
	case "udp":
		logger.Info("使用UDP接收器", zap.String("address", *udpAddress))
		alertReceiver = receiver.NewUDPReceiver(*udpAddress, logger)
		
	case "etcd":
		logger.Info("使用etcd接收器",
			zap.String("endpoints", *etcdEndpoints),
			zap.String("prefix", *watchPrefix),
		)
		endpoints := []string{*etcdEndpoints}
		etcdReceiver, err := receiver.NewEtcdReceiver(endpoints, *watchPrefix, logger)
		if err != nil {
			logger.Fatal("创建etcd接收器失败", zap.Error(err))
		}
		alertReceiver = etcdReceiver
		
	default:
		logger.Fatal("不支持的接收器类型", zap.String("type", *receiverType))
	}

	// 设置告警处理函数
	alertReceiver.SetHandler(func(alert *models.AlertEvent) {
		diagnosisEngine.ProcessAlert(alert)
	})

	// 启动接收器
	if err := alertReceiver.Start(); err != nil {
		logger.Fatal("启动接收器失败", zap.Error(err))
	}
	defer alertReceiver.Stop()

	logger.Info("故障诊断模块已启动，等待告警事件...",
		zap.String("receiver_type", *receiverType),
	)

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
		zap.Time("诊断时间", diagnosis.Timestamp),
		zap.Strings("触发路径", diagnosis.TriggerPath),
		zap.Strings("基本事件", diagnosis.BasicEvents),
	)

	// 如果指定了输出路径，将诊断结果写入文件
	if *outputPath != "" {
		// writeToFile 逻辑保持不变
	}
}
