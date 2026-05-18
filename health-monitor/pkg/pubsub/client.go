package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acoinfo/vsoa/client"
	"github.com/acoinfo/vsoa/protocol"

	"health-monitor/pkg/business"
	"health-monitor/pkg/models"
)

const (
	DefaultAddress = "127.0.0.1:3002"
	DefaultURL     = "/hardware/metrics"
)

// Client subscribes hardware metric publications and forwards them into the
// existing health-monitor business dispatcher.
type Client struct {
	address    string
	url        string
	password   string
	dispatcher *business.Dispatcher

	mu     sync.Mutex
	client *client.Client
}

type Option struct {
	Address  string
	URL      string
	Password string
}

// Payload is the JSON shape accepted from the hardware publisher.
//
// Example:
//
//	{
//	  "component": "momentum_wheel",
//	  "timestamp": 1714032000,
//	  "values": {"WheelSpeedX": 120, "MomentumWheel_No_telemetry_count": 1}
//	}
type Payload struct {
	Component string                 `json:"component"`
	Timestamp int64                  `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
}

func NewClient(opt Option, dispatcher *business.Dispatcher) *Client {
	if opt.Address == "" {
		opt.Address = DefaultAddress
	}
	if opt.URL == "" {
		opt.URL = DefaultURL
	}
	return &Client{
		address:    opt.Address,
		url:        opt.URL,
		password:   opt.Password,
		dispatcher: dispatcher,
	}
}

func (c *Client) Start(ctx context.Context) error {
	if c.dispatcher == nil {
		return errors.New("pubsub dispatcher is nil")
	}

	vc := client.NewClient(client.Option{Password: c.password})
	serverInfo, err := vc.Connect("tcp", c.address)
	if err != nil {
		return fmt.Errorf("connect hardware pubsub server %s: %w", c.address, err)
	}

	if err := vc.Subscribe(c.url, c.onPublish(ctx)); err != nil {
		_ = vc.Close()
		return fmt.Errorf("subscribe %s: %w", c.url, err)
	}

	c.mu.Lock()
	c.client = vc
	c.mu.Unlock()

	fmt.Printf("[硬件PubSub] 已连接 %s (%s)，订阅URL=%s\n", c.address, serverInfo, c.url)
	go func() {
		<-ctx.Done()
		if err := c.Close(); err != nil {
			fmt.Printf("[硬件PubSub] 关闭失败: %v\n", err)
		}
	}()

	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	vc := c.client
	c.client = nil
	c.mu.Unlock()
	if vc == nil {
		return nil
	}

	if err := vc.UnSubscribe(c.url); err != nil {
		_ = vc.Close()
		return err
	}
	return vc.Close()
}

func (c *Client) onPublish(ctx context.Context) func(*protocol.Message) {
	return func(m *protocol.Message) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		bm, err := DecodeMessage(m)
		if err != nil {
			fmt.Printf("[硬件PubSub] 解析发布数据失败 URL=%s: %v\n", string(m.URL), err)
			return
		}

		fmt.Printf("[硬件PubSub] 收到指标 URL=%s Type=%T Timestamp=%d\n", string(m.URL), bm.Data, bm.Timestamp)
		c.dispatcher.HandleBusinessMetrics(ctx, bm)
	}
}

func DecodeMessage(m *protocol.Message) (*model.BusinessMetrics, error) {
	if m == nil {
		return nil, errors.New("nil VSOA message")
	}

	raw := m.Data
	if len(raw) == 0 {
		raw = m.Param
	}
	if len(raw) == 0 {
		return nil, errors.New("empty publish payload")
	}

	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Timestamp == 0 {
		payload.Timestamp = time.Now().Unix()
	}
	if payload.Values == nil {
		payload.Values = map[string]interface{}{}
	}

	data, err := toBusinessData(payload)
	if err != nil {
		return nil, err
	}
	return &model.BusinessMetrics{Timestamp: payload.Timestamp, Data: data}, nil
}

func toBusinessData(payload Payload) (interface{}, error) {
	switch normalizeComponent(payload.Component) {
	case "power":
		return &model.PowerMetrics{
			Timestamp:                payload.Timestamp,
			BatteryVoltage:           floatValue(payload.Values, "TMEZD01095cjb_BatteryVoltage", "BatteryVoltage"),
			BusVoltage:               floatValue(payload.Values, "TMEZD01096cjb_BusVoltage", "BusVoltage"),
			CPUVoltage:               floatValue(payload.Values, "TMEZD01011cjb_CPUVoltage", "CPUVoltage"),
			ThermalRefVoltage:        floatValue(payload.Values, "TMEZD01100cjb_ThermalRefVoltage", "ThermalRefVoltage"),
			LoadCurrent:              floatValue(payload.Values, "TMEZD01247_LoadCurrent", "LoadCurrent"),
			CommunicationPowerStatus: boolValue(payload.Values, "Communication_power_status", "CommunicationPowerStatus"),
			GNSSAPowerStatus:         boolValue(payload.Values, "GNSSA_power_status", "GNSSAPowerStatus"),
			GNSSBPowerStatus:         boolValue(payload.Values, "GNSSB_power_status", "GNSSBPowerStatus"),
			GyroscopePowerStatus:     boolValue(payload.Values, "Gyroscope_power_status", "GyroscopePowerStatus"),
			MEMSPowerStatus:          boolValue(payload.Values, "MEMS_power_status", "MEMSPowerStatus"),
			StarTrackerlPowerStatus:  boolValue(payload.Values, "StarTrackerl_power_status", "StarTracker_power_status", "StarTrackerlPowerStatus"),
			MomentumWheelPowerStatus: boolValue(payload.Values, "MomentumWheel_power_status", "MomentumWheelPowerStatus"),
		}, nil
	case "thermal":
		var temps [10]float64
		for i := range temps {
			temps[i] = floatValue(payload.Values, fmt.Sprintf("TMEZD%05dcjb_ThermalTemp", 1066+i), fmt.Sprintf("ThermalTemp%d", i+1))
		}
		return &model.ThermalMetrics{
			Timestamp:            payload.Timestamp,
			ThermalTemps:         temps,
			BatteryTemp1:         floatValue(payload.Values, "TMEZD01084_BatteryTemp1", "BatteryTemp1"),
			BatteryTemp2:         floatValue(payload.Values, "TMEZD01085_BatteryTemp2", "BatteryTemp2"),
			PlatformThermalTemp:  floatValue(payload.Values, "PlatformThermalTemp"),
			BatteryThermalTemp:   floatValue(payload.Values, "BatteryThermalTemp"),
			TankThermalTemp:      floatValue(payload.Values, "TankThermalTemp"),
			PlatformHeaterSwitch: boolValue(payload.Values, "TMEZD01121_PlatformHeater", "PlatformHeaterSwitch"),
			BatteryHeaterSwitch:  boolValue(payload.Values, "TMEZD01254_BatteryHeater", "BatteryHeaterSwitch"),
			TankHeaterSwitch:     boolValue(payload.Values, "TMEZD01115_TankHeater", "TankHeaterSwitch"),
		}, nil
	case "comm":
		return &model.CommMetrics{
			Timestamp:                           payload.Timestamp,
			CANStatus:                           uint8Value(payload.Values, "CANStatus"),
			SerialStatus:                        uint8Value(payload.Values, "SerialStatus", "Com_ExpectedState"),
			AirToAirStatus:                      uint8Value(payload.Values, "AirToAirStatus"),
			SNR:                                 uint8Value(payload.Values, "TMEZD01145_SNR", "SNR"),
			Rate:                                uint16Value(payload.Values, "Rate"),
			ParityErrorCount:                    uint16Value(payload.Values, "TMEZD01046_CheckErrorCount", "ParityErrorCount"),
			FrameHeaderErrorCount:               uint16Value(payload.Values, "TMEZD01047_FrameHeaderErrorCount", "FrameHeaderErrorCount"),
			FrameLengthErrorCount:               uint16Value(payload.Values, "TMEZD01048_FrameLengthErrorCount", "FrameLengthErrorCount"),
			SerialResetCount:                    uint16Value(payload.Values, "TMEZD01052_ResetCount", "SerialResetCount"),
			ReceiveCmdCount:                     uint32Value(payload.Values, "TMEZD01004cjb_InstructionCount", "ReceiveCmdCount"),
			O1ReceiveDeviceResponseCount:        uint32Value(payload.Values, "O1_ReceiveDeviceResponseCount"),
			O1ReceiveTelemetryResponseCount:     uint32Value(payload.Values, "O1_ReceiveTelemetryResponseCount"),
			O1ReceiveRemoteControlResponseCount: uint32Value(payload.Values, "O1_ReceiveRemoteControlResponseCount"),
			NoTelemetryCount:                    uint32Value(payload.Values, "Com_No_telemetry_count", "NoTelemetryCount"),
			TransmitSwitch:                      uint8Value(payload.Values, "TMEZD01155_SwitchState", "TransmitSwitch"),
			ReceiveCTACount:                     uint32Value(payload.Values, "TMEZD01150_ReceiveCTACount", "ReceiveCTACount"),
			TelemetryEncryptStatus:              uint8Value(payload.Values, "TMEZD01167_TelemetryEncryptStatus"),
			TelecontrolEncryptStatus:            uint8Value(payload.Values, "TMEZD01168_TelemetryEncryptStatus", "TelecontrolEncryptStatus"),
		}, nil
	case "momentum_wheel", "actuator":
		return &model.ActuatorMetrics{
			Timestamp:             payload.Timestamp,
			SendCmdK53029:         uint8Value(payload.Values, "SendCmd_K53029"),
			WheelSpeedX:           int16Value(payload.Values, "WheelSpeedX"),
			WheelSpeedY:           int16Value(payload.Values, "WheelSpeedY"),
			WheelSpeedZ:           int16Value(payload.Values, "WheelSpeedZ"),
			NoTelemetryCount:      uint32Value(payload.Values, "MomentumWheel_No_telemetry_count", "NoTelemetryCount"),
			CheckErrorCount:       uint32Value(payload.Values, "TMEZD01013_CheckErrorCount", "CheckErrorCount"),
			FrameHeaderErrorCount: uint32Value(payload.Values, "TMEZD01014_FrameHeaderErrorCount", "FrameHeaderErrorCount"),
			FrameLengthErrorCount: uint32Value(payload.Values, "TMEZD01015_FrameLengthErrorCount", "FrameLengthErrorCount"),
			ResetCount:            uint32Value(payload.Values, "TMEZD01016_ResetCount", "ResetCount"),
		}, nil
	case "attitude_orbit_control", "attitudeorbitcontrol":
		return &model.AttitudeOrbitControlMetrics{Timestamp: payload.Timestamp, Values: payload.Values}, nil
	default:
		return nil, fmt.Errorf("unsupported component %q", payload.Component)
	}
}

func normalizeComponent(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func value(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func floatValue(m map[string]interface{}, keys ...string) float64 {
	f, _ := toFloat64(value(m, keys...))
	return f
}

func boolValue(m map[string]interface{}, keys ...string) bool {
	switch v := value(m, keys...).(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
		f, _ := strconv.ParseFloat(v, 64)
		return f != 0
	default:
		return false
	}
}

func uint8Value(m map[string]interface{}, keys ...string) uint8 {
	f, _ := toFloat64(value(m, keys...))
	return uint8(f)
}

func uint16Value(m map[string]interface{}, keys ...string) uint16 {
	f, _ := toFloat64(value(m, keys...))
	return uint16(f)
}

func uint32Value(m map[string]interface{}, keys ...string) uint32 {
	f, _ := toFloat64(value(m, keys...))
	return uint32(f)
}

func int16Value(m map[string]interface{}, keys ...string) int16 {
	f, _ := toFloat64(value(m, keys...))
	return int16(f)
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
