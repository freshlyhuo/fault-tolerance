/*
阈值检查统一实现：
1. 传入 StateManager 时，按状态变化输出 firing/resolved。
2. 不传 StateManager（启动初期/无状态模式）时，只输出 firing。
*/
package alert

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"health-monitor/pkg/config"
	"health-monitor/pkg/models"
	"health-monitor/pkg/state"
)

func valueOrDefault(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

func intOrDefault(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func rangeOrDefault(t config.MetricThreshold, defMin, defMax float64) (float64, float64) {
	return normalizeByUnit(valueOrDefault(t.Min, defMin), t.Unit), normalizeByUnit(valueOrDefault(t.Max, defMax), t.Unit)
}

func normalizeByUnit(v float64, unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "mv":
		return v / 1000.0
	case "ma":
		return v / 1000.0
	default:
		return v
	}
}

func isConfiguredInt(v *int) bool {
	return v != nil
}

func shouldSendByState(sm *state.StateManager, alertID string, isFiring bool) (bool, bool) {
	if sm == nil {
		if isFiring {
			return true, true
		}
		return false, false
	}

	return sm.CheckAndUpdateAlertState(alertID, isFiring)
}

func appendAlert(
	alerts []*model.AlertEvent,
	sm *state.StateManager,
	alertID, source string,
	isFiring bool,
	firingMsg, resolvedMsg string,
	metricValue float64,
	metadata map[string]interface{},
	timestamp int64,
) []*model.AlertEvent {
	shouldSend, firing := shouldSendByState(sm, alertID, isFiring)
	if !shouldSend {
		return alerts
	}

	status := model.AlertStatusResolved
	message := resolvedMsg
	if firing {
		status = model.AlertStatusFiring
		message = firingMsg
	}

	return append(alerts, &model.AlertEvent{
		AlertID:     alertID,
		Status:      status,
		Source:      source,
		Message:     message,
		Timestamp:   timestamp,
		FaultCode:   faultCodeForAlertID(alertID),
		MetricValue: metricValue,
		Metadata:    metadata,
	})
}

func previousBusinessData(sm *state.StateManager, id string, current interface{}) interface{} {
	if sm == nil {
		return nil
	}

	history := sm.QueryHistory(state.MetricTypeBusiness, id, HistoryLookupWindow)
	for i := len(history) - 1; i >= 0; i-- {
		bm, ok := history[i].Data.(*model.BusinessMetrics)
		if !ok || bm == nil {
			continue
		}
		if bm.Data == current {
			continue
		}
		return bm.Data
	}
	return nil
}

const HistoryLookupWindow = 10 * time.Minute

func faultCodeForAlertID(alertID string) string {
	switch alertID {
	case "YW-power-TMEZD01095cjb_BatteryVoltage",
		"YW-power-TMEZD01096cjb_BusVoltage",
		"YW-power-TMEZD01011cjb_CPUVoltage":
		return "YW-RG-ZD-3"
	case "YW-power-TMEZD01100cjb_ThermalRefVoltage":
		return "YW-RG-ZD-4"
	case "YW-power-TMEZD01247_LoadCurrent":
		return "YW-O2-CS-1"
	case "YW-thermal-TMEZD01121_PlatformHeater":
		return "YW-RG-ZD-5"
	case "YW-thermal-TMEZD01254_BatteryHeater":
		return "YW-RG-ZD-6"
	case "YW-thermal-TMEZD01115_TankHeater":
		return "YW-RG-ZD-7"
	case "YW-comm-TMEZD01004cjb_InstructionCount":
		return "YW-RG-ZD-2"
	case "YW-comm-noresponse":
		return "YW-RG-ZD-2"
	case "YW-comm-Com_No_telemetry_count":
		return "YW-O2-CS-1"
	case "YW-comm-TMEZD01046_CheckErrorCount",
		"YW-comm-TMEZD01047_FrameHeaderErrorCount",
		"YW-comm-TMEZD01048_FrameLengthErrorCount",
		"YW-comm-TMEZD01052_ResetCount":
		return "YW-O2-CS-2"
	case "YW-comm-TMEZD01155_SwitchState",
		"YW-comm-TMEZD01150_ReceiveCTACount":
		return "YW-O2-CS-3"
	case "YW-comm-TMEZD01167_TelemetryEncryptStatus":
		return "YW-O2-CS-5"
	case "YW-comm-TMEZD01168_TelemetryEncryptStatus":
		return "YW-O2-CS-6"
	case "YW-MomentumWheel-MomentumWheel_No_telemetry_count":
		return "YW-O2-CS-15"
	case "YW-MomentumWheel-WheelSpeed_X",
		"YW-MomentumWheel-WheelSpeed_Y",
		"YW-MomentumWheel-WheelSpeed_Z":
		return "YW-O2-CS-16"
	case "YW-AttitudeOrbitControl-TMEZD01013_CheckErrorCount",
		"YW-AttitudeOrbitControl-TMEZD01014_FrameHeaderErrorCount",
		"YW-AttitudeOrbitControl-TMEZD01015_FrameLengthErrorCount",
		"YW-AttitudeOrbitControl-TMEZD01016_ResetCount":
		return "YW-O2-CS-17"
	}

	const prefix = "YW-AttitudeOrbitControl-"
	if strings.HasPrefix(alertID, prefix) {
		key := strings.TrimPrefix(alertID, prefix)
		if key == "TMEZD01044_ResetCount" {
			key = "TMEZD01054_ResetCount"
		}
		return attitudeOrbitControlFaultCode(key)
	}

	return ""
}

// CheckPowerThresholds 检查供电服务阈值（无状态入口）
func CheckPowerThresholds(metrics *model.PowerMetrics) []*model.AlertEvent {
	return CheckPowerThresholdsWithState(metrics, nil)
}

// CheckPowerThresholdsWithState 检查供电服务阈值（支持恢复告警）
func CheckPowerThresholdsWithState(metrics *model.PowerMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}

	tc := config.GetThresholdConfig()
	now := time.Now().Unix()
	var alerts []*model.AlertEvent
	batMin, batMax := rangeOrDefault(tc.Power.TMEZD01095CjBBatteryVolt, 21.0, 29.4)
	busMin, busMax := rangeOrDefault(tc.Power.TMEZD01096CjBBusVolt, 21.0, 29.4)
	cpuMin, cpuMax := rangeOrDefault(tc.Power.TMEZD01011CjBCPUVolt, 3.1, 3.5)
	thermalRefMin, thermalRefMax := rangeOrDefault(tc.Power.TMEZD01100CjBThermalRef, 4.5, 5.5)
	loadMin, loadMax := rangeOrDefault(tc.Power.TMEZD01247LoadCurrent, 0.5, 5.0)

	isFiring := metrics.BatteryVoltage < batMin || metrics.BatteryVoltage > batMax
	alerts = appendAlert(alerts, sm,
		"YW-power-TMEZD01095cjb_BatteryVoltage", "BatteryVoltage", isFiring,
		fmt.Sprintf("蓄电池电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.BatteryVoltage, batMin, batMax),
		fmt.Sprintf("蓄电池电压已恢复正常: %.2fV", metrics.BatteryVoltage),
		metrics.BatteryVoltage, nil, now)

	isFiring = metrics.BusVoltage < busMin || metrics.BusVoltage > busMax
	alerts = appendAlert(alerts, sm,
		"YW-power-TMEZD01096cjb_BusVoltage", "BusVoltage", isFiring,
		fmt.Sprintf("母线电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.BusVoltage, busMin, busMax),
		fmt.Sprintf("母线电压已恢复正常: %.2fV", metrics.BusVoltage),
		metrics.BusVoltage, nil, now)

	isFiring = metrics.CPUVoltage < cpuMin || metrics.CPUVoltage > cpuMax
	alerts = appendAlert(alerts, sm,
		"YW-power-TMEZD01011cjb_CPUVoltage", "CPUVoltage", isFiring,
		fmt.Sprintf("CPU板电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.CPUVoltage, cpuMin, cpuMax),
		fmt.Sprintf("CPU板电压已恢复正常: %.2fV", metrics.CPUVoltage),
		metrics.CPUVoltage, nil, now)

	isFiring = metrics.ThermalRefVoltage < thermalRefMin || metrics.ThermalRefVoltage > thermalRefMax
	alerts = appendAlert(alerts, sm,
		"YW-power-TMEZD01100cjb_ThermalRefVoltage", "ThermalRefVoltage", isFiring,
		fmt.Sprintf("热敏基准电压异常: %.2fV (正常[%.2f,%.2f]V)", metrics.ThermalRefVoltage, thermalRefMin, thermalRefMax),
		fmt.Sprintf("热敏基准电压已恢复正常: %.2fV", metrics.ThermalRefVoltage),
		metrics.ThermalRefVoltage, nil, now)

	isFiring = metrics.LoadCurrent < loadMin || metrics.LoadCurrent > loadMax
	alerts = appendAlert(alerts, sm,
		"YW-power-TMEZD01247_LoadCurrent", "LoadCurrent", isFiring,
		fmt.Sprintf("负载电流异常: %.2fA (正常[%.2f,%.2f]A)", metrics.LoadCurrent, loadMin, loadMax),
		fmt.Sprintf("负载电流已恢复正常: %.2fA", metrics.LoadCurrent),
		metrics.LoadCurrent, nil, now)

	return alerts
}

// CheckThermalThresholds 检查热控服务阈值（当前仅触发告警）
func CheckThermalThresholds(metrics *model.ThermalMetrics) []*model.AlertEvent {
	return CheckThermalThresholdsWithState(metrics, nil)
}

// CheckThermalThresholdsWithState 检查热控服务阈值（支持开关状态恢复告警）
func CheckThermalThresholdsWithState(metrics *model.ThermalMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}
	tc := config.GetThresholdConfig()
	var alerts []*model.AlertEvent
	thermMin, thermMax := rangeOrDefault(config.MetricThreshold{}, -20.0, 50.0)
	if len(tc.Thermal.ThermalTemps) > 0 {
		thermMin, thermMax = rangeOrDefault(config.MetricThreshold{Min: tc.Thermal.ThermalTemps[0].Min, Max: tc.Thermal.ThermalTemps[0].Max}, -20.0, 50.0)
	}
	bat1Min, bat1Max := rangeOrDefault(tc.Thermal.BatteryTemp1, 0.0, 45.0)
	bat2Min, bat2Max := rangeOrDefault(tc.Thermal.BatteryTemp2, 0.0, 45.0)

	for i, temp := range metrics.ThermalTemps {
		if temp < thermMin || temp > thermMax {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("YW-thermal-TMEZD%05dcjb_ThermalTemp", 1066+i),
				Type:        "TemperatureAbnormal",
				Status:      model.AlertStatusFiring,
				Source:      fmt.Sprintf("ThermalTemp%d", i+1),
				Message:     fmt.Sprintf("热控温度%d异常: %.1f℃", i+1, temp),
				Timestamp:   metrics.Timestamp,
				FaultCode:   "YW-RG-ZD-4",
				MetricValue: temp,
			})
		}
	}

	if metrics.BatteryTemp1 < bat1Min || metrics.BatteryTemp1 > bat1Max {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     "YW-thermal-TMEZD01084_BatteryTemp1",
			Type:        "TemperatureAbnormal",
			Status:      model.AlertStatusFiring,
			Source:      "BatteryTemp1",
			Message:     fmt.Sprintf("蓄电池温度1异常: %.1f℃", metrics.BatteryTemp1),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "YW-RG-ZD-4",
			MetricValue: metrics.BatteryTemp1,
		})
	}

	if metrics.BatteryTemp2 < bat2Min || metrics.BatteryTemp2 > bat2Max {
		alerts = append(alerts, &model.AlertEvent{
			AlertID:     "YW-thermal-TMEZD01085_BatteryTemp2",
			Type:        "TemperatureAbnormal",
			Status:      model.AlertStatusFiring,
			Source:      "BatteryTemp2",
			Message:     fmt.Sprintf("蓄电池温度2异常: %.1f℃", metrics.BatteryTemp2),
			Timestamp:   metrics.Timestamp,
			FaultCode:   "YW-RG-ZD-4",
			MetricValue: metrics.BatteryTemp2,
		})
	}

	now := time.Now().Unix()
	checkHeater := func(alertID, source string, actual bool, expected *bool) {
		if expected == nil {
			return
		}
		isFiring := actual != *expected
		value := 0.0
		if actual {
			value = 1
		}
		alerts = appendAlert(alerts, sm,
			alertID, source, isFiring,
			fmt.Sprintf("热控开关状态异常: %s 当前=%t, 期望=%t", source, actual, *expected),
			fmt.Sprintf("热控开关状态已恢复: %s 当前=%t", source, actual),
			value, map[string]interface{}{"expected": *expected, "actual": actual}, now)
	}

	checkHeater("YW-thermal-TMEZD01121_PlatformHeater", "TMEZD01121_PlatformHeater",
		metrics.PlatformHeaterSwitch, tc.Thermal.PlatformHeatingExpected.Value)
	checkHeater("YW-thermal-TMEZD01254_BatteryHeater", "TMEZD01254_BatteryHeater",
		metrics.BatteryHeaterSwitch, tc.Thermal.BatteryHeatingExpected.Value)
	checkHeater("YW-thermal-TMEZD01115_TankHeater", "TMEZD01115_TankHeater",
		metrics.TankHeaterSwitch, tc.Thermal.TankHeatingExpected.Value)

	return alerts
}

