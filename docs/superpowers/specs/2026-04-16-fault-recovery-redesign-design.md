# Fault-Recovery 重构设计（接收 -> 映射 -> 引擎）

## 1. 文档目标

本文定义 fault-recovery 模块的破坏性升级方案，覆盖诊断结果接收、故障码归一化、JSON 映射中心、编排引擎与执行闭环。

本次设计的主验收轴为可靠性，同时提供最小可用可观测能力。

## 2. 背景与现状问题

现有实现可完成基础闭环，但在长期演进中存在以下问题：

1. 接收层与协议形态耦合，扩展不同接入方式成本高。
2. 归一化与映射边界不清晰，策略演进容易侵入引擎。
3. 引擎内混杂动作细节，可靠性策略难以统一治理。
4. 微服务故障路径包含熔断限流等非目标动作，不符合当前策略收敛方向。
5. 测试与可观测覆盖不足，故障定位与回归验证成本偏高。

## 3. 设计约束（已确认）

1. 接收层只接收故障诊断模块输出的诊断结果对象，不依赖 HTTP 等协议。
2. 归一化层核心职责是从诊断信息提取故障码并形成映射输入。
3. 映射中心采用 JSON 文件维护故障码与修复计划映射。
4. 业务层故障统一走修复容器管理：向修复容器传入指定修复计划。
5. 微服务层故障舍弃熔断限流，仅保留容器启停修复。
6. 允许破坏性升级。

## 4. 重构目标

1. 建立清晰单向流水线：Receive -> Normalize -> PlanMatch -> Orchestrate -> Execute -> Verify -> Report。
2. 强化可靠性：同目标串行、去重、统一超时与重试、失败升级。
3. 统一计划模型：映射中心输出标准修复计划，执行侧按计划驱动。
4. 降低耦合：接收、映射、引擎、动作运行时各自独立。
5. 提供最小可观测：阶段日志、核心指标、trace 串联。

## 5. 总体架构

### 5.1 分层

1. Receive 接收层
- 输入诊断结果对象。
- 补齐 trace_id 与接收时间。
- 产出原始事件封装对象。

2. Normalize 归一化层
- 提取 fault_code、target_id、status、timestamp、trace_id。
- 执行最小校验。
- 产出统一内部事件 NormalizedEvent。

3. Mapping 映射中心
- 从 JSON 映射配置中匹配 fault_code。
- 产出标准修复计划 RecoveryPlan。
- 无命中或计划非法时给出标准错误码。

4. Orchestrator 编排引擎
- 按 target_id 串行调度。
- 执行去重、限队列、超时、重试退避。
- 推进阶段状态并调用执行器。

5. Action Runtime 动作运行时
- 业务层：将计划下发到统一修复容器。
- 微服务层：容器启停执行器。
- 动作运行时不持有全局调度状态。

6. Verify 验证层
- 统一执行后验证。
- 输出 SUCCESS、FAILED、TIMEOUT、ESCALATED 等终态依据。

7. Report 状态回报层
- 回写阶段与终态。
- 输出结构化日志与指标。

### 5.2 边界规则

1. 接收层不做映射决策。
2. 映射层不执行动作。
3. 引擎不解析原始诊断协议，只消费 NormalizedEvent 与 RecoveryPlan。
4. 状态迁移只能由编排层触发，执行器只返回结果。

## 6. 数据模型

### 6.1 NormalizedEvent（内部最小事件）

字段建议（需要再确认）：

1. trace_id: string
2. fault_code: string
3. target_id: string
4. status: string（FIRING 或 RESOLVED）
5. diagnosis_time: int64
6. metadata: map[string]any（可选扩展）

### 6.2 RecoveryPlan（映射输出）

字段建议（需要再确认）：

1. plan_id: string
2. fault_code: string
3. domain: string（business 或 microservice）
4. executor: string（repair_container 或 container_lifecycle）
5. params: object（执行参数）
6. timeout_ms: int
7. max_retries: int
8. retry_backoff: object（initial_ms、max_ms、jitter）
9. escalation: object（channel、severity、owner）

### 6.3 ExecutionResult（执行结果）

字段建议（需要再确认）：

1. trace_id: string
2. plan_id: string
3. target_id: string
4. stage: string
5. status: string
6. error_code: string
7. message: string
8. started_at: int64
9. finished_at: int64

## 7. 映射中心设计

### 7.1 JSON 配置结构

使用单一映射文件维护 fault_code -> plan：

- 顶层包含 version、updated_at、plans。
- plans 为 fault_code 到 RecoveryPlan 的映射。
- 第一阶段不支持 default_plan，必须 fault_code 精确配置计划。

### 7.2 加载策略

1. 启动强校验：文件不存在、结构非法、关键字段缺失时启动失败。
2. 运行期策略：第一阶段采用静态加载，进程启动后不再读取文件变更。
3. 未来增强：后续阶段再引入热重载与快照回退机制。

### 7.3 匹配策略

1. 精确 fault_code 命中优先。
2. 未命中返回 PLAN_NOT_FOUND，并终态 NO_PLAN。
3. 计划字段非法返回 PLAN_INVALID，并终态 FAILED。

