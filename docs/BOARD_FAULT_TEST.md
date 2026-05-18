# 板上故障闭环测试

本测试包用于在 SylixOS ARM64 板上实际跑通：

硬件遥测发布 -> health-monitor 状态入库 -> fault-diagnosis 诊断 -> fault-recovery 执行修复指令 -> repair container VSOA 下发。

## 产物

从仓库根目录执行：

```sh
scripts/build_board_test.sh
```

生成目录：

```sh
build/board_fault_tolerance
```

将该目录整体放到板上：

```sh
/apps/fault_tolerance
```

## 前置条件

修复容器需要先在板上启动，并监听：

```sh
127.0.0.1:4551
```

如果地址不同，运行场景前设置：

```sh
export FR_REPAIR_CONTAINER_ADDR=板卡IP:端口
```

## 运行场景

进入板上目录：

```sh
cd /apps/fault_tolerance
```

执行：

```sh
scripts/run_board_fault_scenario.sh power_dispatch
```

可用场景：

```sh
./board-hardware-pubsub-sylixos -list
```

当前场景：

- `power_dispatch`: 供电电压异常，触发 RP-003，直接下发 `K50032`。
- `power_resolved_cancel`: 先故障后恢复，验证 resolved 事件取消修复任务。
- `ad_dispatch`: 蓄电池/母线/CPU 电压异常，触发 AD 重启 RP-004。
- `momentum_recheck_ok`: 动量轮转速异常，`MomentumWheel_power_status=off`，验证 `CTRL_RECHECK_MomentumWheel_power_status_off` 成功后继续执行。
- `momentum_recheck_fail`: 动量轮转速异常，`MomentumWheel_power_status=on`，验证 recheck 不满足时修复失败/重试。
- `momentum_direct_dispatch`: 通信计数不增长 + 动量轮转速异常，触发 RP-028 直接重发 `K53029`。

日志输出在：

```sh
/apps/fault_tolerance/logs
```

## 常用命令

指定 PubSub 地址：

```sh
PUBSUB_ADDR=127.0.0.1:6551 scripts/run_board_fault_scenario.sh momentum_recheck_ok
```

延长 recheck 场景等待时间：

```sh
RECOVERY_DRAIN_TIMEOUT=60s TEST_TIMEOUT=180s scripts/run_board_fault_scenario.sh momentum_recheck_fail
```

指定修复容器地址：

```sh
FR_REPAIR_CONTAINER_ADDR=127.0.0.1:4551 scripts/run_board_fault_scenario.sh power_dispatch
```
