# 智能化卫星开发的软件容错系统

本项目用于卫星业务系统的故障闭环处理，当前主链路为：

```text
硬件遥测发布 -> 健康监测 -> 故障诊断 -> 故障修复 -> 修复容器 VSOA 指令下发
```

项目依赖 SylixOS ARM64 目标环境运行板上测试程序，故障修复指令通过修复容器的 VSOA 接口下发。

## 模块组成

- `health-monitor`：通过 VSOA Pub/Sub 接收硬件遥测，写入内存状态，并按阈值生成告警。
- `fault-diagnosis`：加载故障树配置，根据告警事件进行故障树推理，输出故障码和诊断结果。
- `fault-recovery`：加载故障码到修复方案的映射，按诊断结果执行修复动作。
- `cmd/board_hardware_pubsub`：板上硬件遥测场景发布器。
- `cmd/integration_health_diagnosis_pubsub`：监测、诊断、修复闭环集成测试程序。
- `cmd/repair_container_probe`：修复容器 VSOA 接口自检工具。

## 关键配置

| 配置 | 路径 | 说明 |
|------|------|------|
| 告警阈值 | `health-monitor/pkg/config/thresholds.json` | 健康监测阈值，板上包会复制为 `thresholds.json` |
| 故障树 | `fault-diagnosis/configs/fault_trees_multi_template.json` | 告警到故障码的诊断规则 |
| 修复方案 | `fault-recovery/configs/recovery_plan_mapping_template.json` | 故障码到修复指令的映射 |

常用环境变量：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HM_THRESHOLD_CONFIG` | `./thresholds.json` | 指定阈值配置文件 |
| `FR_REPAIR_CONTAINER_ADDR` | `127.0.0.1:4551` | 修复容器 VSOA 地址 |
| `FR_REPAIR_CONTAINER_PASSWORD` | 空 | 修复容器 VSOA 密码 |
| `FR_REPAIR_CONTAINER_TIMEOUT_MS` | `10000` | 修复容器调用超时时间 |
| `FR_RECOVERY_PLAN_CONFIG` | `./fault-recovery/configs/recovery_plan_mapping_template.json` | 修复方案配置文件 |

## 板上部署

### 方式 A：使用构建脚本

在仓库根目录构建 SylixOS ARM64 板上测试包：

```sh
scripts/build_board_test.sh
```

生成目录：

```text
build/board_fault_tolerance
```

将该目录整体放到板上 `/apps/fault_tolerance`。目录内包含：

- `integration-health-diagnosis-pubsub-sylixos`
- `board-hardware-pubsub-sylixos`
- `repair-container-probe-sylixos`
- `fault-diagnosis/configs/fault_trees_multi_template.json`
- `fault-recovery/configs/recovery_plan_mapping_template.json`
- `thresholds.json`
- `scripts/run_board_fault_scenario.sh`

### 方式 B：手动构建和上板

如果脚本在当前环境不可执行，可在仓库根目录手动编译三个板上程序：

```sh
mkdir -p build

GOCACHE=/tmp/go-build GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" \
  -o build/integration-health-diagnosis-pubsub-sylixos \
  ./cmd/integration_health_diagnosis_pubsub

GOCACHE=/tmp/go-build GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" \
  -o build/board-hardware-pubsub-sylixos \
  ./cmd/board_hardware_pubsub

GOCACHE=/tmp/go-build GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" \
  -o build/repair-container-probe-sylixos \
  ./cmd/repair_container_probe