## 8. 编排引擎设计

### 8.1 调度模型

1. 全局输入队列接收 NormalizedEvent。
2. 以 target_id 为键进行串行化执行。
3. 同目标同一时刻只允许一个 RUNNING 任务。
4. 队列饱和时返回 TARGET_BUSY 或 QUEUE_FULL 对应拒绝结果。

### 8.2 幂等与去重

1. 幂等键建议：hash(trace_id, fault_code, target_id, status)。
2. 去重窗口内命中相同幂等键则拒绝重复执行。
3. 重复请求写 REJECTED 并保留原任务 trace 关联。

### 8.3 超时与重试

1. 超时由 RecoveryPlan.timeout_ms 控制，统一由引擎 context cancel。
2. 仅可重试错误进入重试路径。
3. 采用指数退避加随机抖动。
4. 超过 max_retries 进入 RETRY_EXHAUSTED 并升级。

### 8.4 两条执行路径

1. 业务层
- executor=repair_container。
- 引擎向统一修复容器提交计划参数。
- 修复容器内部执行具体动作并返回执行结果。

2. 微服务层
- executor=container_lifecycle。
- 仅执行容器启停动作。
- 不再包含熔断限流执行路径。

## 9. 状态机与终态

### 9.1 阶段状态

1. RECEIVED
2. NORMALIZED
3. PLANNED
4. DISPATCHED
5. RUNNING
6. VERIFIED
7. TERMINAL

### 9.2 终态分类

1. SUCCESS
2. FAILED
3. TIMEOUT
4. REJECTED
5. NO_PLAN
6. ESCALATED

### 9.3 错误码映射

1. INPUT_INVALID -> REJECTED
2. PLAN_NOT_FOUND -> NO_PLAN
3. PLAN_INVALID -> FAILED
4. TARGET_BUSY -> REJECTED
5. EXEC_TIMEOUT -> TIMEOUT
6. EXEC_FAILED -> FAILED
7. VERIFY_FAILED -> ESCALATED
8. RETRY_EXHAUSTED -> ESCALATED

## 10. 降级与升级

### 10.1 降级

1. 第一阶段映射为本地静态配置：启动成功后使用内存中的已加载映射，不进行运行期切换。
2. 某执行器不可用时，仅拒绝对应计划，不阻塞全局。
3. 状态回写失败时记录补偿日志，异步补写。
4. 验证依赖不可用时标记 VERIFY_FAILED，不假定成功。

### 10.2 升级

1. 重试耗尽。
2. 验证失败。
3. 关键依赖持续不可用。

升级记录最小字段：trace_id、fault_code、target_id、plan_id、failed_stage、error_code。

## 11. 最小可观测方案

### 11.1 结构化日志字段

1. trace_id
2. fault_code
3. target_id
4. plan_id
5. stage
6. status
7. error_code
8. duration_ms

### 11.2 最小指标

1. recovery_events_total{status}
2. recovery_stage_failures_total{stage,error_code}
3. recovery_retry_total{plan_id}
4. recovery_execute_duration_ms{plan_id}
5. recovery_queue_depth

### 11.3 追踪要求

同一 trace_id 可串联接收、映射、执行、验证、回写全过程。

## 12. 测试策略

### 12.1 单元测试

1. 归一化字段提取与输入校验。
2. JSON 映射加载、命中、未命中、非法计划。
3. 编排层串行、去重、超时、重试、终态映射。
4. 业务层执行器与微服务层执行器错误分层。

### 12.2 组件测试

1. 假执行器模拟 success、failed、timeout。
2. 并发同目标事件验证单运行约束。
3. 映射文件异常与回退行为验证。

### 12.3 集成测试

1. 业务层完整闭环。
2. 微服务层完整闭环。
3. 重试耗尽升级闭环。

## 13. 验收标准（第一阶段）

1. 可靠性
- 同目标并发冲突场景保持单任务执行。
- 超时、重试与计划配置一致。

2. 行为
- 业务层仅通过修复容器计划执行。
- 微服务层仅容器启停，无熔断限流路径。

3. 可观测
- 任一失败可通过 trace_id 定位到失败阶段与错误码。

## 14. 破坏性变更说明

1. 移除 fault-recovery 对 HTTP 接收接口的依赖，改为内部输入接口。
2. 替换原故障码到动作直接绑定模式，改为 fault_code -> RecoveryPlan。
3. 微服务故障恢复路径移除熔断限流动作。
4. 业务层执行统一改为修复容器计划下发。

## 15. 范围外事项

1. 状态持久化后端替换（当前可维持内存态）。
2. 跨进程分布式调度与分片。
3. 多租户隔离策略。
4. 高级可视化运维面板。

## 16. 后续衔接

本设计经确认后，下一步进入实现计划文档，拆分为：

1. 模块骨架重组与接口定义。
2. 映射中心与计划模型落地。
3. 编排可靠性策略实现。
4. 两类执行器改造与回归测试。
5. 可观测与验收补齐。
