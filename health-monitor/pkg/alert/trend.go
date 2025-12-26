/* 针对微服务层指标根据历史序列判断：

连续上升

连续下降

重启次数增长趋势

业务校验失败率上升 */

package alert

import (
	"context"
	"fmt"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
	"time"
)

// TrendAnalyzer 趋势分析器
type TrendAnalyzer struct {
	stateManager *state.StateManager
	
	// 趋势判断参数
	trendWindowSize   int           // 趋势窗口大小（数据点数量）
	trendThreshold    float64       // 趋势判断阈值（变化率）
	continuousCount   int           // 连续上升/下降的最小次数
	lookbackDuration  time.Duration // 回溯时长
}

// NewTrendAnalyzer 创建趋势分析器
func NewTrendAnalyzer(sm *state.StateManager) *TrendAnalyzer {
	return &TrendAnalyzer{
		stateManager:     sm,
		trendWindowSize:  10,              // 分析最近10个数据点
		trendThreshold:   0.1,             // 10%变化率
		continuousCount:  3,               // 连续3次确认趋势
		lookbackDuration: 5 * time.Minute, // 回溯5分钟历史
	}
}

// AnalyzeNodeTrends 分析节点趋势
func (ta *TrendAnalyzer) AnalyzeNodeTrends(ctx context.Context, nodeID string) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// // 查询历史数据
	// history := ta.stateManager.QueryHistory(state.MetricTypeNode, nodeID, ta.lookbackDuration)
	// if len(history) < ta.trendWindowSize {
	// 	// 数据不足，无法进行趋势分析
	// 	return alerts
	// }
	
	// // 提取节点指标序列
	// nodeMetrics := make([]*model.NodeMetrics, 0, len(history))
	// timestamps := make([]int64, 0, len(history))
	
	// for _, entry := range history {
	// 	if nodeMetric, ok := entry.Data.(*model.NodeMetrics); ok {
	// 		nodeMetrics = append(nodeMetrics, nodeMetric)
	// 		timestamps = append(timestamps, entry.Timestamp)
	// 	}
	// }
	
	// // 分析 CPU 使用率趋势
	// cpuTrend := ta.analyzeCPUTrend(nodeMetrics)
	// if cpuTrend != nil {
	// 	alert := &model.AlertEvent{
	// 		AlertID:     fmt.Sprintf("TREND-NODE-CPU-%s-%d", nodeID, time.Now().Unix()),
	// 		Type:        "Node-CPU-Trend",
	// 		FaultCode:   "TREND_CPU_INCREASE",
	// 		Severity:    model.SeverityWarning,
	// 		Source:      fmt.Sprintf("node:%s", nodeID),
	// 		Message:     cpuTrend.Message,
	// 		MetricValue: cpuTrend.Value,
	// 		Timestamp:   time.Now().Unix(),
	// 		Metadata: map[string]interface{}{
	// 			"trend_type":  cpuTrend.Type,
	// 			"change_rate": cpuTrend.ChangeRate,
	// 			"prediction":  cpuTrend.Prediction,
	// 		},
	// 	}
	// 	alerts = append(alerts, alert)
	// }
	
	// // 分析内存使用率趋势
	// memoryTrend := ta.analyzeMemoryTrend(nodeMetrics)
	// if memoryTrend != nil {
	// 	alert := &model.AlertEvent{
	// 		AlertID:     fmt.Sprintf("TREND-NODE-MEM-%s-%d", nodeID, time.Now().Unix()),
	// 		Type:        "Node-Memory-Trend",
	// 		FaultCode:   "TREND_MEMORY_INCREASE",
	// 		Severity:    model.SeverityWarning,
	// 		Source:      fmt.Sprintf("node:%s", nodeID),
	// 		Message:     memoryTrend.Message,
	// 		MetricValue: memoryTrend.Value,
	// 		Timestamp:   time.Now().Unix(),
	// 		Metadata: map[string]interface{}{
	// 			"trend_type":  memoryTrend.Type,
	// 			"change_rate": memoryTrend.ChangeRate,
	// 			"prediction":  memoryTrend.Prediction,
	// 		},
	// 	}
	// 	alerts = append(alerts, alert)
	// }
	
	return alerts
}