// CheckCommThresholds 检查通信服务阈值（当前仅触发告警）
func CheckCommThresholds(metrics *model.CommMetrics) []*model.AlertEvent {
	return CheckCommThresholdsWithState(metrics, nil)
}

// CheckCommThresholdsWithState 检查通信服务计数类指标（支持历史比较与恢复告警）
func CheckCommThresholdsWithState(metrics *model.CommMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}
	tc := config.GetThresholdConfig()
	var alerts []*model.AlertEvent
	prev, _ := previousBusinessData(sm, "comm", metrics).(*model.CommMetrics)

	now := time.Now().Unix()
	checkNotIncreased := func(alertID, source string, current, previous uint32) {
		isFiring := current <= previous
		alerts = appendAlert(alerts, sm,
			alertID, source, isFiring,
			fmt.Sprintf("%s 计数未增加: 上次=%d, 当前=%d", source, previous, current),
			fmt.Sprintf("%s 计数已恢复增长: 上次=%d, 当前=%d", source, previous, current),
			float64(current), map[string]interface{}{"previous": previous, "current": current}, now)
	}
	checkIncrease := func(alertID, source string, current, previous uint32) {
		isFiring := current > previous
		alerts = appendAlert(alerts, sm,
			alertID, source, isFiring,
			fmt.Sprintf("%s 计数增加: 上次=%d, 当前=%d", source, previous, current),
			fmt.Sprintf("%s 计数未继续增加: 当前=%d", source, current),
			float64(current), map[string]interface{}{"previous": previous, "current": current}, now)
	}
	checkExpected := func(alertID, source string, current uint8, expected *int) {
		if expected == nil {
			return
		}
		actual := int(current)
		isFiring := actual != *expected
		alerts = appendAlert(alerts, sm,
			alertID, source, isFiring,
			fmt.Sprintf("%s 状态与期望不一致: 当前=%d, 期望=%d", source, actual, *expected),
			fmt.Sprintf("%s 状态已恢复: 当前=%d", source, actual),
			float64(actual), map[string]interface{}{"expected": *expected, "actual": actual}, now)
	}

	if prev != nil {
		checkNotIncreased("YW-comm-TMEZD01004cjb_InstructionCount", "TMEZD01004cjb_InstructionCount",
			metrics.ReceiveCmdCount, prev.ReceiveCmdCount)

		allO1NotIncreased := metrics.O1ReceiveDeviceResponseCount <= prev.O1ReceiveDeviceResponseCount &&
			metrics.O1ReceiveTelemetryResponseCount <= prev.O1ReceiveTelemetryResponseCount &&
			metrics.O1ReceiveRemoteControlResponseCount <= prev.O1ReceiveRemoteControlResponseCount
		alerts = appendAlert(alerts, sm,
			"YW-comm-noresponse", "O1_ResponseCounts", allO1NotIncreased,
			"通信O1响应计数均未增加",
			"通信O1响应计数已恢复增长",
			float64(metrics.O1ReceiveDeviceResponseCount+metrics.O1ReceiveTelemetryResponseCount+metrics.O1ReceiveRemoteControlResponseCount),
			map[string]interface{}{
				"previous_device":         prev.O1ReceiveDeviceResponseCount,
				"previous_telemetry":      prev.O1ReceiveTelemetryResponseCount,
				"previous_remote_control": prev.O1ReceiveRemoteControlResponseCount,
				"current_device":          metrics.O1ReceiveDeviceResponseCount,
				"current_telemetry":       metrics.O1ReceiveTelemetryResponseCount,
				"current_remote_control":  metrics.O1ReceiveRemoteControlResponseCount,
			}, now)

		checkNotIncreased("YW-comm-TMEZD01150_ReceiveCTACount", "TMEZD01150_ReceiveCTACount",
			metrics.ReceiveCTACount, prev.ReceiveCTACount)

		checkIncrease("YW-comm-TMEZD01046_CheckErrorCount", "TMEZD01046_CheckErrorCount",
			uint32(metrics.ParityErrorCount), uint32(prev.ParityErrorCount))
		checkIncrease("YW-comm-TMEZD01047_FrameHeaderErrorCount", "TMEZD01047_FrameHeaderErrorCount",
			uint32(metrics.FrameHeaderErrorCount), uint32(prev.FrameHeaderErrorCount))
		checkIncrease("YW-comm-TMEZD01048_FrameLengthErrorCount", "TMEZD01048_FrameLengthErrorCount",
			uint32(metrics.FrameLengthErrorCount), uint32(prev.FrameLengthErrorCount))
		checkIncrease("YW-comm-TMEZD01052_ResetCount", "TMEZD01052_ResetCount",
			uint32(metrics.SerialResetCount), uint32(prev.SerialResetCount))
		checkIncrease("YW-comm-Com_No_telemetry_count", "Com_No_telemetry_count",
			metrics.NoTelemetryCount, prev.NoTelemetryCount)
	}

	checkExpected("YW-comm-TMEZD01155_SwitchState", "TMEZD01155_SwitchState",
		metrics.TransmitSwitch, tc.Comm.TransmissionExpected.Value)
	checkExpected("YW-comm-TMEZD01167_TelemetryEncryptStatus", "TMEZD01167_TelemetryEncryptStatus",
		metrics.TelemetryEncryptStatus, tc.Comm.TelemetryExpected.Value)
	checkExpected("YW-comm-TMEZD01168_TelemetryEncryptStatus", "TMEZD01168_TelemetryEncryptStatus",
		metrics.TelecontrolEncryptStatus, tc.Comm.RemoteControlExpected.Value)

	return alerts
}

