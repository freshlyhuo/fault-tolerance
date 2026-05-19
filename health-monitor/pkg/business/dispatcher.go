/*将业务指标 push 到 state manager,统一通路：StateManager.UpdateMetric()
 */
package business

import (
	"context"
	"fmt"
	"time"

	"health-monitor/pkg/alert"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

// Dispatcher 业务层分发器
type Dispatcher struct {
	generator    *alert.Generator
	stateManager *state.StateManager
}

// NewDispatcher 创建新的分发器
func NewDispatcher(stateManager *state.StateManager) *Dispatcher {
	return &Dispatcher{
		generator:    alert.NewGenerator(stateManager), // 使用带状态管理的生成器
		stateManager: stateManager,
	}
}

// SetDiagnosisReceiver 设置故障诊断接收器
func (d *Dispatcher) SetDiagnosisReceiver(receiver alert.DiagnosisReceiver) {
	d.generator.SetDiagnosisReceiver(receiver)
}

// HandleBusinessMetrics 处理业务层解析后的指标
func (d *Dispatcher) HandleBusinessMetrics(ctx context.Context, bm *model.BusinessMetrics) {
	// 1. 推送到 StateManager
	if d.stateManager != nil {
		businessMetric := &state.BusinessMetric{
			Data:      bm,
			Timestamp: time.Now().Unix(),
		}
		if err := d.stateManager.UpdateMetric(businessMetric); err != nil {
			fmt.Printf("[业务层Dispatcher] 保存到StateManager失败: %v\n", err)
		}
	}

	// 2. 发送到告警生成器进行阈值判断
	// Generator会调用threshold检查，生成告警事件并直接输出
	d.generator.ProcessBusinessMetrics(ctx, bm)

}