// AnalyzeContainerTrends 分析容器趋势
func (ta *TrendAnalyzer) AnalyzeContainerTrends(ctx context.Context, containerID string) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// // 查询历史数据
	// history := ta.stateManager.QueryHistory(state.MetricTypeContainer, containerID, ta.lookbackDuration)
	// if len(history) < ta.trendWindowSize {
	// 	return alerts
	// }
	
	// // 提取容器指标序列
	// containerMetrics := make([]*model.ContainerMetrics, 0, len(history))
	
	// for _, entry := range history {
	// 	if containerMetric, ok := entry.Data.(*model.ContainerMetrics); ok {
	// 		containerMetrics = append(containerMetrics, containerMetric)
	// 	}
	// }
	
	// // 分析重启趋势
	// restartTrend := ta.analyzeRestartTrend(containerMetrics)
	// if restartTrend != nil {
	// 	alert := &model.AlertEvent{
	// 		AlertID:     fmt.Sprintf("TREND-CONTAINER-RESTART-%s-%d", containerID, time.Now().Unix()),
	// 		Type:        "Container-Restart-Trend",
	// 		FaultCode:   "TREND_RESTART_INCREASE",
	// 		Severity:    model.SeverityWarning,
	// 		Source:      fmt.Sprintf("container:%s", containerID),
	// 		Message:     restartTrend.Message,
	// 		MetricValue: restartTrend.Value,
	// 		Timestamp:   time.Now().Unix(),
	// 		Metadata: map[string]interface{}{
	// 			"trend_type":   restartTrend.Type,
	// 			"restart_rate": restartTrend.ChangeRate,
	// 		},
	// 	}
	// 	alerts = append(alerts, alert)
	// }
	
	return alerts
}

// AnalyzeServiceTrends 分析服务趋势
func (ta *TrendAnalyzer) AnalyzeServiceTrends(ctx context.Context, serviceID string) []*model.AlertEvent {
	var alerts []*model.AlertEvent
	
	// // 查询历史数据
	// history := ta.stateManager.QueryHistory(state.MetricTypeService, serviceID, ta.lookbackDuration)
	// if len(history) < ta.trendWindowSize {
	// 	return alerts
	// }
	
	// // 提取服务指标序列
	// serviceMetrics := make([]*model.ServiceMetrics, 0, len(history))
	
	// for _, entry := range history {
	// 	if serviceMetric, ok := entry.Data.(*model.ServiceMetrics); ok {
	// 		serviceMetrics = append(serviceMetrics, serviceMetric)
	// 	}
	// }
	
	// // 分析业务校验失败率趋势
	// validationTrend := ta.analyzeValidationTrend(serviceMetrics)
	// if validationTrend != nil {
	// 	alert := &model.AlertEvent{
	// 		AlertID:     fmt.Sprintf("TREND-SERVICE-VALIDATION-%s-%d", serviceID, time.Now().Unix()),
	// 		Type:        "Service-Validation-Trend",
	// 		FaultCode:   "TREND_VALIDATION_FAILURE",
	// 		Severity:    model.SeverityWarning,
	// 		Source:      fmt.Sprintf("service:%s", serviceID),
	// 		Message:     validationTrend.Message,
	// 		MetricValue: validationTrend.Value,
	// 		Timestamp:   time.Now().Unix(),
	// 		Metadata: map[string]interface{}{
	// 			"trend_type":   validationTrend.Type,
	// 			"failure_rate": validationTrend.ChangeRate,
	// 		},
	// 	}
	// 	alerts = append(alerts, alert)
	// }
	
	return alerts
}

// TrendResult 趋势分析结果
type TrendResult struct {
	Type       string  // "increasing", "decreasing", "stable"
	Message    string  // 描述信息
	Value      float64 // 当前值
	ChangeRate float64 // 变化率
	Prediction string  // 预测信息
}

// analyzeCPUTrend 分析 CPU 使用率趋势
func (ta *TrendAnalyzer) analyzeCPUTrend(metrics []*model.NodeMetrics) *TrendResult {
	if len(metrics) < ta.trendWindowSize {
		return nil
	}
	
	// 提取 CPU 使用率序列
	cpuValues := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		// CPU 使用率可能是 float64 或 string
		switch v := m.CPUUsage.(type) {
		case float64:
			cpuValues = append(cpuValues, v)
		case string:
			// 尝试解析字符串
			var cpu float64
			fmt.Sscanf(v, "%f", &cpu)
			cpuValues = append(cpuValues, cpu)
		}
	}
	
	if len(cpuValues) < ta.trendWindowSize {
		return nil
	}
	
	// 计算趋势
	trend := ta.calculateTrend(cpuValues)
	
	// 判断是否连续上升
	if trend.IsIncreasing && trend.ContinuousCount >= ta.continuousCount {
		currentValue := cpuValues[len(cpuValues)-1]
		
		// 预测未来值
		prediction := "正常"
		if currentValue > 70 && trend.ChangeRate > 0.1 {
			prediction = "可能在未来5分钟内达到80%"
		} else if currentValue > 80 {
			prediction = "可能在未来3分钟内达到90%"
		}
		
		return &TrendResult{
			Type:       "increasing",
			Message:    fmt.Sprintf("CPU使用率持续上升，当前%.1f%%，变化率%.1f%%", currentValue, trend.ChangeRate*100),
			Value:      currentValue,
			ChangeRate: trend.ChangeRate,
			Prediction: prediction,
		}
	}
	
	return nil
}

