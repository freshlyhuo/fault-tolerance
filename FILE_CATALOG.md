# 项目文件全量清单 (fault-tolerance)

> 智能化卫星容错系统 — **监测→诊断→修复** 闭环
>
> Go 1.25.0 · 目标平台 SylixOS ARM64 · 3 个子模块 (health-monitor / fault-diagnosis / fault-recovery)

---

## 目录

1. [项目概览](#1-项目概览)
2. [根目录文件](#2-根目录文件)
3. [cmd/ — 集成测试入口](#3-cmd--集成测试入口)
4. [health-monitor/ — 健康监测模块](#4-health-monitor--健康监测模块)
5. [fault-diagnosis/ — 故障诊断模块](#5-fault-diagnosis--故障诊断模块)
6. [fault-recovery/ — 故障修复模块](#6-fault-recovery--故障修复模块)
7. [build/ — 编译产物](#7-build--编译产物)
8. [.trunk/ — Linter 配置](#8-trunk--linter-配置)
9. [重复/相似文件分析](#9-重复相似文件分析)
10. [已知问题](#10-已知问题)

---

## 1. 项目概览

| 维度 | 说明 |
|------|------|
| 语言 | Go 1.25.0 |
| 工作区 | `go.work` 链接 `.` / `./fault-diagnosis` / `./health-monitor`（注意 fault-recovery 不在 go.work 里，通过 go.mod replace 引入） |
| 部署平台 | SylixOS ARM64（卫星嵌入式） |
| 外部依赖 | ECSM 容器管理平台 API · etcd (可选持久化) · zap 日志 · golang.org/x/crypto/ssh |
| 通信方式 | Go channel（进程内），HTTP（与 ECSM / fault-recovery 服务通信），SSH（SylixOS 防火墙命令） |

### 数据流

```
卫星遥测二进制报文 / ECSM API
        ↓
  health-monitor (解析+阈值检测+告警生成)
        ↓ (Go channel / AlertAdapter)
  fault-diagnosis (故障树评估 AND/OR/NOT)
        ↓ (callback / HTTP POST)
  fault-recovery  (熔断限流 / 启动镜像容器)
```

---

## 2. 根目录文件

| 文件 | 分类 | 说明 |
|------|------|------|
| `go.mod` | **配置** | 根模块 `fault-tolerance`，depend fault-diagnosis/health-monitor/fault-recovery (replace 本地路径) |
| `go.work` | **配置** | Go workspace：`.` / `./fault-diagnosis` / `./health-monitor` |
| `go.sum` / `go.work.sum` | 配置 | 依赖校验和 |
| `README.md` | **文档** | 项目简介（中文），描述监测-诊断-修复三步闭环 |
| `INTEGRATION_TEST.md` | **文档** | 集成测试文档：8 个测试场景（业务层+微服务层），含预期结果 |
| `MEMORY_INTEGRATION.md` | **文档** | 内存集成架构：AlertAdapter / ReceiverWrapper 对接模式 |
| `SYLIXOS_DEPLOY.md` | **文档** | SylixOS ARM64 部署指南：交叉编译、SSH 上传、nohup 运行 |
| `actuator.json` | **配置/示例** | 飞轮转速样例数据（X/Y/Z 轴，rpm） |
| `comm.json` | **配置/示例** | 通信链路样例数据（SNR、CAN、串口统计） |
| `power.json` | **配置/示例** | 电源样例数据（电压/电流，mV/mA） |
| `thermal.json` | **配置/示例** | 热控样例数据（10 路温度点、加热器开关） |
| `osv-scanner.toml` | **配置** | CVE 漏洞忽略列表（受 SylixOS SDK 版本限制，79 行） |
| `run_integration_simple.sh` | **脚本** | 编译+运行 `cmd/integration_simple`（纯模拟数据测试） |
| `run_integration_test.sh` | **脚本** | 编译+运行 `cmd/integration_test`（连接 ECSM 真实监测） |
| `scripts/flowctl_throttle.sh` | **脚本** | SylixOS flowctl 网络限流脚本（用于容器） |

---

## 3. cmd/ — 集成测试入口

> **关键结论**：`cmd/` 下全部 6 个入口均为 **集成测试/演示** 程序，不是生产运行入口。各子模块自身的 `cmd/` 才是独立服务入口。

| 文件 | 分类 | 说明 |
|------|------|------|
| `cmd/integration_simple/main.go` | **集成测试** | 纯 mock 数据，测试业务层（电源电压阈值）和微服务层（容器 CPU/内存）共 8 个场景。使用 `CheckPowerThresholdsWithState` / `CheckContainerThresholdsWithState`，不含 recovery 模块。 |
| `cmd/integration_memory/main.go` | **Demo** | 演示 health-monitor 与 fault-diagnosis 内存集成，通过 `ReceiverWrapper` 桥接。引用了已废弃的 `Engine.Diagnose()` API。 |
| `cmd/integration_test/main.go` | **全链路集成测试** | 业务+微服务双层，连接 ECSM（`192.168.31.127:3001`），含 fault-recovery，前台持续运行+信号处理。`runBusinessSimulation`（4 场景 10s 定时）+ `runMicroserviceMonitoring`（5s 轮询）。 |
| `cmd/integration_test_business/main.go` | **业务层集测+Recovery** | 注入电池+母线电压异常→1s 后恢复正常→验证自动修复。使用 `RegisterPrefixAction("YW", ...)`。 |
| `cmd/integration_test_business_no_recovery/main.go` | **业务层故障持续测试** | 与上面类似但 **不发送恢复数据**，测试 recovery 引擎超时行为。 |
| `cmd/integration_test_microservice/main.go` | **微服务层集测+Recovery** | 连接 ECSM 真实监测 + recovery 引擎。为 `CONTAINER-RESOURCE-001` 注册 `CircuitBreakerAction`。 |

---

## 4. health-monitor/ — 健康监测模块

### 4.1 模块根

| 文件 | 分类 | 说明 |
|------|------|------|
| `go.mod` | 配置 | 模块 `health-monitor`，依赖 etcd client v3 |
| `build.sh` | 脚本 | SylixOS ARM64 交叉编译脚本 |
| `test-build.sh` | 脚本 | 测试构建脚本 |
| `design.md` | 文档 | 模块设计文档 |
| `README.md` | 文档 | 模块说明 |
| `README_DEPLOY.md` | 文档 | 部署说明 |
| `INTEGRATION.md` | 文档 | 集成说明 |
| `TREND_SUMMARY.md` | 文档 | 趋势分析功能总结 |
| `build/health-monitor-sylixos` | 构建产物 | 编译后的 SylixOS 二进制 |

### 4.2 cmd/

| 文件 | 分类 | 说明 |
|------|------|------|
| `cmd/monitor/main.go` | **核心入口** | **模块唯一正式入口**。CLI 参数 `--ecsm-url` / `--etcd` / `--interval` / `--test-business`。初始化业务层 Receiver（二进制报文解析）和微服务层 Fetcher/Dispatcher。内含业务测试循环。 |
| `cmd/integration_demo/main.go` | Demo | 全系统集成演示：StateManager + 业务报文模拟 + 节点指标 + 趋势分析 |
| `cmd/trend_demo/main.go` | Demo | 趋势分析专项演示：CPU/内存/重启/校验趋势 |

### 4.3 pkg/alert/ — 告警生成

| 文件 | 分类 | 说明 |
|------|------|------|
| `adapter.go` | **核心** | `DiagnosisReceiver` 接口 + `AlertAdapter`：将告警转换为诊断模块可用格式（map / struct 双路径） |
| `generator.go` | **核心** | `Generator`：处理业务/微服务指标 → 调用阈值检测 → `outputAlerts`（去重+严重度分类）。282 行 |
| `threshold.go` | **核心** | **无状态**阈值检测：`CheckPowerThresholds` / `CheckThermalThresholds` / `CheckCommThresholds` / `CheckActuatorThresholds` / `CheckNodeThresholds` / `CheckContainerThresholds` / `CheckServiceThresholds`。430 行 |
| `threshold_stateful.go` | **核心** | **有状态**阈值检测（通过 StateManager.CheckAndUpdateAlertState）：电源(电池/CPU/母线电压)、容器(CPU/内存/磁盘)。节点检测代码大部分被注释。445 行 |
| `trend.go` | **核心（半成品）** | `TrendAnalyzer`：节点/容器/服务趋势分析框架。大部分实现被注释掉。418 行 |
| `correlate.go` | **占位** | TODO：时间/空间相关性分析 |
| `debounce.go` | **占位** | TODO：告警防抖 |
| `TREND_ANALYSIS.md` | 文档 | 趋势分析设计文档 |
| `TREND_QUICK_REF.md` | 文档 | 趋势分析快速参考 |

### 4.4 pkg/business/ — 业务层报文处理

| 文件 | 分类 | 说明 |
|------|------|------|
| `receiver.go` | **核心** | 二进制报文接收器：`Submit()` / `ParsePacket()`，支持 14 种组件类型（byte 0=type, bytes 1-2=长度, bytes 3-n=payload）。412 行 |
| `dispatcher.go` | **核心** | 将解析后的 `BusinessMetrics` 分发到 StateManager 和 Generator 进行阈值检测 |
| `integration_test.go` | 测试 | 调度器→生成器流程测试（**有编译问题**：`NewDispatcher()` 缺少 StateManager 参数） |
| `ALERT_FLOW.md` | 文档 | 告警流程说明 |
| `BUSINESS_PACKET_PUBSUB_SPEC.doc` | 文档 | 业务报文 PubSub 规范（二进制格式） |
| `BUSINESS_PACKET_PUBSUB_SPEC.md` | 文档 | 同上 Markdown 版本 |
| `metrics.md` | 文档 | 指标说明 |
| `metrics_structure.md` | 文档 | 指标结构说明 |
| `packet_format.md` | 文档 | 报文格式说明 |

### 4.5 pkg/microservice/ — 微服务层监测

| 文件 | 分类 | 说明 |
|------|------|------|
| `fetcher.go` | **核心** | ECSM API HTTP 客户端：`ListNode` / `ListContainerByNode` / `ListContainerByService` / `ListService`，支持分页。623 行 |
| `disoatcher.go` | **核心（文件名拼写错误）** | Dispatcher：调用 fetcher.GatherRawMetrics → extractor.Extract → 存入 StateManager + generator |
| `extractor.go` | **核心** | 将 `RawMetrics` 转换为 `MicroServiceMetricsSet` |
| `container_types.go` | **核心** | ContainerInfo / CPUUsage / ContainerList / 查询选项结构体 |
| `node_types.go` | **核心** | NodeInfo / NodeStatus / NodeList / NodeCPUUsage / NodeNetInfo 结构体 |
| `service_types.go` | **核心** | Service 创建/更新类型：`EcsImageConfig` / `ImageSpec` / `NodeSpec` / `Resources` 等。248 行 |
| `topology.go` | **占位** | TODO：拓扑分析 |
| `integration_test.go` | 测试 | **有编译问题**：使用错误的导入路径（`"alert"` / `"model"` 非完整路径） |
| `fetcher` | **杂项** | 无扩展名文件（可能是空文件或误创建的产物） |
| `ALERT_INTEGRATION.md` | 文档 | 告警集成说明 |
| `metrics.md` | 文档 | 指标说明 |

### 4.6 pkg/models/ — 数据模型

| 文件 | 分类 | 说明 |
|------|------|------|
| `alert.go` | **核心** | `AlertEvent` + `AlertSeverity`(info/warning/critical) + `AlertStatus`(firing/resolved) |
| `metrics.go` | **核心** | 所有指标结构体：`MicroServiceMetricsSet`, `NodeMetrics`, `ContainerMetrics`, `ServiceMetrics`, `BusinessMetrics`, `PowerMetrics`, `ThermalMetrics`, `CommMetrics`, `ActuatorMetrics`, `TransceiverMetrics` 等。~300 行 |
| `topology.go` | **占位** | TODO |

### 4.7 pkg/config/

| 文件 | 分类 | 说明 |
|------|------|------|
| `config.go` | **配置桩** | 组件类型常量（`CompRunMgr` 到 `CompEPS`，0x01–0x0E），含注释掉的配置设计笔记 |

### 4.8 pkg/state/ — 状态管理

| 文件 | 分类 | 说明 |
|------|------|------|
| `state_manager.go` | **核心** | `StateManager`：实时状态维护(`UpdateMetric`)、统一查询(`GetLatestState`)、历史窗口缓存(RingBuffer 600条/10min)、时间戳对齐、etcd 快照持久化(`SaveSnapshot`/`LoadSnapshot`)、告警状态跟踪(`CheckAndUpdateAlertState`)。584 行 |
| `types.go` | **核心** | `Metric` 接口 + 4 种包装类型：`NodeMetric`,`ContainerMetric`,`ServiceMetric`,`BusinessMetric` + `StateSnapshot` |
| `storage.go` | **核心（桩）** | 仅包头注释"最近 N 分钟关键指标持久化，重启后恢复状态"，无实际实现 |
| `recovery_state.go` | **核心** | `RecoveryStateManager` 接口 + `InMemoryRecoveryStateManager`：`LockRecovering`/`UpdateState`/`ReportResult`。109 行 |
| `state_manager_test.go` | 测试 | 单元测试+基准测试（**有编译问题**：使用 `"model"` 短路径而非完整模块路径） |
| `example/main.go` | Demo | 状态管理器使用示例（标注 UNUSED）。使用短导入路径 `"model"` / `"state"`，无法直接编译。 |
| `USAGE.md` | 文档 | 状态管理器使用指南（Ring Buffer + BoltDB 设计说明，418 行） |
| `function.md` | 文档 | 状态管理器功能规格（输入/输出/核心函数清单） |

### 4.9 pkg/utils/

| 文件 | 分类 | 说明 |
|------|------|------|
| `logger.go` | **占位** | 标注 UNUSED。日志封装设计草稿，无实际代码 |

---

## 5. fault-diagnosis/ — 故障诊断模块

### 5.1 模块根

| 文件 | 分类 | 说明 |
|------|------|------|
| `go.mod` | 配置 | 模块 `fault-diagnosis`，依赖 zap |
| `.gitignore` | 配置 | 标准 Go gitignore |
| `build.sh` | 脚本 | SylixOS ARM64 交叉编译 |
| `Makefile` | 脚本 | 构建 diagnosis + demo 二进制 |
| `test.sh` | 脚本 | 运行测试 |
| `run_demo.sh` | 脚本 | 运行 demo |
| `Dockerfile` | 部署 | Docker 镜像构建 |
| `docker-compose.yml` | 部署 | Docker Compose 含 etcd 服务 |
| `design.md` | 文档 | 设计文档 |
| `README.md` | 文档 | 模块说明 |
| `ARCHITECTURE.md` | 文档 | 架构说明 |
| `DEPLOYMENT.md` | 文档 | 部署说明 |
| `INTEGRATION.md` | 文档 | 集成说明 |
| `PROJECT_OVERVIEW.md` | 文档 | 项目概览 |
| `QUICKSTART.md` | 文档 | 快速入门 |
| `RECEIVER_GUIDE.md` | 文档 | 接收器使用指南 |
| `STATE_MANAGEMENT.md` | 文档 | 状态管理说明 |
| `build/fault-diagnosis-sylixos` | 构建产物 | SylixOS 二进制 |

### 5.2 cmd/

| 文件 | 分类 | 说明 |
|------|------|------|
| `cmd/diagnosis/main.go` | **核心入口** | 独立运行入口，使用 etcd `AlertReceiver` 监听告警。CLI：`--config` / `--etcd` / `--prefix` / `--log-level` / `--output` |
| `cmd/diagnosis/main_flexible.go` | **核心入口** | 灵活版入口，支持 `--receiver` 切换 channel/udp/etcd 类型。引用了 `NewUDPReceiver`（可能未实现） |
| `cmd/demo/main.go` | Demo | 交互式菜单演示（1=业务 2=微服务 3=全部）。手动注入告警，测试 4 个业务场景 + 微服务场景 |

### 5.3 pkg/config/

| 文件 | 分类 | 说明 |
|------|------|------|
| `loader.go` | **核心** | JSON 故障树配置加载器，含校验逻辑 |

### 5.4 pkg/engine/

| 文件 | 分类 | 说明 |
|------|------|------|
| `engine.go` | **核心** | `DiagnosisEngine`：`ProcessAlert()`, `SetCallback()`, `buildTree()`, `diagnose()`。处理 firing/resolved 告警，NOT 门逻辑，顶事件上下文跟踪(source, serviceID, serviceName)。419 行 |
| `evaluator.go` | **核心** | 故障树求值器：`EvaluateNode()`, `evaluateAND/OR/NOT/BASIC`，`CollectTriggerPath()`, `CollectTriggeredBasicEvents()` |
| `state.go` | **核心** | `StateManager`（诊断侧）：带 TTL 的事件状态管理（默认 5 分钟），自动清理协程。209 行 |

### 5.5 pkg/models/

| 文件 | 分类 | 说明 |
|------|------|------|
| `diagnosis.go` | **核心** | `DiagnosisResult`：DiagnosisID, FaultTreeID, TopEventID, FaultCode, TriggerPath, BasicEvents, Metadata |
| `event.go` | **核心** | `AlertEvent`(AlertID, Status firing/resolved, Severity), `EventState`(True/False/Unknown) |
| `fault_tree.go` | **核心** | `FaultTree`, `Event`, `BasicEvent`, `EventNode`, `GateType`(AND/OR/NOT/BASIC) |

### 5.6 pkg/receiver/

| 文件 | 分类 | 说明 |
|------|------|------|
| `interface.go` | **核心** | `Receiver` 接口：Start/Stop/SetHandler |
| `channel_receiver.go` | **核心** | Go channel 实现：带缓冲队列的 `SendAlert()` + `consume()` 协程 |
| `wrapper.go` | **核心** | `ReceiverWrapper`：将 `interface{}` 适配为 `*models.AlertEvent`（类型断言 + JSON 回退） |

### 5.7 pkg/utils/

| 文件 | 分类 | 说明 |
|------|------|------|
| `logger.go` | **核心** | zap logger 工厂，支持日志级别配置 |

### 5.8 configs/

| 文件 | 分类 | 说明 |
|------|------|------|
| `fault_tree_business.json` | **配置** | 业务层故障树：TOP-001 "CJB-RG-ZD-3"，OR 门 → MID-001 (AND: 电池电压+母线电压+NOT CPU电压) ∣ MID-002 (BASIC: CPU电压) |
| `fault_tree_microservice.json` | **配置** | 微服务层故障树：TOP-MS-001 "SVC-PERF-001" OR(CPU高/CPU波动), TOP-MS-002 "CONTAINER-RESOURCE-001" OR(CPU/内存/磁盘高) |

### 5.9 test/

| 文件 | 分类 | 说明 |
|------|------|------|
| `diagnosis_test.go` | **测试** | 业务层 3 场景 + 微服务层 3 场景单元测试 |

---

## 6. fault-recovery/ — 故障修复模块

### 6.1 模块根

| 文件 | 分类 | 说明 |
|------|------|------|
| `design.md` | **文档** | 模块设计说明：异步非阻塞，故障码→修复动作映射，状态锁定+超时监控+结果验证 |

### 6.2 cmd/

| 文件 | 分类 | 说明 |
|------|------|------|
| `cmd/recovery/main.go` | **核心入口** | HTTP 服务（默认 `:8088`），接收 POST `/diagnosis` 诊断事件。注册 `CircuitBreakerAction`（CONTAINER-RESOURCE-001）和 `StartContainerAction`（BUSINESS-IMAGE-START）。含 `/health` 健康检查端点 |

### 6.3 pkg/recovery/

| 文件 | 分类 | 说明 |
|------|------|------|
| `types.go` | **核心** | `DiagnosisResult` 结构（与 fault-diagnosis 一致）, `RecoveryResult`, `DiagnosisStatus()`, `DiagnosisTargetID()`, 常量定义 (FIRING/RESOLVED, SUCCESS/FAILED/TIMEOUT/REJECTED/NO_ACTION) |
| `engine.go` | **核心** | `Engine`：异步非阻塞修复执行引擎。`RegisterAction` / `RegisterPrefixAction`, `Start` (goroutine 从 queue 消费), `Submit`, `handleEvent` (状态锁 → 执行 → 超时判定 → 结果上报), `executeAction` (区分 firing 执行 Execute / resolved 走 Resolve+Verify)。242 行 |
| `actions.go` | **核心** | 两大修复动作实现（**1183 行**，模块最大文件）：<br>• `CircuitBreakerAction`：通过 SSH 调用 SylixOS netfilter 防火墙命令实现熔断(DROP)/恢复(DELETE)，含安全端口白名单(22/3001/8080)、PTY 伪终端支持<br>• `StartContainerAction`：通过 ECSM API 创建服务→等待容器退出→验证故障是否解除→重试（最多 maxRetries 次）→升级报告。支持从 JSON 配置文件加载镜像参数<br>• `RuntimeStore`：内存模拟控制面状态（熔断器状态、容器运行状态、服务 ID）<br>• 辅助函数：SSH 执行、ECSM 实例 IP 查询、环境变量读取、元数据提取 |
| `state.go` | **核心** | `StateManager` 接口 + `InMemoryStateManager`：`LockRecovering` / `UpdateState` / `ReportResult`。状态：RECOVERING → HEALTHY ∣ FAILED |

### 6.4 configs/

| 文件 | 分类 | 说明 |
|------|------|------|
| `recovery_service_config.json` | **配置** | 故障码→镜像映射：<br>• `BUSINESS-IMAGE-START` → c_worker 容器<br>• `MID-001` → worker 容器（含完整 SylixOS 资源限制、VSOA 配置、设备映射）<br>• `MID-002` → ad_module_service 容器。196 行 |

---

## 7. build/ — 编译产物

| 文件 | 分类 | 说明 |
|------|------|------|
| `build/health-monitor-sylixos` | 构建产物 | SylixOS ARM64 二进制 |
| `build/integration-test` | 构建产物 | 集成测试二进制 |
| `build/test` | 构建产物 | 测试二进制 |
| `fault-diagnosis/build/fault-diagnosis-sylixos` | 构建产物 | SylixOS ARM64 二进制 |
| `health-monitor/build/health-monitor-sylixos` | 构建产物 | SylixOS ARM64 二进制 |

---

## 8. .trunk/ — Linter 配置

| 文件 | 分类 | 说明 |
|------|------|------|
| `.trunk/.gitignore` | 配置 | trunk 忽略文件 |
| `.trunk/trunk.yaml` | 配置 | trunk 主配置 |
| `.trunk/configs/.hadolint.yaml` | 配置 | Dockerfile linter 配置 |
| `.trunk/configs/.markdownlint.yaml` | 配置 | Markdown linter 配置 |
| `.trunk/configs/.shellcheckrc` | 配置 | Shell 脚本 linter 配置 |
| `.trunk/configs/.yamllint.yaml` | 配置 | YAML linter 配置 |
| `.trunk/configs/analyzers.yml` | 配置 | 分析器配置 |

---

## 9. 重复/相似文件分析

### 9.1 `RecoveryResult` / `RecoveryStateManager` — 重复定义

| 位置 | 定义 |
|------|------|
| `fault-recovery/pkg/recovery/types.go` | `RecoveryResult` 结构体 + 常量 |
| `fault-recovery/pkg/recovery/state.go` | `StateManager` 接口 + `InMemoryStateManager` |
| `health-monitor/pkg/state/recovery_state.go` | **几乎完全相同**的 `RecoveryResult` + `RecoveryStateManager` 接口 + `InMemoryRecoveryStateManager` |

**结论**：recovery_state.go 是 fault-recovery/state.go 的镜像拷贝，放在 health-monitor 侧供状态查询使用。两者结构一致但包名不同，存在同步风险。

### 9.2 `DiagnosisResult` — 三处定义

| 位置 | 说明 |
|------|------|
| `fault-diagnosis/pkg/models/diagnosis.go` | 原始定义 |
| `fault-recovery/pkg/recovery/types.go` | 重新定义（字段完全一致） |
| `health-monitor/pkg/alert/adapter.go` | 通过 `ConvertToDiagnosisAlert` 转换为 `map[string]interface{}` |

**结论**：fault-recovery 重新定义了 DiagnosisResult 而非导入 fault-diagnosis 的定义，属于跨模块同步风险。

### 9.3 Logger 占位

| 位置 | 说明 |
|------|------|
| `fault-diagnosis/pkg/utils/logger.go` | **已实现**：完整的 zap logger 工厂 |
| `health-monitor/pkg/utils/logger.go` | **空壳**：标注 UNUSED，仅注释无代码 |

### 9.4 `topology.go` — 两处 TODO

| 位置 |
|------|
| `health-monitor/pkg/models/topology.go` |
| `health-monitor/pkg/microservice/topology.go` |

### 9.5 `StateManager` — 两处不同实现

| 位置 | 用途 |
|------|------|
| `health-monitor/pkg/state/state_manager.go` | 全局指标状态管理（实时+历史+告警状态+etcd快照） |
| `fault-diagnosis/pkg/engine/state.go` | 诊断引擎事件状态管理（TTL+自动清理） |

**结论**：两者功能不同，非重复；但同名可能造成混淆。

### 9.6 文件名拼写错误

| 文件 | 问题 |
|------|------|
| `health-monitor/pkg/microservice/disoatcher.go` | 应为 `dispatcher.go` |
| `health-monitor/pkg/microservice/fetcher` | 无扩展名的空文件 |

---

## 10. 已知问题

| # | 文件 | 问题 | 严重度 |
|---|------|------|--------|
| 1 | `health-monitor/pkg/microservice/disoatcher.go` | 文件名拼写错误（disoatcher → dispatcher） | 低 |
| 2 | `health-monitor/pkg/microservice/fetcher` | 无扩展名的杂项文件，疑似误创建 | 低 |
| 3 | `health-monitor/pkg/business/integration_test.go` | `NewDispatcher()` 调用缺少 StateManager 参数，无法编译 | 高 |
| 4 | `health-monitor/pkg/microservice/integration_test.go` | 使用错误的导入路径 `"alert"` / `"model"`（应为完整模块路径） | 高 |
| 5 | `health-monitor/pkg/state/state_manager_test.go` | 使用错误的导入路径 `"model"`（应为 `health-monitor/pkg/models`） | 高 |
| 6 | `health-monitor/pkg/state/example/main.go` | 使用短导入路径 `"model"` / `"state"`，标注 UNUSED | 中 |
| 7 | `health-monitor/pkg/alert/trend.go` | 大部分实现被注释掉（418 行中仅少量生效） | 中 |
| 8 | `health-monitor/pkg/alert/correlate.go` | TODO 占位，无实现 | 中 |
| 9 | `health-monitor/pkg/alert/debounce.go` | TODO 占位，无实现 | 中 |
| 10 | `health-monitor/pkg/models/topology.go` | TODO 占位 | 低 |
| 11 | `health-monitor/pkg/microservice/topology.go` | TODO 占位 | 低 |
| 12 | `health-monitor/pkg/state/storage.go` | 仅包头注释，无实际代码 | 中 |
| 13 | `health-monitor/pkg/utils/logger.go` | 标注 UNUSED，无实现 | 低 |
| 14 | `cmd/integration_memory/main.go` | 引用已废弃的 `Engine.Diagnose()` API | 中 |
| 15 | `fault-recovery` 不在 `go.work` 中 | 通过 go.mod replace 引入，但缺少 go.work 声明 | 低 |
| 16 | `fault-recovery/pkg/recovery/types.go` 与 `fault-diagnosis/pkg/models/diagnosis.go` | DiagnosisResult 重复定义，跨模块同步风险 | 中 |
| 17 | `health-monitor/pkg/state/recovery_state.go` 与 `fault-recovery/pkg/recovery/state.go` | RecoveryStateManager 几乎完全相同的拷贝 | 中 |
| 18 | `health-monitor/pkg/alert/threshold_stateful.go` | 节点阈值检测代码大部分被注释掉 | 中 |

---

## 文件统计

| 分类 | 数量 |
|------|------|
| Go 核心源码 (.go) | ~42 |
| 测试文件 (*_test.go) | 4 |
| Demo/示例入口 | 5 |
| 集成测试入口 | 6 |
| 配置文件 (json/toml/yaml) | ~15 |
| 文档 (.md) | ~30 |
| 构建脚本 (.sh) | 7 |
| 构建产物（二进制） | 5 |
| 其他 (Dockerfile/Makefile/.gitignore) | ~5 |
| **总计（不含 .git/）** | **~119** |
