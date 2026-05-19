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
	Name    string
	Payload payload
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
		step, ok := nextStep(seq, *warmupCount, steps, *repeat)
		if !ok {
			doneOnce.Do(func() { close(done) })
			return
		}

		body := step.Payload
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
		"power_dispatch",
		"power_resolved_cancel",
		"ad_dispatch",
		"momentum_recheck_ok",
		"momentum_recheck_fail",
		"momentum_direct_dispatch",
	} {
		fmt.Printf("  %s\n", name)
	}
}

func nextStep(seq uint64, warmupCount uint, steps []scenarioStep, repeat bool) (scenarioStep, bool) {
	if seq <= uint64(warmupCount) {
		return scenarioStep{Name: "warmup_normal_power", Payload: normalPowerPayload()}, true
	}

	idx := int(seq - uint64(warmupCount) - 1)
	if idx >= len(steps) {
		if !repeat {
			return scenarioStep{}, false
		}
		idx %= len(steps)
	}
	return steps[idx], true
}

func scenarioSteps(name string) ([]scenarioStep, error) {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "power_dispatch":
		return []scenarioStep{
			{Name: "battery_bus_voltage_fault_rp003", Payload: powerBatteryFaultPayload()},
		}, nil
	case "power_resolved_cancel":
		return []scenarioStep{
			{Name: "battery_bus_voltage_fault_rp003", Payload: powerBatteryFaultPayload()},
			{Name: "power_recovered", Payload: normalPowerPayload()},
		}, nil
	case "ad_dispatch":
		return []scenarioStep{
			{Name: "battery_bus_cpu_voltage_fault_rp004", Payload: powerADFaultPayload()},
		}, nil
	case "momentum_recheck_ok":
		return []scenarioStep{
			{Name: "momentum_power_status_off", Payload: powerWithMomentumStatus(false)},
			{Name: "momentum_speed_fault_rp029", Payload: momentumSpeedFaultPayload()},
		}, nil
	case "momentum_recheck_fail":
		return []scenarioStep{
			{Name: "momentum_power_status_on", Payload: powerWithMomentumStatus(true)},
			{Name: "momentum_speed_fault_rp029", Payload: momentumSpeedFaultPayload()},
		}, nil
	case "momentum_direct_dispatch":
		return []scenarioStep{
			{Name: "comm_instruction_counter_seed", Payload: commInstructionCountPayload(10)},
			{Name: "comm_instruction_counter_not_increased", Payload: commInstructionCountPayload(10)},
			{Name: "momentum_speed_fault_rp028", Payload: momentumSpeedFaultPayload()},
		}, nil
	default:
		return nil, fmt.Errorf("unknown scenario %q; run with -list", name)
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

func powerWithMomentumStatus(on bool) payload {
	p := normalPowerPayload()
	if on {
		p.Values["MomentumWheel_power_status"] = 1
	} else {
		p.Values["MomentumWheel_power_status"] = 0
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

func commInstructionCountPayload(count uint32) payload {
	return payload{
		Component: "comm",
		Values: map[string]interface{}{
			"TMEZD01004cjb_InstructionCount":           count,
			"O1_ReceiveDeviceResponseCount":            count,
			"O1_ReceiveTelemetryResponseCount":         count,
			"O1_ReceiveRemoteControlResponseCount":     count,
			"TMEZD01150_ReceiveCTACount":               count,
			"TMEZD01046_CheckErrorCount":               0,
			"TMEZD01047_FrameHeaderErrorCount":         0,
			"TMEZD01048_FrameLengthErrorCount":         0,
			"TMEZD01052_ResetCount":                    0,
			"Com_No_telemetry_count":                   0,
			"TMEZD01155_SwitchState":                   1,
			"TMEZD01167_TelemetryEncryptStatus":        1,
			"TMEZD01168_TelemetryEncryptStatus":        1,
			"Communication_Telemetry_ExpectedState":    1,
			"Communicator_RemoteControl_ExpectedState": 1,
		},
	}
}