// analyzeMemoryTrend 分析内存使用率趋势
func (ta *TrendAnalyzer) analyzeMemoryTrend(metrics []*model.NodeMetrics) *TrendResult {
	if len(metrics) < ta.trendWindowSize {
		return nil
	}
	
	// 计算内存使用率序列
	memoryUsageValues := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		if m.MemoryTotal > 0 {
			usageRate := float64(m.MemoryTotal-m.MemoryFree) / float64(m.MemoryTotal) * 100
			memoryUsageValues = append(memoryUsageValues, usageRate)
		}
	}
	
	if len(memoryUsageValues) < ta.trendWindowSize {
		return nil
	}
	
	// 计算趋势
	trend := ta.calculateTrend(memoryUsageValues)
	
	// 判断是否连续上升
	if trend.IsIncreasing && trend.ContinuousCount >= ta.continuousCount {
		currentValue := memoryUsageValues[len(memoryUsageValues)-1]
		
		prediction := "正常"
		if currentValue > 70 && trend.ChangeRate > 0.1 {
			prediction = "可能在未来10分钟内达到85%"
		} else if currentValue > 85 {
			prediction = "可能在未来5分钟内触发OOM"
		}
		
		return &TrendResult{
			Type:       "increasing",
			Message:    fmt.Sprintf("内存使用率持续上升，当前%.1f%%，变化率%.1f%%", currentValue, trend.ChangeRate*100),
			Value:      currentValue,
			ChangeRate: trend.ChangeRate,
			Prediction: prediction,
		}
	}
	
	return nil
}

// analyzeRestartTrend 分析容器重启趋势
func (ta *TrendAnalyzer) analyzeRestartTrend(metrics []*model.ContainerMetrics) *TrendResult {
	if len(metrics) < ta.trendWindowSize {
		return nil
	}
	
	// 统计重启次数变化
	// 通过比较相邻时间点的 Uptime 来判断是否发生重启
	restartCount := 0
	for i := 1; i < len(metrics); i++ {
		// Uptime 减少表示发生了重启
		if metrics[i].Uptime < metrics[i-1].Uptime {
			restartCount++
		}
	}
	
	// 计算重启率（每分钟重启次数）
	timeSpan := float64(len(metrics)) // 假设每个数据点间隔约为 30 秒
	restartRate := float64(restartCount) / (timeSpan / 2.0) // 转换为每分钟
	
	// 如果重启率异常
	if restartCount >= 2 {
		return &TrendResult{
			Type:       "increasing",
			Message:    fmt.Sprintf("容器重启次数异常，最近%d分钟内重启%d次", int(ta.lookbackDuration.Minutes()), restartCount),
			Value:      float64(restartCount),
			ChangeRate: restartRate,
		}
	}
	
	return nil
}

// analyzeValidationTrend 分析业务校验失败率趋势
func (ta *TrendAnalyzer) analyzeValidationTrend(metrics []*model.ServiceMetrics) *TrendResult {
	if len(metrics) < ta.trendWindowSize {
		return nil
	}
	
	// 计算校验失败率序列
	failureRates := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		totalChecks := m.BusinessCheckSuccess + m.BusinessCheckFail
		if totalChecks > 0 {
			failureRate := float64(m.BusinessCheckFail) / float64(totalChecks) * 100
			failureRates = append(failureRates, failureRate)
		}
	}
	
	if len(failureRates) < ta.trendWindowSize {
		return nil
	}
	
	// 计算趋势
	trend := ta.calculateTrend(failureRates)
	
	// 判断是否连续上升
	if trend.IsIncreasing && trend.ContinuousCount >= ta.continuousCount {
		currentValue := failureRates[len(failureRates)-1]
		
		// 如果失败率超过阈值且持续上升
		if currentValue > 5.0 { // 失败率超过5%
			return &TrendResult{
				Type:       "increasing",
				Message:    fmt.Sprintf("业务校验失败率持续上升，当前%.1f%%，变化率%.1f%%", currentValue, trend.ChangeRate*100),
				Value:      currentValue,
				ChangeRate: trend.ChangeRate,
			}
		}
	}
	
	return nil
}

// TrendInfo 趋势信息
type TrendInfo struct {
	IsIncreasing    bool
	IsDecreasing    bool
	ContinuousCount int     // 连续上升/下降的次数
	ChangeRate      float64 // 平均变化率
}

// calculateTrend 计算数值序列的趋势
func (ta *TrendAnalyzer) calculateTrend(values []float64) *TrendInfo {
	if len(values) < 2 {
		return &TrendInfo{}
	}
	
	// 计算相邻点的变化方向
	increases := 0
	decreases := 0
	totalChange := 0.0
	
	for i := 1; i < len(values); i++ {
		diff := values[i] - values[i-1]
		if diff > 0 {
			increases++
			totalChange += diff / values[i-1] // 相对变化率
		} else if diff < 0 {
			decreases++
			totalChange += diff / values[i-1]
		}
	}
	
	// 判断主要趋势
	info := &TrendInfo{}
	
	if increases > len(values)/2 {
		info.IsIncreasing = true
		info.ContinuousCount = increases
		info.ChangeRate = totalChange / float64(increases)
	} else if decreases > len(values)/2 {
		info.IsDecreasing = true
		info.ContinuousCount = decreases
		info.ChangeRate = totalChange / float64(decreases)
	}
	
	return info
}