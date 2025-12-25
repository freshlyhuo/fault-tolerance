package microservice

import (
	model "health-monitor/pkg/models"
)

// Extractor 用于从 RawMetrics 中提取结构化指标
type Extractor struct{}

// convertNodeNetInfo 将 microservice.NodeNetInfo 切片转换为 model.NodeNetInfo 切片
func convertNodeNetInfo(src []NodeNetInfo) []model.NodeNetInfo {
	if src == nil {
		return nil
	}
	result := make([]model.NodeNetInfo, len(src))
	for i, n := range src {
		result[i] = model.NodeNetInfo{
			NetworkName: n.NetworkName,
			UpNet:       n.UpNet,
			DownNet:     n.DownNet,
		}
	}
	return result
}

func NewExtractor() *Extractor {
	return &Extractor{}
}

// 主入口：将 fetcher 的 RawMetrics → MicroServiceMetricsSet
func (e *Extractor) Extract(raw *RawMetrics) *model.MicroServiceMetricsSet {
	return &model.MicroServiceMetricsSet{
		NodeMetrics:      e.ExtractNodeMetrics(raw.Nodes),
		ContainerMetrics: e.ExtractContainerMetrics(raw.Containers),
		ServiceMetrics:   e.ExtractServiceMetrics(raw.Services),
	}
}

////////////////////////////////////////////////////////////////////////////////
//                              Node Metrics
////////////////////////////////////////////////////////////////////////////////

func (e *Extractor) ExtractNodeMetrics(nodes []NodeStatus) []model.NodeMetrics {
	var out []model.NodeMetrics

	for _, n := range nodes {
		out = append(out, model.NodeMetrics{
			ID:                  n.ID,
			Status:              n.Status,
			MemoryTotal:         n.MemoryTotal,
			MemoryFree:          n.MemoryFree,
			DiskTotal:           n.DiskTotal,
			DiskFree:            n.DiskFree,
			CPUUsage:            n.CPUUsage,   // interface{} 结构保持原样
		ProcessCount:        n.ProcessCount,
		ContainerTotal:      n.ContainerTotal,
		ContainerRunning:    n.ContainerRunning,
		ContainerEcsmTotal:  n.ContainerEcsmTotal,
		ContainerEcsmRunning:n.ContainerEcsmRunning,
		Net:                 convertNodeNetInfo(n.Net),
	})
	}

	return out
}

////////////////////////////////////////////////////////////////////////////////
//                          Container Metrics
////////////////////////////////////////////////////////////////////////////////

func (e *Extractor) ExtractContainerMetrics(containers []ContainerInfo) []model.ContainerMetrics {
	var out []model.ContainerMetrics

	for _, c := range containers {
		out = append(out, model.ContainerMetrics{
			ID:             c.ID,
			Status:         c.Status,
			Uptime:         c.Uptime,
			StartedTime:    c.StartedTime,
			CreatedTime:    c.CreatedTime,
			TaskCreatedTime:c.TaskCreatedTime,
		DeployStatus:   c.DeployStatus,
		FailedMessage:  c.FailedMessage,
		RestartCount:   c.RestartCount,
		DeployNum:      c.DeployNum,
		CPUUsage:       model.CPUUsage{Total: c.CPUUsage.Total, Cores: c.CPUUsage.Cores},
		MemoryLimit:    c.MemoryLimit,
		MemoryUsage:    c.MemoryUsage,
			MemoryMaxUsage: c.MemoryMaxUsage,
			SizeUsage:      c.SizeUsage,
			SizeLimit:      c.SizeLimit,
		})
	}

	return out
}

////////////////////////////////////////////////////////////////////////////////
//                              Service Metrics
////////////////////////////////////////////////////////////////////////////////

func (e *Extractor) ExtractServiceMetrics(services []ServiceGet) []model.ServiceMetrics {
	var out []model.ServiceMetrics

	for _, s := range services {
		out = append(out, model.ServiceMetrics{
			ID:                   s.ID,
			Status:               s.Status,
			ContainerStatusGroup: s.ContainerStatusGroup,
			Healthy:              s.Healthy,
			Factor:               s.Factor,
			Policy:               s.Policy,
			InstanceOnline:       s.InstanceOnline,
			InstanceActive:       s.InstanceActive,
		})
	}

	return out
}