// CheckActuatorThresholds 检查姿态控制机构阈值（当前仅触发告警）
func CheckActuatorThresholds(metrics *model.ActuatorMetrics) []*model.AlertEvent {
	return CheckActuatorThresholdsWithState(metrics, nil)
}

// CheckActuatorThresholdsWithState 检查姿态控制机构阈值与计数增量（支持恢复告警）
func CheckActuatorThresholdsWithState(metrics *model.ActuatorMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}
	tc := config.GetThresholdConfig()
	var alerts []*model.AlertEvent
	now := time.Now().Unix()

	tolerance := valueOrDefault(tc.MomentumWheel.WheelSpeedTolerance, 10)
	xMin, xMax := wheelRange(tc.MomentumWheel.WheelSpeedX, 100, tolerance)
	yMin, yMax := wheelRange(tc.MomentumWheel.WheelSpeedY, 100, tolerance)
	zMin, zMax := wheelRange(tc.MomentumWheel.WheelSpeedZ, 100, tolerance)

	checkWheel := func(speed int16, axis string, min, max float64) {
		s := float64(speed)
		if s < min || s > max {
			alerts = append(alerts, &model.AlertEvent{
				AlertID:     fmt.Sprintf("YW-MomentumWheel-WheelSpeed_%s", axis),
				Type:        "ActuatorAbnormal",
				Status:      model.AlertStatusFiring,
				Source:      fmt.Sprintf("WheelSpeed%s", axis),
				Message:     fmt.Sprintf("%s轴动量轮转速异常: %d (正常[%.0f,%.0f]rpm)", axis, speed, min, max),
				Timestamp:   metrics.Timestamp,
				FaultCode:   "YW-O2-CS-16",
				MetricValue: float64(speed),
			})
		}
	}

	if metrics.SendCmdK53029 == 1 {
		checkWheel(metrics.WheelSpeedX, "X", xMin, xMax)
		checkWheel(metrics.WheelSpeedY, "Y", yMin, yMax)
		checkWheel(metrics.WheelSpeedZ, "Z", zMin, zMax)
	}

	prev, _ := previousBusinessData(sm, "momentum_wheel", metrics).(*model.ActuatorMetrics)
	if prev != nil {
		checkIncrease := func(alertID, source string, current, previous uint32) {
			isFiring := current > previous
			alerts = appendAlert(alerts, sm,
				alertID, source, isFiring,
				fmt.Sprintf("%s 计数增加: 上次=%d, 当前=%d", source, previous, current),
				fmt.Sprintf("%s 计数未继续增加: 当前=%d", source, current),
				float64(current), map[string]interface{}{"previous": previous, "current": current}, now)
		}

		checkIncrease("YW-MomentumWheel-MomentumWheel_No_telemetry_count", "MomentumWheel_No_telemetry_count",
			metrics.NoTelemetryCount, prev.NoTelemetryCount)
		checkIncrease("YW-AttitudeOrbitControl-TMEZD01013_CheckErrorCount", "TMEZD01013_CheckErrorCount",
			metrics.CheckErrorCount, prev.CheckErrorCount)
		checkIncrease("YW-AttitudeOrbitControl-TMEZD01014_FrameHeaderErrorCount", "TMEZD01014_FrameHeaderErrorCount",
			metrics.FrameHeaderErrorCount, prev.FrameHeaderErrorCount)
		checkIncrease("YW-AttitudeOrbitControl-TMEZD01015_FrameLengthErrorCount", "TMEZD01015_FrameLengthErrorCount",
			metrics.FrameLengthErrorCount, prev.FrameLengthErrorCount)
		checkIncrease("YW-AttitudeOrbitControl-TMEZD01016_ResetCount", "TMEZD01016_ResetCount",
			metrics.ResetCount, prev.ResetCount)
	}

	return alerts
}

