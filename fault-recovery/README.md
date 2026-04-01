# 故障修复模块 (Fault Recovery Module)

## 模块概述

故障修复模块负责接收故障诊断结果，并异步执行对应修复动作，形成“诊断 -> 修复 -> 验证 -> 状态回报”的闭环。

## 主要函数说明

以下为 fault-recovery 模块主链路中的核心函数说明，统一包含：函数名、URL（无则写无）、功能、工作流程（简版）、功能解释。



| 函数名 | URL | 功能 | 工作流程（简版） | 功能解释 |
|------|-----|------|------------------|----------|
| NewEngine | 无 | 创建修复执行引擎 | 初始化动作映射、队列容量、超时时间 | 构建异步修复执行核心。 |
| RegisterAction | 无 | 注册故障码精确匹配动作 | faultCode -> Action 绑定到 actions 映射 | 实现“故障码 -> 修复动作”的主路由。 |
| RegisterPrefixAction | 无 | 注册前缀匹配动作 | 按注册顺序保存 prefixAction 列表 | 支持一类故障码共用动作。 |
| Start | 无 | 启动事件消费循环 | 监听上下文和事件队列 -> 每条事件进入 handleEvent | 驱动引擎持续处理修复请求。 |
| Submit | 无 | 提交诊断事件到队列 | 补齐时间戳 -> 非阻塞入队 -> 队列满返回错误 | 保护服务可用性，避免调用方被阻塞。 |
| handleEvent | 无 | 处理单条修复事件 | 提取 targetID/status -> 匹配动作 -> 状态加锁 -> 执行动作 -> 上报结果 | 修复流程核心编排函数。 |
| executeAction | 无 | 执行并验证动作 | resolved 事件走 Resolve -> Verify；firing 事件走 Execute -> Verify | 统一处理“触发修复”和“恢复回滚”两种路径。 |


| NewStartContainerAction | 无 | 创建拉起容器动作实例 | 读取环境变量 -> 加载服务配置 -> 初始化 fetcher/超时/重试参数 | 准备业务层“重启镜像容器”动作。 |
| StartContainerAction.Execute | 无 | 执行容器拉起与重试修复 | 选择配置预设 -> 构建创建服务请求 -> 创建服务 -> 等待容器退出 -> 检查故障是否恢复 -> 失败重试 | 业务层故障主修复动作，实现自动拉起和闭环验证。 |
| StartContainerAction.Resolve | 无 | 处理恢复事件并清理资源 | 根据 targetID 找 serviceID -> 调用 destroy -> 清理 store 标记 | 故障恢复后释放临时拉起的服务资源。 |
| callServiceCommand | 无 | 执行服务控制命令 | 组装 ids 负载 -> 调用服务命令接口 -> 校验返回状态 | 封装 destroy/stop 等服务控制调用。 |
| createService | 无 | 创建修复服务实例 | 发送创建请求 -> 解析响应 data.id -> 返回 serviceID | 拉起新容器的关键接口封装。 |
| fetchAvailableNodeNames | 无 | 获取可用节点列表 | 拉取节点列表 -> 优先在线节点 -> 返回节点名集合 | 为服务创建选择目标节点。 |
| isFaultResolved | 无 | 判断故障是否已恢复 | 先看事件元数据 -> 若配置状态查询接口则远程确认 -> 返回 resolved 状态 | 防止容器重启后故障仍在持续。 |

| NewInMemoryStateManager | 无 | 创建内存状态管理器 | 初始化 states/recovering/lastResult 三类映射 | 提供开发态状态管理实现。 |
| LockRecovering | 无 | 原子锁定恢复中状态 | 检查 recovering 标记 -> 未占用则置为 RECOVERING | 防止同一目标并发重复修复。 |
| UpdateState | 无 | 更新目标恢复状态 | 写入目标状态 -> 非 RECOVERING 时释放锁 | 推进目标状态机。 |
| ReportResult | 无 | 上报修复结果并驱动状态迁移 | 保存 lastResult -> 记录日志 -> SUCCESS/FAILED/TIMEOUT 映射目标状态 | 将动作执行结果标准化并反馈状态管理。 |
| DiagnosisStatus | 无 | 解析诊断事件状态 | 从 metadata.status 或 metadata.resolved 推断 FIRING/RESOLVED | 统一引擎对事件状态的理解。 |
| DiagnosisTargetID | 无 | 解析诊断目标 ID | 优先使用 Source -> 回退 metadata.source | 统一动作执行目标定位。 |

### 主要函数相关变量说明

以下变量为主要函数链路中的关键运行变量，仅做简要说明。

| 变量名 | 作用 |
|------|------|
| sm | 引擎依赖的状态管理器，用于加锁恢复目标、更新状态和上报结果。 |
| actions | 故障码到修复动作的精确映射表。 |
| prefixActions | 故障码前缀到修复动作的映射列表。 |
| queue | 诊断事件异步队列，承接提交流量并由引擎消费。 |
| timeout | 单个修复动作执行超时时间。 |
| targetID | 当前修复目标标识，来自诊断结果 source 或 metadata。 |
| action | 本次故障匹配到的修复动作实例。 |
| status | 当前诊断状态（FIRING 或 RESOLVED），决定走修复还是恢复分支。 |
| result | 单次修复执行结果对象，包含状态、耗时和错误信息。 |
| store | 动作运行时存储对象，保存熔断开关、服务ID、resolved 等本地状态。 |
| baseURL | 修复模块调用外部控制面 API 的基础地址。 |
| client | 发送 HTTP 请求的客户端对象。 |
| config | 容器拉起动作的服务创建配置（来自 recovery_service_config.json）。 |
| fetcher | 微服务数据采集器，用于查询节点/容器状态辅助修复判断。 |
| monitorInterval | 轮询容器状态的时间间隔。 |
| maxWait | 等待容器退出或状态收敛的最长时长。 |
| maxRetries | 容器拉起失败后的最大重试次数。 |
| states | 内存状态管理器中的目标状态表（HEALTHY/FAILED/RECOVERING）。 |
| recovering | 内存状态管理器中的恢复锁表，防止同一目标并发修复。 |
| lastResult | 内存状态管理器中每个目标的最近一次修复结果记录。 |

