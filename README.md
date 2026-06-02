# 智能化卫星开发的软件容错系统
项目主要负责实现卫星共性服务和微服务的故障监测-诊断-修复
本项目依赖sylinxos下的ECSM容器管理平台管理微服务

## SylixOS 部署指南
### 方法 A: 交叉编译到 SylixOS（推荐）

```bash
# 1. 安装依赖
cd /home/yzj/fault-tolerance/health-monitor
go mod tidy

# 2. 交叉编译 SylixOS ARM64
mkdir -p build
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/health-monitor-sylixos ./cmd/integration_test
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/health-monitor-sylixos ./cmd/integration_test_microservice
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/health-monitor-sylixos ./cmd/integration_test_business

# 3. 编译故障修复配置接口测试程序（从仓库根目录执行）
cd /home/yzj/fault-tolerance
mkdir -p build
GOOS=sylixos GOARCH=arm64 go build -ldflags="-s -w" -o build/fault-recovery-configrpc-sylixos ./fault-recovery/cmd/configrpc_recovery
```

### 3. 在 SylixOS 上运行

```bash
# SSH 登录到 SylixOS
192.168.31.127


# 运行监控程序（纯内存模式）
/usr/local/bin/health-monitor \
  --ecsm-url=http://your-ecsm-platform:8080 \
  --interval=30

# 或使用 etcd 持久化(暂不支持)
/usr/local/bin/health-monitor \
  --ecsm-url=http://your-ecsm-platform:8080 \
  --etcd=192.168.1.200:2379 \
  --interval=30
```
./board-hardware-pubsub-sylixos -addr 127.0.0.1:6551 -url /hardware/metrics -interval 2s -scenario power_dispatch -warmup-count 3 -repeat=true &

./integration-health-diagnosis-pubsub-sylixos -hardware-pubsub-addr 127.0.0.1:6551 -hardware-pubsub-url /hardware/metrics -diagnosis-config ./fault-diagnosis/configs/fault_trees_multi_template.json -recovery-plan-config ./fault-recovery/configs/recovery_plan_mapping_template.json -exit-after-diagnoses-recovery-drain-timeout 45s -timeout 120s

./integration-health-diagnosis-pubsub-sylixos -hardware-pubsub-addr 127.0.0.1:6551 -hardware-pubsub-url /hardware/metrics -diagnosis-config ./fault-diagnosis/configs/fault_trees_multi_template.json -recovery-plan-config ./fault-recovery/configs/recovery_plan_mapping_template.json -exit-after-diagnoses 1 -recovery-drain-timeout 45s -timeout 120s
### 前台运行（调试）

```bash
# 直接运行，查看实时输出
./health-monitor --ecsm-url=http://192.168.1.50:8080 --interval=10
```

### 命令行参数

| 参数 | 说明 | 默认值 | 示例 |
|------|------|--------|------|
| `--ecsm-url` | 容器平台 API 地址 | `http://localhost:8080` | `http://192.168.1.50:8080` |
| `--etcd` | etcd 集群地址 | `` (纯内存) | `localhost:2379` |
| `--interval` | 采集间隔（秒） | `30` | `10` (测试) / `60` (生产) |



## 健康监测

## 故障诊断

## 故障恢复
1. 启停能够解决的微服务问题
2. 如果故障恢复未成功如何处理

## 场景测试
1. 启动发布器：
```
./board-hardware-pubsub-sylixos -addr 127.0.0.1:6551 -url /hardware/metrics -interval 2s -scenario power_resolved_cancel -warmup-count 3 -repeat=true &
```
2. 启动集成测试
```
./integration-health-diagnosis-pubsub-sylixos -hardware-pubsub-addr 127.0.0.1:6551 -hardware-pubsub-url /hardware/metrics -diagnosis-config ./fault-diagnosis/configs/fault_trees_multi_template.json -recovery-plan-config ./fault-recovery/configs/recovery_plan_mapping_template.json -scenario power_resolved_cancel -exit-after-diagnoses 1 -recovery-drain-timeout 45s -timeout 120s
```
3. 所有场景
power_dispatch
ad_dispatch
thermal_sensor_fault
heater_platform_fault
heater_battery_fault
heater_tank_fault
can_noresponse
comm_telemetry_fault
comm_start_fail
comm_transmit_switch_fault
comm_air_link_fault
comm_telemetry_encrypt_fault
comm_remote_encrypt_fault
gnss_telemetry_fault
gyro_telemetry_fault
mems_telemetry_fault
startracker_telemetry_fault
momentum_start_fail
momentum_recheck_ok
momentum_direct_dispatch
momentum_telemetry_fault
power_resolved_cancel