func wheelRange(t config.MetricThreshold, expected, tolerance float64) (float64, float64) {
	if t.Min != nil || t.Max != nil {
		return rangeOrDefault(t, expected-tolerance, expected+tolerance)
	}
	if t.Value != nil {
		expected = normalizeByUnit(*t.Value, t.Unit)
	}
	return expected - tolerance, expected + tolerance
}

// CheckAttitudeOrbitControlThresholds 检查姿态轨道控制阈值（无状态入口）
func CheckAttitudeOrbitControlThresholds(metrics *model.AttitudeOrbitControlMetrics) []*model.AlertEvent {
	return CheckAttitudeOrbitControlThresholdsWithState(metrics, nil)
}

// CheckAttitudeOrbitControlThresholdsWithState 检查姿态轨道控制指标（支持恢复告警）
func CheckAttitudeOrbitControlThresholdsWithState(metrics *model.AttitudeOrbitControlMetrics, sm *state.StateManager) []*model.AlertEvent {
	if metrics == nil {
		return nil
	}

	now := time.Now().Unix()
	var alerts []*model.AlertEvent
	prev, _ := previousBusinessData(sm, "attitude_orbit_control", metrics).(*model.AttitudeOrbitControlMetrics)
	if prev == nil {
		return alerts
	}

	checkIncrease := func(key string) {
		actual, ok := intValueFromMap(metrics.Values, key)
		previous, prevOK := intValueFromMap(prev.Values, key)
		metadata := map[string]interface{}{
			"present":          ok,
			"previous_present": prevOK,
		}
		if ok {
			metadata["actual"] = actual
		}
		if prevOK {
			metadata["previous"] = previous
		}

		isFiring := ok && prevOK && actual > previous
		metricValue := 0.0
		if ok {
			metricValue = float64(actual)
		}
		alerts = appendAlert(alerts, sm,
			"YW-AttitudeOrbitControl-"+aocAlertKey(key), key, isFiring,
			fmt.Sprintf("姿态轨道控制计数 %s 增加: 上次=%s, 当前=%s", key, actualValueText(prevOK, previous), actualValueText(ok, actual)),
			fmt.Sprintf("姿态轨道控制计数 %s 未继续增加: 当前=%s", key, actualValueText(ok, actual)),
			metricValue, metadata, now)
	}

	for _, key := range attitudeOrbitControlIncreaseKeys() {
		checkIncrease(key)
	}

	return alerts
}

