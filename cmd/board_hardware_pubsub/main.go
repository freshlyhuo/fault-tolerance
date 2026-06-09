package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/acoinfo/vsoa/protocol"
	vsoaserver "github.com/acoinfo/vsoa/server"
)

type payload struct {
	Component string                 `json:"component"`
	Timestamp int64                  `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

type scenarioStep struct {
	Name                string
	Payload             payload
	RepeatIncrementKeys []string
	RepeatCycle         bool
}

func main() {
	addr := flag.String("addr", "127.0.0.1:6551", "VSOA发布订阅测试服务监听地址")
	url := flag.String("url", "/hardware/metrics", "发布硬件指标的URL")
	interval := flag.Duration("interval", 2*time.Second, "发布周期")
	scenario := flag.String("scenario", "power_dispatch", "测试场景，使用 -list 查看")
	warmupCount := flag.Uint("warmup-count", 2, "正式场景前发布正常供电遥测的次数")
	repeat := flag.Bool("repeat", true, "场景发布完成后是否循环发布")
	list := flag.Bool("list", false, "列出可用场景后退出")
	flag.Parse()

	if *list {
		printScenarios()
		return
	}

	steps, err := scenarioSteps(*scenario)
	if err != nil {
		log.Fatal(err)
	}

	srv := vsoaserver.NewServer("board-hardware-pubsub-scenarios", vsoaserver.Option{AutoAuth: true})
	done := make(chan struct{})
	var doneOnce sync.Once
	var seq uint64

	if err := srv.Publish(*url, *interval, func(req *protocol.Message, _ *protocol.Message) {
		seq++
		step, repeatIndex, ok := nextStep(seq, *warmupCount, steps, *repeat)
		if !ok {
			doneOnce.Do(func() { close(done) })
			return
		}

		body := materializePayload(step, repeatIndex)
		body.Timestamp = time.Now().Unix()
		data, err := json.Marshal(body)
		if err != nil {
			log.Printf("marshal payload: %v", err)
			return
		}
		req.Param = nil
		req.Data = data
		log.Printf("publish seq=%d scenario=%s step=%s component=%s", seq, *scenario, step.Name, body.Component)
	}); err != nil {
		log.Fatalf("register publisher %s: %v", *url, err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(*addr)
	}()

	fmt.Printf("板上硬件遥测场景发布器已启动: %s URL=%s interval=%s scenario=%s repeat=%v warmup=%d\n",
		*addr, *url, interval.String(), *scenario, *repeat, *warmupCount)
	fmt.Println("按 Ctrl+C 停止")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-done:
		fmt.Println("场景发布完成")
	case sig := <-sigCh:
		fmt.Printf("\n收到退出信号: %s\n", sig)
	case err := <-errCh:
		if err != nil {
			log.Printf("server exited: %v", err)
		}
	}

	if err := srv.Close(); err != nil {
		log.Printf("close server: %v", err)
	}
}

func printScenarios() {
	fmt.Println("可用场景:")
	for _, name := range []string{
		"s-001",
		"s-002",
		"s-003",
		"s-004",
		"s-005",
		"s-006",
		"s-007",
		"s-008",
		"s-009",
		"s-010",
		"s-011",
		"s-012",
		"power_dispatch",
		"power_resolved_cancel",
		"ad_dispatch",
		"thermal_sensor_fault",
		"heater_platform_fault",
		"heater_battery_fault",
		"heater_tank_fault",
		"can_noresponse",
		"comm_start_fail",
		"comm_telemetry_fault",
		"comm_transmit_switch_fault",
		"comm_air_link_fault",
		"comm_telemetry_encrypt_fault",
		"comm_remote_encrypt_fault",
		"gnss_telemetry_fault",
		"gyro_telemetry_fault",
		"mems_telemetry_fault",
		"startracker_telemetry_fault",
		"momentum_start_fail",
		"momentum_recheck_ok",
		"momentum_recheck_fail",
		"momentum_direct_dispatch",
		"momentum_telemetry_fault",
	} {
		fmt.Printf("  %s\n", name)
	}
}

func nextStep(seq uint64, warmupCount uint, steps []scenarioStep, repeat bool) (scenarioStep, uint64, bool) {
	if seq <= uint64(warmupCount) {
		return scenarioStep{Name: "warmup_normal_power", Payload: normalPowerPayload()}, 0, true
	}

	idx := int(seq - uint64(warmupCount) - 1)
	if idx >= len(steps) {
		if !repeat {
			return scenarioStep{}, 0, false
		}
		if shouldCycleRepeat(steps) {
			repeatIndex := uint64(idx / len(steps))
			idx %= len(steps)
			return steps[idx], repeatIndex, true
		}
		repeatIndex := uint64(idx - len(steps) + 1)
		idx = len(steps) - 1
		return steps[idx], repeatIndex, true
	}
	return steps[idx], 0, true
}

func shouldCycleRepeat(steps []scenarioStep) bool {
	for _, step := range steps {
		if step.RepeatCycle {
			return true
		}
	}
	return false
}

func scenarioSteps(name string) ([]scenarioStep, error) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "s-001", "s001", "power_dispatch":
		return []scenarioStep{
			{Name: "battery_bus_voltage_fault_rp003", Payload: powerBatteryFaultPayload()},
		}, nil
	case "s-012", "s012", "power_resolved_cancel":
		return []scenarioStep{
			{Name: "battery_bus_voltage_fault_rp003", Payload: powerBatteryFaultPayload()},
			{Name: "power_recovered", Payload: normalPowerPayload()},
		}, nil
	case "s-002", "s002", "ad_dispatch":
		return []scenarioStep{
			{Name: "battery_bus_cpu_voltage_fault_rp004", Payload: powerADFaultPayload()},
		}, nil
	case "s-003", "s003", "thermal_sensor_fault":
		return []scenarioStep{
			{Name: "thermal_temperature_fault", Payload: thermalTemperatureFaultPayload()},
			{Name: "thermal_ref_voltage_fault_rp005", Payload: powerThermalRefFaultPayload()},
		}, nil
	case "s-004", "s004", "heater_platform_fault":
		return []scenarioStep{
			{Name: "platform_heater_expected_on_actual_off_rp008", Payload: thermalHeaterFaultPayload("platform")},
		}, nil
	case "heater_battery_fault":
		return []scenarioStep{
			{Name: "battery_heater_expected_on_actual_off_rp011", Payload: thermalHeaterFaultPayload("battery")},
		}, nil
	case "heater_tank_fault":
		return []scenarioStep{
			{Name: "tank_heater_expected_on_actual_off_rp014", Payload: thermalHeaterFaultPayload("tank")},
		}, nil
	case "s-005", "s005", "can_noresponse":
		return []scenarioStep{
			{Name: "comm_instruction_counter_seed", Payload: commInstructionCountPayload(10)},
			{Name: "comm_instruction_counter_not_increased_rp002", Payload: commInstructionCountPayload(10)},
		}, nil
	case "s-006", "s006", "comm_telemetry_fault":
		return []scenarioStep{
			{Name: "comm_counter_seed", Payload: commFaultCounterPayload(10, 0, 0, 0, 0, 0)},
			{Name: "comm_no_telemetry_and_error_counter_increase_rp015", Payload: commFaultCounterPayload(11, 1, 1, 0, 0, 0), RepeatIncrementKeys: commTelemetryFaultRepeatCounterKeys()},
		}, nil
	case "comm_start_fail":
		return []scenarioStep{
			{Name: "comm_counter_seed", Payload: commFaultCounterPayload(10, 0, 0, 0, 0, 0)},
			{Name: "comm_no_telemetry_increase", Payload: commFaultCounterPayload(11, 1, 0, 0, 0, 0)},
			{Name: "comm_load_current_low_rp015", Payload: powerLoadCurrentFaultPayload()},
		}, nil
	case "comm_transmit_switch_fault":
		return []scenarioStep{
			{Name: "comm_transmit_switch_expected_on_actual_off_rp017", Payload: commSwitchStateFaultPayload("transmit"), RepeatIncrementKeys: commNormalCounterKeys(), RepeatCycle: true},
			{Name: "communication_power_status_off_for_recheck", Payload: powerWithCommunicationStatus(false), RepeatCycle: true},
		}, nil
	case "comm_air_link_fault":
		return []scenarioStep{
			{Name: "comm_rf_counter_seed", Payload: commRFLinkPayload(10, 10, 10, 10, 40)},
			{Name: "comm_rf_link_fault_rp018", Payload: commRFLinkPayload(11, 10, 10, 10, 200), RepeatIncrementKeys: commRFLinkRepeatCounterKeys()},
		}, nil
	case "comm_telemetry_encrypt_fault":
		return []scenarioStep{
			{Name: "comm_telemetry_encrypt_expected_on_actual_off_rp020", Payload: commSwitchStateFaultPayload("telemetry_encrypt"), RepeatIncrementKeys: commNormalCounterKeys(), RepeatCycle: true},
			{Name: "communication_power_status_off_for_recheck", Payload: powerWithCommunicationStatus(false), RepeatCycle: true},
		}, nil
	case "comm_remote_encrypt_fault":
		return []scenarioStep{
			{Name: "comm_remote_encrypt_expected_on_actual_off_rp022", Payload: commSwitchStateFaultPayload("remote_encrypt"), RepeatIncrementKeys: commNormalCounterKeys(), RepeatCycle: true},
			{Name: "communication_power_status_off_for_recheck", Payload: powerWithCommunicationStatus(false), RepeatCycle: true},
		}, nil
	case "s-007", "s007", "gnss_telemetry_fault":
		return []scenarioStep{
			{Name: "aoc_counter_seed", Payload: attitudeOrbitCounterPayload(nil)},
			{Name: "gnss_no_telemetry_and_error_counter_increase_rp023", Payload: attitudeOrbitCounterPayload(map[string]int{
				"GNSS_No_telemetry_count":    1,
				"TMEZD01041_CheckErrorCount": 1,
			})},
			{Name: "gnssa_power_status_off_for_recheck", Payload: powerWithDeviceStatus("GNSSA_power_status", false)},
		}, nil
	case "s-008", "s008", "gyro_telemetry_fault":
		return []scenarioStep{
			{Name: "aoc_counter_seed", Payload: attitudeOrbitCounterPayload(nil)},
			{Name: "gyro_no_telemetry_and_error_counter_increase_rp024", Payload: attitudeOrbitCounterPayload(map[string]int{
				"Gyroscope_No_telemetry_count": 1,
				"TMEZD01021_CheckErrorCount":   1,
			})},
			{Name: "gyroscope_power_status_off_for_recheck", Payload: powerWithDeviceStatus("Gyroscope_power_status", false)},
		}, nil
	case "s-009", "s009", "mems_telemetry_fault":
		return []scenarioStep{
			{Name: "aoc_counter_seed", Payload: attitudeOrbitCounterPayload(nil)},
			{Name: "mems_no_telemetry_and_error_counter_increase_rp025", Payload: attitudeOrbitCounterPayload(map[string]int{
				"MEMS_No_telemetry_count":    1,
				"TMEZD01025_CheckErrorCount": 1,
			})},
			{Name: "mems_power_status_off_for_recheck", Payload: powerWithDeviceStatus("MEMS_power_status", false)},
		}, nil
	case "s-010", "s010", "startracker_telemetry_fault":
		return []scenarioStep{
			{Name: "aoc_counter_seed", Payload: attitudeOrbitCounterPayload(nil)},
			{Name: "startracker_no_telemetry_and_error_counter_increase_rp026", Payload: attitudeOrbitCounterPayload(map[string]int{
				"StarTrackerl_No_telemetry_count": 1,
				"TMEZD01033_CheckErrorCount":      1,
			})},
			{Name: "startracker_power_status_off_for_recheck", Payload: powerWithDeviceStatus("StarTrackerl_power_status", false)},
		}, nil
	case "momentum_start_fail":
		return []scenarioStep{
			{Name: "momentum_counter_seed", Payload: momentumTelemetryPayload(0, 0, 0, 0, 0)},
			{Name: "momentum_no_telemetry_increase", Payload: momentumTelemetryPayload(1, 0, 0, 0, 0)},
			{Name: "momentum_load_current_low_rp027", Payload: powerLoadCurrentFaultPayload()},
		}, nil
	case "momentum_recheck_ok":
		return []scenarioStep{
			{Name: "momentum_power_status_off", Payload: powerWithMomentumStatus(false), RepeatCycle: true},
			{Name: "momentum_speed_fault_rp029", Payload: momentumSpeedFaultPayload(), RepeatCycle: true},
		}, nil
	case "momentum_recheck_fail":
		return []scenarioStep{
			{Name: "momentum_power_status_on", Payload: powerWithMomentumStatus(true), RepeatCycle: true},
			{Name: "momentum_speed_fault_rp029", Payload: momentumSpeedFaultPayload(), RepeatCycle: true},
		}, nil
	case "momentum_direct_dispatch":
		return []scenarioStep{
			{Name: "comm_instruction_counter_seed", Payload: commInstructionCountPayload(10)},
			{Name: "comm_instruction_counter_not_increased", Payload: commReceiveCmdStalledPayload(10, 11)},
			{Name: "momentum_speed_fault_rp028", Payload: momentumSpeedFaultPayload()},
		}, nil
	case "s-011", "s011":
		return []scenarioStep{
			{Name: "momentum_power_status_off", Payload: powerWithMomentumStatus(false), RepeatCycle: true},
			{Name: "momentum_speed_fault_rp029", Payload: momentumSpeedFaultPayload(), RepeatCycle: true},
		}, nil
	case "momentum_telemetry_fault":
		return []scenarioStep{
			{Name: "momentum_counter_seed", Payload: momentumTelemetryPayload(0, 0, 0, 0, 0)},
			{Name: "momentum_no_telemetry_and_error_counter_increase_rp030", Payload: momentumTelemetryPayload(1, 1, 1, 1, 1)},
			{Name: "momentum_power_status_off_for_recheck", Payload: powerWithMomentumStatus(false)},
		}, nil
	default:
		return nil, fmt.Errorf("unknown scenario %q; run with -list", name)
	}
}

func materializePayload(step scenarioStep, repeatIndex uint64) payload {
	body := clonePayload(step.Payload)
	if repeatIndex == 0 {
		return body
	}
	for _, key := range step.RepeatIncrementKeys {
		if value, ok := body.Values[key]; ok {
			body.Values[key] = incrementNumeric(value, repeatIndex)
		}
	}
	return body
}

func clonePayload(in payload) payload {
	out := payload{
		Component: in.Component,
		Timestamp: in.Timestamp,
		Values:    make(map[string]interface{}, len(in.Values)),
	}
	for key, value := range in.Values {
		out.Values[key] = value
	}
	return out
}

func incrementNumeric(value interface{}, delta uint64) interface{} {
	switch v := value.(type) {
	case uint:
		return v + uint(delta)
	case uint8:
		return v + uint8(delta)
	case uint16:
		return v + uint16(delta)
	case uint32:
		return v + uint32(delta)
	case uint64:
		return v + delta
	case int:
		return v + int(delta)
	case int8:
		return v + int8(delta)
	case int16:
		return v + int16(delta)
	case int32:
		return v + int32(delta)
	case int64:
		return v + int64(delta)
	case float32:
		return v + float32(delta)
	case float64:
		return v + float64(delta)
	default:
		return value
	}
}

func normalPowerPayload() payload {
	return payload{
		Component: "power",
		Values: map[string]interface{}{
			"TMEZD01095cjb_BatteryVoltage":    25.0,
			"TMEZD01096cjb_BusVoltage":        25.0,
			"TMEZD01011cjb_CPUVoltage":        3.3,
			"TMEZD01100cjb_ThermalRefVoltage": 5.0,
			"TMEZD01247_LoadCurrent":          1.0,
			"Communication_power_status":      0,
			"GNSSA_power_status":              0,
			"GNSSB_power_status":              0,
			"Gyroscope_power_status":          0,
			"MEMS_power_status":               0,
			"StarTrackerl_power_status":       0,
			"MomentumWheel_power_status":      0,
		},
	}
}

func powerBatteryFaultPayload() payload {
	p := normalPowerPayload()
	p.Values["TMEZD01095cjb_BatteryVoltage"] = 18.5
	p.Values["TMEZD01096cjb_BusVoltage"] = 18.0
	return p
}

func powerADFaultPayload() payload {
	p := powerBatteryFaultPayload()
	p.Values["TMEZD01011cjb_CPUVoltage"] = 2.4
	return p
}

func powerThermalRefFaultPayload() payload {
	p := normalPowerPayload()
	p.Values["TMEZD01100cjb_ThermalRefVoltage"] = 3.8
	return p
}

func powerLoadCurrentFaultPayload() payload {
	p := normalPowerPayload()
	p.Values["TMEZD01247_LoadCurrent"] = 0.0
	return p
}

func powerWithMomentumStatus(on bool) payload {
	p := normalPowerPayload()
	if on {
		p.Values["MomentumWheel_power_status"] = 1
	} else {
		p.Values["MomentumWheel_power_status"] = 0
	}
	return p
}

func powerWithCommunicationStatus(on bool) payload {
	p := normalPowerPayload()
	if on {
		p.Values["Communication_power_status"] = 1
	} else {
		p.Values["Communication_power_status"] = 0
	}
	return p
}

func powerWithDeviceStatus(metricName string, on bool) payload {
	p := normalPowerPayload()
	if on {
		p.Values[metricName] = 1
	} else {
		p.Values[metricName] = 0
	}
	return p
}

func normalThermalPayload() payload {
	values := map[string]interface{}{
		"TMEZD01084_BatteryTemp1":   20.0,
		"TMEZD01085_BatteryTemp2":   20.0,
		"PlatformThermalTemp":       20.0,
		"BatteryThermalTemp":        20.0,
		"TankThermalTemp":           20.0,
		"TMEZD01121_PlatformHeater": false,
		"TMEZD01254_BatteryHeater":  false,
		"TMEZD01115_TankHeater":     false,
	}
	for i := 0; i < 10; i++ {
		values[fmt.Sprintf("TMEZD%05dcjb_ThermalTemp", 1066+i)] = 20.0
	}
	return payload{Component: "thermal", Values: values}
}

func thermalTemperatureFaultPayload() payload {
	p := normalThermalPayload()
	p.Values["TMEZD01066cjb_ThermalTemp"] = 65.0
	return p
}

func thermalHeaterFaultPayload(which string) payload {
	p := normalThermalPayload()
	switch strings.ToLower(which) {
	case "platform":
		p.Values["PlatformHeatingSwitch_ExpectedState"] = 1
		p.Values["TMEZD01121_PlatformHeater"] = 0
	case "battery":
		p.Values["BatteryHeatingSwitch_ExpectedState"] = 1
		p.Values["TMEZD01254_BatteryHeater"] = 0
	case "tank":
		p.Values["TankHeatingSwitch_ExpectedState"] = 1
		p.Values["TMEZD01115_TankHeater"] = 0
	}
	return p
}

func momentumSpeedFaultPayload() payload {
	return payload{
		Component: "momentum_wheel",
		Values: map[string]interface{}{
			"SendCmd_K53029":                   1,
			"WheelSpeedX":                      125,
			"WheelSpeedY":                      100,
			"WheelSpeedZ":                      100,
			"MomentumWheel_No_telemetry_count": 0,
			"TMEZD01013_CheckErrorCount":       0,
			"TMEZD01014_FrameHeaderErrorCount": 0,
			"TMEZD01015_FrameLengthErrorCount": 0,
			"TMEZD01016_ResetCount":            0,
		},
	}
}

func momentumTelemetryPayload(noTelemetry, checkErr, headerErr, lengthErr, reset uint32) payload {
	return payload{
		Component: "momentum_wheel",
		Values: map[string]interface{}{
			"SendCmd_K53029":                   0,
			"WheelSpeedX":                      100,
			"WheelSpeedY":                      100,
			"WheelSpeedZ":                      100,
			"MomentumWheel_No_telemetry_count": noTelemetry,
			"TMEZD01013_CheckErrorCount":       checkErr,
			"TMEZD01014_FrameHeaderErrorCount": headerErr,
			"TMEZD01015_FrameLengthErrorCount": lengthErr,
			"TMEZD01016_ResetCount":            reset,
		},
	}
}

func commInstructionCountPayload(count uint32) payload {
	return payload{
		Component: "comm",
		Values: map[string]interface{}{
			"TMEZD01004cjb_InstructionCount":           count,
			"O1_ReceiveDeviceResponseCount":            count,
			"O1_ReceiveTelemetryResponseCount":         count,
			"O1_ReceiveRemoteControlResponseCount":     count,
			"TMEZD01150_ReceiveCTACount":               count,
			"Com_SoftwareRecInstructionCount":          count,
			"Com_CorrectRecInstructionCount":           count,
			"Com_CommandSeriaPortCount":                count,
			"TMEZD01046_CheckErrorCount":               0,
			"TMEZD01047_FrameHeaderErrorCount":         0,
			"TMEZD01048_FrameLengthErrorCount":         0,
			"TMEZD01052_ResetCount":                    0,
			"Com_No_telemetry_count":                   0,
			"TMEZD01155_SwitchState":                   1,
			"TMEZD01167_TelemetryEncryptStatus":        1,
			"TMEZD01168_TelemetryEncryptStatus":        1,
			"TMEZD01145_SNR":                           40,
			"TMEZD01147_ReceiveRSSI":                   40,
			"Communication_Telemetry_ExpectedState":    1,
			"Communicator_RemoteControl_ExpectedState": 1,
		},
	}
}

func commNormalCounterKeys() []string {
	return []string{
		"TMEZD01004cjb_InstructionCount",
		"O1_ReceiveDeviceResponseCount",
		"O1_ReceiveTelemetryResponseCount",
		"O1_ReceiveRemoteControlResponseCount",
		"TMEZD01150_ReceiveCTACount",
		"Com_SoftwareRecInstructionCount",
		"Com_CorrectRecInstructionCount",
		"Com_CommandSeriaPortCount",
	}
}

func commReceiveCmdStalledPayload(receiveCmd, normalCount uint32) payload {
	p := commInstructionCountPayload(normalCount)
	p.Values["TMEZD01004cjb_InstructionCount"] = receiveCmd
	return p
}

func commTelemetryFaultRepeatCounterKeys() []string {
	return append(commNormalCounterKeys(),
		"Com_No_telemetry_count",
		"TMEZD01046_CheckErrorCount",
	)
}

func commRFLinkRepeatCounterKeys() []string {
	return []string{
		"TMEZD01004cjb_InstructionCount",
		"O1_ReceiveDeviceResponseCount",
		"O1_ReceiveTelemetryResponseCount",
		"O1_ReceiveRemoteControlResponseCount",
		"TMEZD01150_ReceiveCTACount",
	}
}

func commFaultCounterPayload(receiveCmd, noTelemetry uint32, checkErr, headerErr, lengthErr, reset uint16) payload {
	p := commInstructionCountPayload(receiveCmd)
	p.Values["Com_No_telemetry_count"] = noTelemetry
	p.Values["TMEZD01046_CheckErrorCount"] = checkErr
	p.Values["TMEZD01047_FrameHeaderErrorCount"] = headerErr
	p.Values["TMEZD01048_FrameLengthErrorCount"] = lengthErr
	p.Values["TMEZD01052_ResetCount"] = reset
	return p
}

func commSwitchStateFaultPayload(which string) payload {
	p := commInstructionCountPayload(10)
	switch strings.ToLower(which) {
	case "transmit":
		p.Values["transmission_channel_ExpectedState"] = 1
		p.Values["TMEZD01155_SwitchState"] = 0
	case "telemetry_encrypt":
		p.Values["Communication_Telemetry_ExpectedState"] = 1
		p.Values["TMEZD01167_TelemetryEncryptStatus"] = 0
	case "remote_encrypt":
		p.Values["Communicator_RemoteControl_ExpectedState"] = 1
		p.Values["TMEZD01168_TelemetryEncryptStatus"] = 0
	}
	return p
}

func commRFLinkPayload(baseCount, softwareRec, correctRec, serialPort uint32, snr uint8) payload {
	p := commInstructionCountPayload(baseCount)
	p.Values["Com_SoftwareRecInstructionCount"] = softwareRec
	p.Values["Com_CorrectRecInstructionCount"] = correctRec
	p.Values["Com_CommandSeriaPortCount"] = serialPort
	p.Values["TMEZD01145_SNR"] = snr
	return p
}

func attitudeOrbitCounterPayload(overrides map[string]int) payload {
	keys := []string{
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
	values := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		values[key] = 0
	}
	for key, value := range overrides {
		values[key] = value
	}
	return payload{Component: "attitude_orbit_control", Values: values}
}
