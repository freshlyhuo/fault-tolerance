/* 统一派发 ECSM 指标到 StateManager和alert */
package microservice

import (
	"context"
	"fmt"
	"time"
	"health-monitor/pkg/alert"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

type Dispatcher struct {
	fetcher      *Fetcher
	extractor    *Extractor
	generator    *alert.Generator
	stateManager *state.StateManager
}

func NewDispatcher(fetcher *Fetcher, stateManager *state.StateManager) *Dispatcher {
	return &Dispatcher{
		fetcher:      fetcher,
		extractor:    NewExtractor(),
		generator:    alert.NewGeneratorWithStateManager(stateManager), // 使用带状态管理的生成器
		stateManager: stateManager,
	}
}

func (d *Dispatcher) RunOnce(ctx context.Context) (*model.MicroServiceMetricsSet, error) {
	raw, err := d.fetcher.GatherRawMetrics(ctx)
	if err != nil {
		return nil, err
	}
	
	// 提取指标
	metrics := d.extractor.Extract(raw)
	
	// 1. 存储到 StateManager
	if d.stateManager != nil {
		if err := d.saveToStateManager(metrics); err != nil {
			fmt.Printf("[Dispatcher] 保存到StateManager失败: %v\n", err)
		}
	}
	
	// 2. 发送到告警生成器进行阈值检查
	if d.generator != nil {
		d.generator.ProcessMicroserviceMetrics(ctx, metrics)
	}
	
	// TODO: 其他处理
	// 3. 发送到数据库
	// 4. 推送到可视化平台
	
	return metrics, nil
}

// saveToStateManager 保存指标到状态管理器
func (d *Dispatcher) saveToStateManager(metrics *model.MicroServiceMetricsSet) error {
	timestamp := time.Now().Unix()
	
	// 保存所有节点指标
	for _, nodeData := range metrics.NodeMetrics {
		nodeCopy := nodeData
		nodeMetric := &state.NodeMetric{
			Data:      &nodeCopy,
			Timestamp: timestamp,
		}
		if err := d.stateManager.UpdateMetric(nodeMetric); err != nil {
			return fmt.Errorf("更新节点指标失败: %w", err)
		}
	}
	
	// 保存所有容器指标
	for _, containerData := range metrics.ContainerMetrics {
		containerCopy := containerData
		containerMetric := &state.ContainerMetric{
			Data:      &containerCopy,
			Timestamp: timestamp,
		}
		if err := d.stateManager.UpdateMetric(containerMetric); err != nil {
			return fmt.Errorf("更新容器指标失败: %w", err)
		}
	}
	
	// 保存所有服务指标
	for _, serviceData := range metrics.ServiceMetrics {
		serviceCopy := serviceData
		serviceMetric := &state.ServiceMetric{
			Data:      &serviceCopy,
			Timestamp: timestamp,
		}
		if err := d.stateManager.UpdateMetric(serviceMetric); err != nil {
			return fmt.Errorf("更新服务指标失败: %w", err)
		}
	}
	
	fmt.Printf("[Dispatcher] 已保存到StateManager: %d nodes, %d containers, %d services\n",
		len(metrics.NodeMetrics), len(metrics.ContainerMetrics), len(metrics.ServiceMetrics))
	
	return nil
}