func actualValueText(ok bool, actual int) string {
	if !ok {
		return "missing"
	}
	return strconv.Itoa(actual)
}

func aocAlertKey(key string) string {
	// 故障树模板里 GNSS 复位计数使用 TMEZD01044，阈值配置当前键为 TMEZD01054。
	// 这里按故障树 alert_id 输出，保证告警可以路由到 basic_event。
	if key == "TMEZD01054_ResetCount" {
		return "TMEZD01044_ResetCount"
	}
	return key
}

func attitudeOrbitControlFaultCode(key string) string {
	switch key {
	case "GNSS_No_telemetry_count":
		return "YW-O2-CS-7"
	case "TMEZD01041_CheckErrorCount",
		"TMEZD01042_FrameHeaderErrorCount",
		"TMEZD01043_FrameLengthErrorCount",
		"TMEZD01054_ResetCount":
		return "YW-O2-CS-8"
	case "Gyroscope_No_telemetry_count":
		return "YW-O2-CS-9"
	case "TMEZD01021_CheckErrorCount",
		"TMEZD01022_FrameHeaderErrorCount",
		"TMEZD01023_FrameLengthErrorCount",
		"TMEZD01024_ResetCount":
		return "YW-O2-CS-10"
	case "MEMS_No_telemetry_count":
		return "YW-O2-CS-11"
	case "TMEZD01025_CheckErrorCount",
		"TMEZD01026_FrameHeaderErrorCount",
		"TMEZD01027_FrameLengthErrorCount",
		"TMEZD01028_ResetCount":
		return "YW-O2-CS-12"
	case "StarTrackerl_No_telemetry_count":
		return "YW-O2-CS-13"
	case "TMEZD01033_CheckErrorCount",
		"TMEZD01034_FrameHeaderErrorCount",
		"TMEZD01035_FrameLengthErrorCount",
		"TMEZD01036_ResetCount",
		"TMEZD01037_CheckErrorCount",
		"TMEZD01038_FrameHeaderErrorCount",
		"TMEZD01039_FrameLengthErrorCount",
		"TMEZD01040_ResetCount":
		return "YW-O2-CS-14"
	default:
		return "YW-AOC-UNKNOWN"
	}
}

