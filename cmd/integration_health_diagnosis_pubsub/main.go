package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	diagnosisConfig "fault-diagnosis/pkg/config"
	diagnosisEngine "fault-diagnosis/pkg/engine"
	diagnosisModels "fault-diagnosis/pkg/models"
	diagnosisReceiver "fault-diagnosis/pkg/receiver"
	diagnosisUtils "fault-diagnosis/pkg/utils"
	"go.uber.org/zap"
	healthBusiness "health-monitor/pkg/business"
	healthPubSub "health-monitor/pkg/pubsub"
	healthState "health-monitor/pkg/state"
)

func main() {
	diagnosisConfigPath := flag.String("diagnosis-config", "./fault-diagnosis/configs/fault_trees_multi_template.json", "故障树配置文件路径")
	hardwarePubSubAddr := flag.String("hardware-pubsub-addr", healthPubSub.DefaultAddress, "硬件指标VSOA发布订阅服务地址")
	hardwarePubSubURL := flag.String("hardware-pubsub-url", healthPubSub.DefaultURL, "硬件指标VSOA发布订阅URL")
	hardwarePubSubPassword := flag.String("hardware-pubsub-password", "", "硬件指标VSOA发布订阅服务密码")
	receiverBuffer := flag.Int("receiver-buffer", 500, "故障诊断内存接收队列大小")
	exitAfterDiagnoses := flag.Int("exit-after-diagnoses", 1, "收到多少条诊断结果后自动退出，0表示不自动退出")
	timeout := flag.Duration("timeout", 30*time.Second, "测试超时时间，0表示不启用超时")
	logLevel := flag.String("log-level", "info", "诊断日志级别 (debug/info/warn/error)")
	flag.Parse()

	logger, err := diagnosisUtils.NewLogger(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建诊断日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *timeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, *timeout)
		defer timeoutCancel()
	}

	diagnosis, err := newDiagnosisEngine(*diagnosisConfigPath, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化故障诊断失败: %v\n", err)
		os.Exit(1)
	}

	var diagnosisCount int32
	diagnosis.SetCallback(func(result *diagnosisModels.DiagnosisResult) {
		count := atomic.AddInt32(&diagnosisCount, 1)
		printDiagnosis(result, int(count))
		if *exitAfterDiagnoses > 0 && int(count) >= *exitAfterDiagnoses {
			cancel()
		}
	})

	alertReceiver := diagnosisReceiver.NewChannelReceiver(*receiverBuffer, logger)
	alertReceiver.SetHandler(func(alert *diagnosisModels.AlertEvent) {
		fmt.Printf("[故障诊断] 收到告警: alert_id=%s status=%s source=%s\n", alert.AlertID, alert.Status, alert.Source)
		diagnosis.ProcessAlert(alert)
	})
	if err := alertReceiver.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动故障诊断告警接收器失败: %v\n", err)
		os.Exit(1)
	}
	defer alertReceiver.Stop()

	stateManager, err := healthState.NewStateManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化健康监测状态管理器失败: %v\n", err)
		os.Exit(1)
	}
	defer stateManager.Close()

	dispatcher := healthBusiness.NewDispatcher(stateManager)
	dispatcher.SetDiagnosisReceiver(diagnosisReceiver.NewReceiverWrapper(alertReceiver))

	pubsubClient := healthPubSub.NewClient(healthPubSub.Option{
		Address:  *hardwarePubSubAddr,
		URL:      *hardwarePubSubURL,
		Password: *hardwarePubSubPassword,
	}, dispatcher)
	if err := pubsubClient.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "启动健康监测PubSub客户端失败: %v\n", err)
		os.Exit(1)
	}
	defer pubsubClient.Close()

	fmt.Println("========== 健康监测 + 故障诊断集成测试 ==========")
	fmt.Printf("故障树配置: %s\n", *diagnosisConfigPath)
	fmt.Printf("硬件PubSub: %s %s\n", *hardwarePubSubAddr, *hardwarePubSubURL)
	fmt.Printf("自动退出诊断数: %d\n", *exitAfterDiagnoses)
	fmt.Println("等待硬件指标、告警和诊断结果...")
	fmt.Println("================================================")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "FAIL: 等待诊断结果超时，已收到 %d 条诊断\n", atomic.LoadInt32(&diagnosisCount))
			os.Exit(1)
		}
	case sig := <-sigCh:
		fmt.Printf("\n收到退出信号: %s\n", sig)
		cancel()
	}

	if *exitAfterDiagnoses > 0 && atomic.LoadInt32(&diagnosisCount) < int32(*exitAfterDiagnoses) {
		fmt.Fprintf(os.Stderr, "FAIL: 诊断结果数量不足，got=%d want=%d\n", atomic.LoadInt32(&diagnosisCount), *exitAfterDiagnoses)
		os.Exit(1)
	}
	fmt.Printf("PASS: 已输出 %d 条诊断结果\n", atomic.LoadInt32(&diagnosisCount))
}

func newDiagnosisEngine(configPath string, logger *zap.Logger) (*diagnosisEngine.MultiDiagnosisEngine, error) {
	loader := diagnosisConfig.NewLoader(configPath)
	faultTrees, err := loader.LoadFaultTrees()
	if err != nil {
		return nil, fmt.Errorf("加载故障树配置失败: %w", err)
	}

	eng, err := diagnosisEngine.NewMultiDiagnosisEngine(faultTrees, logger)
	if err != nil {
		return nil, fmt.Errorf("创建多故障树诊断引擎失败: %w", err)
	}
	return eng, nil
}

func printDiagnosis(diagnosis *diagnosisModels.DiagnosisResult, index int) {
	status := "FIRING"
	if diagnosis.Metadata != nil {
		if v, ok := diagnosis.Metadata["status"].(string); ok && v != "" {
			status = v
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Printf("[故障诊断结果 #%d] status=%s\n", index, status)
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("诊断ID:   %s\n", diagnosis.DiagnosisID)
	fmt.Printf("故障树ID: %s\n", diagnosis.FaultTreeID)
	fmt.Printf("顶层事件: %s (%s)\n", diagnosis.TopEventID, diagnosis.TopEventName)
	fmt.Printf("故障码:   %s\n", diagnosis.FaultCode)
	fmt.Printf("故障原因: %s\n", diagnosis.FaultReason)
	fmt.Printf("诊断源:   %s\n", diagnosis.Source)
	fmt.Printf("触发路径: %v\n", diagnosis.TriggerPath)
	fmt.Printf("基本事件: %v\n", diagnosis.BasicEvents)
	fmt.Println(strings.Repeat("=", 72))
}