```

在板上创建目录：

```sh
mkdir -p /apps/fault_tolerance/fault-diagnosis/configs
mkdir -p /apps/fault_tolerance/fault-recovery/configs
mkdir -p /apps/fault_tolerance/logs
```

把以下文件放到板上对应路径：

| 本地文件 | 板上路径 |
|----------|----------|
| `build/integration-health-diagnosis-pubsub-sylixos` | `/apps/fault_tolerance/integration-health-diagnosis-pubsub-sylixos` |
| `build/board-hardware-pubsub-sylixos` | `/apps/fault_tolerance/board-hardware-pubsub-sylixos` |
| `build/repair-container-probe-sylixos` | `/apps/fault_tolerance/repair-container-probe-sylixos` |
| `fault-diagnosis/configs/fault_trees_multi_template.json` | `/apps/fault_tolerance/fault-diagnosis/configs/fault_trees_multi_template.json` |
| `fault-recovery/configs/recovery_plan_mapping_template.json` | `/apps/fault_tolerance/fault-recovery/configs/recovery_plan_mapping_template.json` |
| `health-monitor/pkg/config/thresholds.json` | `/apps/fault_tolerance/thresholds.json` |

如果板上支持 `scp`，可参考：

```sh
scp build/integration-health-diagnosis-pubsub-sylixos root@板卡IP:/apps/fault_tolerance/
scp build/board-hardware-pubsub-sylixos root@板卡IP:/apps/fault_tolerance/
scp build/repair-container-probe-sylixos root@板卡IP:/apps/fault_tolerance/
scp fault-diagnosis/configs/fault_trees_multi_template.json root@板卡IP:/apps/fault_tolerance/fault-diagnosis/configs/
scp fault-recovery/configs/recovery_plan_mapping_template.json root@板卡IP:/apps/fault_tolerance/fault-recovery/configs/
scp health-monitor/pkg/config/thresholds.json root@板卡IP:/apps/fault_tolerance/thresholds.json
```

上板后赋予执行权限：

```sh
cd /apps/fault_tolerance
chmod +x integration-health-diagnosis-pubsub-sylixos
chmod +x board-hardware-pubsub-sylixos
chmod +x repair-container-probe-sylixos
```

## 运行闭环场景

进入板上目录：

```sh
cd /apps/fault_tolerance
```

如果修复容器不在默认地址，先设置：

```sh
export FR_REPAIR_CONTAINER_ADDR=127.0.0.1:4551
```

运行单个场景：

```sh
scripts/run_board_fault_scenario.sh power_dispatch
```

脚本会启动硬件遥测发布器，并运行集成程序完成健康监测、故障诊断和故障修复。日志输出到：

```text
/apps/fault_tolerance/logs
```

查看当前可用场景：

```sh
./board-hardware-pubsub-sylixos -list
```

常用运行参数：

```sh
PUBSUB_ADDR=127.0.0.1:6551 scripts/run_board_fault_scenario.sh comm_telemetry_fault
RECOVERY_DRAIN_TIMEOUT=60s TEST_TIMEOUT=180s scripts/run_board_fault_scenario.sh momentum_recheck_fail
FR_REPAIR_CONTAINER_ADDR=127.0.0.1:4551 scripts/run_board_fault_scenario.sh power_dispatch
```

如果运行脚本不可用，也可以手动启动两个程序。先启动硬件遥测发布器：

```sh
cd /apps/fault_tolerance
mkdir -p logs
export HM_THRESHOLD_CONFIG=./thresholds.json
export FR_REPAIR_CONTAINER_ADDR=127.0.0.1:4551

./board-hardware-pubsub-sylixos \
  -addr 127.0.0.1:6551 \
  -url /hardware/metrics \
  -interval 2s \
  -scenario power_dispatch \
  -warmup-count 3 \
  -repeat=true \
  > logs/board-publisher-power_dispatch.log 2>&1 &
```

再启动闭环集成程序：

```sh
./integration-health-diagnosis-pubsub-sylixos \
  -hardware-pubsub-addr 127.0.0.1:6551 \
  -hardware-pubsub-url /hardware/metrics \
  -diagnosis-config ./fault-diagnosis/configs/fault_trees_multi_template.json \
  -recovery-plan-config ./fault-recovery/configs/recovery_plan_mapping_template.json \
  -scenario power_dispatch \
  -exit-after-diagnoses 1 \
  -recovery-drain-timeout 45s \
  -timeout 120s \
  2>&1 | tee logs/integration-power_dispatch.log
```

集成程序结束后，停止后台发布器：

```sh
ps
kill 发布器进程号
```

## 单程序调试

启动硬件遥测发布器：

```sh
./board-hardware-pubsub-sylixos \
  -addr 127.0.0.1:6551 \
  -url /hardware/metrics \
  -interval 2s \
  -scenario power_dispatch \
  -warmup-count 3 \
  -repeat=true
```

启动闭环集成程序：

```sh
./integration-health-diagnosis-pubsub-sylixos \
  -hardware-pubsub-addr 127.0.0.1:6551 \
  -hardware-pubsub-url /hardware/metrics \
  -diagnosis-config ./fault-diagnosis/configs/fault_trees_multi_template.json \
  -recovery-plan-config ./fault-recovery/configs/recovery_plan_mapping_template.json \
  -scenario power_dispatch \
  -exit-after-diagnoses 1 \
  -recovery-drain-timeout 45s \
  -timeout 120s
```

修复容器接口自检：

```sh
./repair-container-probe-sylixos -addr 127.0.0.1:4551 -command K50032
```

如需实际下发 RS422 帧，增加 `-send=true`。

## 本地开发测试

运行 Go 单元测试：

```sh
go test ./...
```

也可以分别测试子模块：

```sh
go test ./health-monitor/...
go test ./fault-diagnosis/...
go test ./fault-recovery/...
```

更多板上测试说明见 `docs/BOARD_FAULT_TEST.md`，软件测试报告见 `docs/SOFTWARE_FAULT_TOLERANCE_TEST_REPORT.md`。