func intValue(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case uint:
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		return int(x), true
	case uint64:
		return int(x), true
	case float64:
		return int(x), x == float64(int(x))
	case float32:
		return int(x), x == float32(int(x))
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	default:
		return 0, false
	}
}

func intValueFromMap(values map[string]interface{}, key string) (int, bool) {
	if values == nil {
		return 0, false
	}
	return intValue(values[key])
}

func attitudeOrbitControlIncreaseKeys() []string {
	return []string{
		"GNSS_No_telemetry_count",
		"TMEZD01041_CheckErrorCount",
		"TMEZD01042_FrameHeaderErrorCount",
		"TMEZD01043_FrameLengthErrorCount",
		"TMEZD01054_ResetCount",
		"Gyroscope_No_telemetry_count",
		"TMEZD01021_CheckErrorCount",
		"TMEZD01022_FrameHeaderErrorCount",
		"TMEZD01023_FrameLengthErrorCount",
		"TMEZD01024_ResetCount",
		"MEMS_No_telemetry_count",
		"TMEZD01025_CheckErrorCount",
		"TMEZD01026_FrameHeaderErrorCount",
		"TMEZD01027_FrameLengthErrorCount",
		"TMEZD01028_ResetCount",
		"StarTrackerl_No_telemetry_count",
		"TMEZD01033_CheckErrorCount",
		"TMEZD01034_FrameHeaderErrorCount",
		"TMEZD01035_FrameLengthErrorCount",
		"TMEZD01036_ResetCount",
		"TMEZD01037_CheckErrorCount",
		"TMEZD01038_FrameHeaderErrorCount",
		"TMEZD01039_FrameLengthErrorCount",
		"TMEZD01040_ResetCount",
	}
}
