/* 包括两个核心函数：
暂时不做
① 时间相关性

CheckTemporalCorrelation(events []Event)

判断是否在同一 30-60 秒窗口集中发生。

② 空间相关性（拓扑链路）不好做

CheckSpatialCorrelation(events, topo)

判断：

节点 → 容器 → 服务 → 业务
是否沿链路传播。 */

package alert

// TODO: 实现关联分析功能