package test

import (
	"testing"
	"time"

	"fault-diagnosis/pkg/config"
	"fault-diagnosis/pkg/engine"
	"fault-diagnosis/pkg/models"
	"go.uber.org/zap"
)

// TestBusinessLayerDiagnosis 测试业务层故障诊断
func TestBusinessLayerDiagnosis(t *testing.T) {
	logger := zap.NewNop()

	// 加载业务层故障树
	loader := config.NewLoader("../configs/fault_tree_business.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		t.Fatalf("加载故障树失败: %v", err)
	}

	// 创建诊断引擎
	diagnosisEngine, err := engine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		t.Fatalf("创建诊断引擎失败: %v", err)
	}

	var diagnosisResult *models.DiagnosisResult
	diagnosisEngine.SetCallback(func(diagnosis *models.DiagnosisResult) {
		diagnosisResult = diagnosis
	})

	// 测试场景1: 仅蓄电池电压异常（不应触发顶层事件）
	t.Run("仅蓄电池电压异常", func(t *testing.T) {
		diagnosisResult = nil
		alert := &models.AlertEvent{
			AlertID:   "BATTERY_VOLTAGE_ALERT",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert)
		
		if diagnosisResult != nil {
			t.Errorf("不应触发顶层事件，但触发了: %s", diagnosisResult.FaultCode)
		}
	})

	// 测试场景2: 蓄电池和母线电压异常，CPU板电压正常（应触发蓄电池异常）
	t.Run("蓄电池异常", func(t *testing.T) {
		diagnosisEngine.ResetAll()
		diagnosisResult = nil

		// 触发蓄电池电压异常
		alert1 := &models.AlertEvent{
			AlertID:   "BATTERY_VOLTAGE_ALERT",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert1)

		// 触发母线电压异常
		alert2 := &models.AlertEvent{
			AlertID:   "BUS_VOLTAGE_ALERT",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert2)

		if diagnosisResult == nil {
			t.Error("应该触发顶层事件，但没有触发")
			return
		}

		if diagnosisResult.FaultCode != "CJB-RG-ZD-3" {
			t.Errorf("期望故障码 CJB-RG-ZD-3, 得到 %s", diagnosisResult.FaultCode)
		}

		if len(diagnosisResult.BasicEvents) != 2 {
			t.Errorf("期望2个基本事件，得到 %d", len(diagnosisResult.BasicEvents))
		}
	})

	// 测试场景3: CPU板电压异常（应触发AD模块异常）
	t.Run("AD模块异常", func(t *testing.T) {
		diagnosisEngine.ResetAll()
		diagnosisResult = nil

		alert := &models.AlertEvent{
			AlertID:   "CPU_VOLTAGE_ALERT",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert)

		if diagnosisResult == nil {
			t.Error("应该触发顶层事件，但没有触发")
			return
		}

		if diagnosisResult.FaultCode != "CJB-RG-ZD-3" {
			t.Errorf("期望故障码 CJB-RG-ZD-3, 得到 %s", diagnosisResult.FaultCode)
		}
	})
}

// TestMicroserviceLayerDiagnosis 测试微服务层故障诊断
func TestMicroserviceLayerDiagnosis(t *testing.T) {
	logger := zap.NewNop()

	// 加载微服务层故障树
	loader := config.NewLoader("../configs/fault_tree_microservice.json")
	faultTree, err := loader.LoadFaultTree()
	if err != nil {
		t.Fatalf("加载故障树失败: %v", err)
	}

	// 创建诊断引擎
	diagnosisEngine, err := engine.NewDiagnosisEngine(faultTree, logger)
	if err != nil {
		t.Fatalf("创建诊断引擎失败: %v", err)
	}

	var diagnosisResults []*models.DiagnosisResult
	diagnosisEngine.SetCallback(func(diagnosis *models.DiagnosisResult) {
		diagnosisResults = append(diagnosisResults, diagnosis)
	})

	// 测试场景: 服务性能严重下降（P99延迟高 + 错误率高）
	t.Run("服务性能严重下降", func(t *testing.T) {
		diagnosisResults = nil

		// 触发P99延迟过高
		alert1 := &models.AlertEvent{
			AlertID:   "SERVICE_P99_LATENCY_HIGH",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert1)

		// 触发错误率过高
		alert2 := &models.AlertEvent{
			AlertID:   "SERVICE_ERROR_RATE_HIGH",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert2)

		if len(diagnosisResults) == 0 {
			t.Error("应该触发顶层事件，但没有触发")
			return
		}

		// 应该触发 TOP-MS-001
		found := false
		for _, result := range diagnosisResults {
			if result.FaultCode == "SVC-PERF-001" {
				found = true
				break
			}
		}

		if !found {
			t.Error("应该触发 SVC-PERF-001 故障")
		}
	})

	// 测试场景: 容器资源耗尽
	t.Run("容器资源耗尽", func(t *testing.T) {
		diagnosisEngine.ResetAll()
		diagnosisResults = nil

		// 仅触发CPU使用率过高
		alert := &models.AlertEvent{
			AlertID:   "CONTAINER_CPU_HIGH",
			Timestamp: time.Now().Unix(),
		}
		diagnosisEngine.ProcessAlert(alert)

		if len(diagnosisResults) == 0 {
			t.Error("应该触发顶层事件，但没有触发")
			return
		}

		found := false
		for _, result := range diagnosisResults {
			if result.FaultCode == "CONTAINER-RESOURCE-001" {
				found = true
				break
			}
		}

		if !found {
			t.Error("应该触发 CONTAINER-RESOURCE-001 故障")
		}
	})

	// 测试场景: 级联故障
	t.Run("服务级联故障", func(t *testing.T) {
		diagnosisEngine.ResetAll()
		diagnosisResults = nil

		// 触发所有相关告警
		alerts := []*models.AlertEvent{
			{AlertID: "SERVICE_P99_LATENCY_HIGH", Timestamp: time.Now().Unix()},
			{AlertID: "SERVICE_ERROR_RATE_HIGH", Timestamp: time.Now().Unix()},
			{AlertID: "CONTAINER_CPU_HIGH", Timestamp: time.Now().Unix()},
		}

		for _, alert := range alerts {
			diagnosisEngine.ProcessAlert(alert)
		}

		if len(diagnosisResults) == 0 {
			t.Error("应该触发顶层事件，但没有触发")
			return
		}

		// 应该触发多个故障
		if len(diagnosisResults) < 2 {
			t.Errorf("应该触发至少2个故障，但只触发了 %d 个", len(diagnosisResults))
		}

		// 检查是否触发级联故障
		found := false
		for _, result := range diagnosisResults {
			if result.FaultCode == "SVC-CASCADE-001" {
				found = true
				break
			}
		}

		if !found {
			t.Error("应该触发 SVC-CASCADE-001 级联故障")
		}
	})
}
