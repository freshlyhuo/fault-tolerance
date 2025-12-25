节点监测指标 
指标名称	监测参数	正常阈值 / 状态	故障判据	故障编号关联
节点在线状态	status（节点状态）	online	offline	MS-NO-FL-1
CPU 使用率	cpu.percent	≤ 75%	> 85% 持续 60s	MS-NO-FL-2
内存使用率	ram.percent	≤ 80%	> 90%	MS-NO-FL-3
磁盘使用率	rom.percent	≤ 80%	> 90%	MS-NO-FL-4
网络流量平衡	upNet[].value / downNet[].value	上下行流量差 < 30%	流量突降或偏差 > 70%	MS-NO-FL-5
容器运行比例	running/(running+stop)	≥ 0.9	< 0.8	MS-NO-FL-6
节点进程数	processCount	稳定 ±10%	异常突增（均值+3σ）	MS-NO-FL-6
节点在线状态	status（节点状态）	online	offline	MS-NO-FL-1
CPU 使用率	cpu.percent	≤ 75%	> 85% 持续 60s	MS-NO-FL-2
内存使用率	ram.percent	≤ 80%	> 90%	MS-NO-FL-3
磁盘使用率	rom.percent	≤ 80%	> 90%	MS-NO-FL-4
网络流量平衡	upNet[].value / downNet[].value	上下行流量差 < 30%	流量突降或偏差 > 70%	MS-NO-FL-5
容器运行比例	running/(running+stop)	≥ 0.9	< 0.8	MS-NO-FL-6
节点进程数	processCount	稳定 ±10%	异常突增（均值+3σ）	MS-NO-FL-6


（二）容器监测指标 
指标名称	监测参数	正常阈值 / 状态	故障判据	故障编号关联
容器部署状态	deployStatus（部署状态）	success	failure / 部署超时	MS-CN-FL-1
容器启动状态	status（容器状态）	running	created / paused / exited	MS-CN-FL-2
容器运行中断	uptime（运行时长）	≥ 300 s	< 60 s	MS-CN-FL-3
容器重启次数	restartCnt（重启次数）	≤ 3 / day	> 5 / hour	MS-CN-FL-4
容器 CPU 使用率	cpuUsage.total	≤ 80%	> 90% 持续 60s	MS-CN-FL-5
容器内存使用率	memoryUsage / memoryLimit	≤ 85%	> 90%	MS-CN-FL-5
容器磁盘占用率	sizeUsage / sizeLimit	≤ 80%	> 90% 或增长速率 >5%/h	MS-CN-FL-6
容器暂停状态	status=paused	无	paused 状态持续 > 10 min	MS-CN-FL-7
容器部署状态	deployStatus（部署状态）	success	failure / 部署超时	MS-CN-FL-1
容器启动状态	status（容器状态）	running	created / paused / exited	MS-CN-FL-2



（三）服务监测指标 
指标名称	监测参数	正常阈值 / 状态	故障判据	故障编号关联
服务健康状态	health（服务健康）	TRUE	FALSE	MS-SV-FL-1
服务节点状态	children[].status	online	offline	MS-SV-FL-2
节点下容器状态	children[].children[].status	running	stopped / paused	MS-SV-FL-3
容器运行比例	children[].children[].status	≥ 0.9 运行中	< 0.8 运行中	MS-SV-FL-4
服务节点数量	children[].length	≥ 1	0	MS-SV-FL-5
主容器状态	children[].children[].self 与 status	self=true 且 running	self=true 且 非 running	MS-SV-FL-6
服务健康状态	health（服务健康）	TRUE	FALSE	MS-SV-FL-1
服务节点状态	children[].status	online	offline	MS-SV-FL-2
节点下容器状态	children[].children[].status	running	stopped / paused	MS-SV-FL-3
容器运行比例	children[].children[].status	≥ 0.9 运行中	< 0.8 运行中	MS-SV-FL-4
服务节点数量	children[].length	≥ 1	0	MS-SV-FL-5
主容器状态	children[].children[].self 与 status	self=true 且 running	self=true 且 非 running	MS-SV-FL-6