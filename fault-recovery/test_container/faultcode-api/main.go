package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/acoinfo/vsoa/protocol"
	"github.com/acoinfo/vsoa/server"
)

var protocolHeader = []byte{0xC0, 0x00, 0x00, 0x09, 0x50, 0x01}

const protocolTail byte = 0x00

var rawCommandTable = map[string]string{
	"K50166":                              "8008805554AA54AA",
	"K50011":                              "800880550AAA0AAA",
	"K50175":                              "8008805555AA55AA",
	"K50132":                              "8008805547AA47AA",
	"K50005":                              "8708875507AA07AA",
	"K50003":                              "8008805506AA06AA",
	"K50004":                              "8008805506550655",
	"K52519":                              "AF068A5581FF",
	"K55501":                              "95088A55810000AA",
	"K55502":                              "95088A55820000AA",
	"K50502":                              "35068A558383",
	"K50504":                              "35068A558585",
	"K52002":                              "8008805505550555",
	"K52130":                              "8008805546AA46AA",
	"K50164":                              "8008805552AA52AA",
	"K50001":                              "8008805505AA05AA",
	"K51001":                              "8408845500AA00AA",
	"K51002":                              "8408845500550055",
	"K51003":                              "8408845501AA01AA",
	"K51004":                              "8408845501550155",
	"K51005":                              "8408845502AA02AA",
	"K51006":                              "8408845502550255",
	"K51007":                              "8408845503AA03AA",
	"K51008":                              "8408845503550355",
	"K53029":                              "8408845511041104",
	"K500038":                             "8008805517551755",
	"K500037":                             "8008805517AA17AA",
	"K50032":                              "8008805514551450",
	"restart_ad":                          "8008805554AA54AA",
	"platform_heating_belt_switch_on":     "800880550AAA0AAA",
	"restart_oc":                          "8008805555AA55AA",
	"battery_heating_belt_switch_on":      "8008805547AA47AA",
	"tank_heating_belt_switch_on":         "8708875507AA07AA",
	"power_communication_device":          "8008805506AA06AA",
	"turn_off_power_communication_device": "8008805506550655",
	"communicator_transmission_channel_opened": "AF068A5581FF",
	"communicator_receives_attenuation_0dB":    "95088A55810000AA",
	"communicator_transmit_attenuation_0dB":    "95088A55820000AA",
	"communicator_telemetry_secret_mode":       "35068A558383",
	"communicator_remote_secret_mode":          "35068A558585",
	"gnssa_off":                                "8008805505550555",
	"gnssb_on":                                 "8008805546AA46AA",
	"switch_to_gnssb":                          "8008805552AA52AA",
	"gnssa_power_on":                           "8008805505AA05AA",
	"power_gyroscope":                          "8408845500AA00AA",
	"power_off_gyroscope":                      "8408845500550055",
	"power_starsensors":                        "8408845502AA02AA",
	"power_off_starsensors":                    "8408845502550255",
	"power_momentumwheel":                      "8408845503AA03AA",
	"power_off_momentumwheel":                  "8408845503550355",
	"flywheel_test_100_revolutions_start":      "8408845511041104",
	"power_mems":                               "8408845501AA01AA",
	"power_off_mems":                           "8408845501550155",
}

type Command struct {
	Name           string `json:"name"`
	InstructionHex string `json:"instruction_hex"`
	FaultCode      string `json:"fault_code"`
}

type Decision struct {
	Allow     bool               `json:"allow"`
	Reason    string             `json:"reason"`
	Metrics   map[string]float64 `json:"metrics,omitempty"`
	CheckedAt time.Time          `json:"checked_at"`
}

type dispatchRequest struct {
	CommandName string             `json:"command_name,omitempty"`
	FaultCode   string             `json:"fault_code,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	Send        bool               `json:"send"`
}

type dispatchResponse struct {
	Command  Command  `json:"command"`
	Decision Decision `json:"decision"`
	FrameHex string   `json:"frame_hex"`
	Sent     bool     `json:"sent"`
	Bytes    int      `json:"bytes"`
	Port     string   `json:"port"`
}

type evaluateRequest struct {
	CommandName string             `json:"command_name,omitempty"`
	FaultCode   string             `json:"fault_code,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
}

type generateFaultCodeRequest struct {
	InstructionHex string `json:"instruction_hex"`
}

type Service struct {
	portPath            string
	commandsByName      map[string]Command
	commandsByFaultCode map[string]Command
	orderedCommands     []Command

	metricsMu     sync.RWMutex
	cachedMetrics map[string]float64
}

func newService(portPath string) (*Service, error) {
	byName := make(map[string]Command, len(rawCommandTable))
	byFault := make(map[string]Command, len(rawCommandTable))

	for name, rawHex := range rawCommandTable {
		normalizedHex, err := normalizeHex(rawHex)
		if err != nil {
			return nil, fmt.Errorf("invalid command hex for %s: %w", name, err)
		}

		faultCode := generateFaultCode(normalizedHex)

		cmd := Command{
			Name:           name,
			InstructionHex: normalizedHex,
			FaultCode:      faultCode,
		}
		byName[name] = cmd
		if _, ok := byFault[faultCode]; !ok {
			byFault[faultCode] = cmd
		}
	}

	ordered := make([]Command, 0, len(byName))
	for _, cmd := range byName {
		ordered = append(ordered, cmd)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Name < ordered[j].Name
	})

	return &Service{
		portPath:            portPath,
		commandsByName:      byName,
		commandsByFaultCode: byFault,
		orderedCommands:     ordered,
		cachedMetrics:       make(map[string]float64),
	}, nil
}

type queryByFaultCodeRequest struct {
	FaultCode string `json:"fault_code"`
}

type metricsUpdateRequest struct {
	Metrics map[string]float64 `json:"metrics"`
}

func (s *Service) registerRPC(rpcServer *server.Server) error {
	routes := []struct {
		path    string
		method  protocol.RpcMessageType
		handler func(req *protocol.Message, res *protocol.Message)
	}{
		{path: "/health", method: protocol.RpcMethodGet, handler: s.rpcHealth},
		{path: "/v1/commands", method: protocol.RpcMethodGet, handler: s.rpcCommands},
		{path: "/v1/commands/by-fault", method: protocol.RpcMethodGet, handler: s.rpcCommandByFault},
		{path: "/v1/evaluate", method: protocol.RpcMethodSet, handler: s.rpcEvaluate},
		{path: "/v1/dispatch", method: protocol.RpcMethodSet, handler: s.rpcDispatch},
		{path: "/v1/fault-codes/generate", method: protocol.RpcMethodSet, handler: s.rpcGenerateFaultCode},
		{path: "/v1/metrics", method: protocol.RpcMethodGet, handler: s.rpcGetMetrics},
		{path: "/v1/metrics", method: protocol.RpcMethodSet, handler: s.rpcUpdateMetrics},
	}

	for _, route := range routes {
		if err := rpcServer.On(route.path, route.method, route.handler); err != nil {
			return fmt.Errorf("register rpc route failed: %s (%v): %w", route.path, route.method, err)
		}
	}
	return nil
}

func (s *Service) rpcHealth(_ *protocol.Message, res *protocol.Message) {
	writeRPCOK(res, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
		"mode":   "vsoa-rpc",
	})
}

func (s *Service) rpcCommands(_ *protocol.Message, res *protocol.Message) {
	writeRPCOK(res, map[string]any{
		"count":    len(s.orderedCommands),
		"commands": s.orderedCommands,
		"metrics":  s.snapshotMetrics(),
	})
}

func (s *Service) rpcCommandByFault(req *protocol.Message, res *protocol.Message) {
	var query queryByFaultCodeRequest
	if err := decodeRPCParam(req, &query, false); err != nil {
		writeRPCError(res, err)
		return
	}

	faultCode := strings.ToUpper(strings.TrimSpace(query.FaultCode))
	if faultCode == "" {
		writeRPCError(res, errors.New("fault_code is required"))
		return
	}

	cmd, ok := s.commandsByFaultCode[faultCode]
	if !ok {
		writeRPCError(res, errors.New("fault_code not found"))
		return
	}

	writeRPCOK(res, cmd)
}

func (s *Service) rpcDispatch(req *protocol.Message, res *protocol.Message) {
	var body dispatchRequest
	if err := decodeRPCParam(req, &body, false); err != nil {
		writeRPCError(res, err)
		return
	}

	cmd, err := s.resolveCommand(body.CommandName, body.FaultCode)
	if err != nil {
		writeRPCError(res, err)
		return
	}

	metrics := s.mergeMetrics(body.Metrics)
	decision := evaluateMetrics(cmd.Name, metrics)
	frame, err := buildFrame(cmd.InstructionHex)
	if err != nil {
		writeRPCError(res, err)
		return
	}

	resp := dispatchResponse{
		Command:  cmd,
		Decision: decision,
		FrameHex: strings.ToUpper(hex.EncodeToString(frame)),
		Sent:     false,
		Bytes:    0,
		Port:     s.portPath,
	}

	if !decision.Allow {
		writeRPCOK(res, resp)
		return
	}

	if body.Send {
		n, err := writeFrame(s.portPath, frame)
		if err != nil {
			writeRPCError(res, err)
			return
		}
		resp.Sent = true
		resp.Bytes = n
	}

	writeRPCOK(res, resp)
}

func (s *Service) rpcEvaluate(req *protocol.Message, res *protocol.Message) {
	var body evaluateRequest
	if err := decodeRPCParam(req, &body, false); err != nil {
		writeRPCError(res, err)
		return
	}

	cmd, err := s.resolveCommand(body.CommandName, body.FaultCode)
	if err != nil {
		writeRPCError(res, err)
		return
	}

	metrics := s.mergeMetrics(body.Metrics)
	writeRPCOK(res, map[string]any{
		"command":  cmd,
		"decision": evaluateMetrics(cmd.Name, metrics),
	})
}

func (s *Service) rpcGenerateFaultCode(req *protocol.Message, res *protocol.Message) {
	var body generateFaultCodeRequest
	if err := decodeRPCParam(req, &body, false); err != nil {
		writeRPCError(res, err)
		return
	}

	normalizedHex, err := normalizeHex(body.InstructionHex)
	if err != nil {
		writeRPCError(res, err)
		return
	}

	writeRPCOK(res, map[string]any{
		"instruction_hex": normalizedHex,
		"fault_code":      generateFaultCode(normalizedHex),
	})
}

func (s *Service) rpcUpdateMetrics(req *protocol.Message, res *protocol.Message) {
	var body metricsUpdateRequest
	if err := decodeRPCParam(req, &body, false); err != nil {
		writeRPCError(res, err)
		return
	}

	s.upsertMetrics(body.Metrics)
	writeRPCOK(res, map[string]any{
		"metrics": s.snapshotMetrics(),
	})
}

func (s *Service) rpcGetMetrics(_ *protocol.Message, res *protocol.Message) {
	writeRPCOK(res, map[string]any{
		"metrics": s.snapshotMetrics(),
	})
}

func (s *Service) resolveCommand(commandName, faultCode string) (Command, error) {
	if strings.TrimSpace(commandName) == "" && strings.TrimSpace(faultCode) == "" {
		return Command{}, errors.New("command_name or fault_code is required")
	}

	if strings.TrimSpace(commandName) != "" {
		cmd, ok := s.commandsByName[commandName]
		if !ok {
			return Command{}, fmt.Errorf("command_name not found: %s", commandName)
		}
		if strings.TrimSpace(faultCode) != "" {
			if cmd.FaultCode != strings.ToUpper(strings.TrimSpace(faultCode)) {
				return Command{}, errors.New("command_name and fault_code mismatch")
			}
		}
		return cmd, nil
	}

	fc := strings.ToUpper(strings.TrimSpace(faultCode))
	cmd, ok := s.commandsByFaultCode[fc]
	if !ok {
		return Command{}, fmt.Errorf("fault_code not found: %s", fc)
	}
	return cmd, nil
}

func (s *Service) upsertMetrics(metrics map[string]float64) {
	if len(metrics) == 0 {
		return
	}

	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	for key, value := range metrics {
		s.cachedMetrics[key] = value
	}
}

func (s *Service) snapshotMetrics() map[string]float64 {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()

	copyMetrics := make(map[string]float64, len(s.cachedMetrics))
	for key, value := range s.cachedMetrics {
		copyMetrics[key] = value
	}
	return copyMetrics
}

func (s *Service) mergeMetrics(requestMetrics map[string]float64) map[string]float64 {
	merged := s.snapshotMetrics()
	for key, value := range requestMetrics {
		merged[key] = value
	}
	return merged
}

func evaluateMetrics(commandName string, metrics map[string]float64) Decision {
	decision := Decision{
		Allow:     true,
		Reason:    "metrics accepted",
		Metrics:   metrics,
		CheckedAt: time.Now().UTC(),
	}

	if len(metrics) == 0 {
		decision.Reason = "no metrics provided; reserved policy path"
		return decision
	}

	reasons := make([]string, 0, 3)
	if block, ok := metrics["block_dispatch"]; ok && block > 0 {
		decision.Allow = false
		reasons = append(reasons, "block_dispatch > 0")
	}
	if temperature, ok := metrics["temperature_c"]; ok && temperature > 85 {
		decision.Allow = false
		reasons = append(reasons, "temperature_c > 85")
	}
	if batteryV, ok := metrics["battery_voltage_v"]; ok && batteryV < 20 {
		decision.Allow = false
		reasons = append(reasons, "battery_voltage_v < 20")
	}
	if commandName == "flywheel_test_100_revolutions_start" {
		if wheelReady, ok := metrics["flywheel_ready"]; ok && wheelReady < 1 {
			decision.Allow = false
			reasons = append(reasons, "flywheel_ready < 1")
		}
	}

	if len(reasons) > 0 {
		decision.Reason = strings.Join(reasons, "; ")
	}

	return decision
}

func buildFrame(commandHex string) ([]byte, error) {
	commandBytes, err := hex.DecodeString(commandHex)
	if err != nil {
		return nil, fmt.Errorf("decode command hex failed: %w", err)
	}

	frame := make([]byte, 0, len(protocolHeader)+len(commandBytes)+1)
	frame = append(frame, protocolHeader...)
	frame = append(frame, commandBytes...)
	frame = append(frame, protocolTail)
	return frame, nil
}

func writeFrame(portPath string, frame []byte) (int, error) {
	f, err := os.OpenFile(portPath, os.O_RDWR|os.O_SYNC, 0o666)
	if err != nil {
		return 0, fmt.Errorf("open serial failed: %w", err)
	}
	defer f.Close()

	n, err := f.Write(frame)
	if err != nil {
		return 0, fmt.Errorf("write frame failed: %w", err)
	}

	if err := f.Sync(); err != nil {
		return n, fmt.Errorf("sync frame failed: %w", err)
	}

	return n, nil
}

func generateFaultCode(commandHex string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(commandHex))
	return fmt.Sprintf("FC%016X", h.Sum64())
}

func normalizeHex(raw string) (string, error) {
	normalized := strings.ToUpper(strings.Join(strings.Fields(raw), ""))
	if normalized == "" {
		return "", errors.New("hex string is empty")
	}
	if len(normalized)%2 != 0 {
		return "", errors.New("hex string length must be even")
	}
	if _, err := hex.DecodeString(normalized); err != nil {
		return "", fmt.Errorf("invalid hex string: %w", err)
	}
	return normalized, nil
}

func decodeJSONBytes(raw []byte, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(raw), 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode request failed: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func decodeRPCParam(req *protocol.Message, dst any, allowEmpty bool) error {
	if req == nil {
		return errors.New("request message is nil")
	}

	raw := bytes.TrimSpace(req.Param)
	if len(raw) == 0 {
		if allowEmpty {
			return nil
		}
		return errors.New("param is required")
	}

	return decodeJSONBytes(raw, dst)
}

func writeRPC(res *protocol.Message, payload any) {
	if res == nil {
		return
	}

	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal rpc response failed: %v", err)
		res.Param = json.RawMessage(`{"ok":false,"error":"marshal response failed"}`)
		res.Data = nil
		return
	}

	res.Param = b
	res.Data = nil
}

func writeRPCOK(res *protocol.Message, data any) {
	writeRPC(res, map[string]any{
		"ok":   true,
		"data": data,
	})
}

func writeRPCError(res *protocol.Message, err error) {
	writeRPC(res, map[string]any{
		"ok":    false,
		"error": err.Error(),
	})
}

func main() {
	portPath := os.Getenv("RS422_PORT")
	if portPath == "" {
		portPath = "/dev/ttyS2"
	}

	vsoaAddr := os.Getenv("VSOA_ADDR")
	if vsoaAddr == "" {
		vsoaAddr = "0.0.0.0:3001"
	}

	vsoaName := os.Getenv("VSOA_NAME")
	if vsoaName == "" {
		vsoaName = "faultcode-vsoa-rpc"
	}

	vsoaPassword := os.Getenv("VSOA_PASSWORD")

	service, err := newService(portPath)
	if err != nil {
		log.Fatalf("init service failed: %v", err)
	}

	option := server.Option{}
	if strings.TrimSpace(vsoaPassword) != "" {
		option.Password = vsoaPassword
	}

	rpcServer := server.NewServer(vsoaName, option)
	if rpcServer == nil {
		log.Fatal("create vsoa server failed")
	}

	if err := service.registerRPC(rpcServer); err != nil {
		log.Fatalf("register rpc handlers failed: %v", err)
	}

	log.Printf("fault-code VSOA RPC service started on %s, serial port %s", vsoaAddr, portPath)
	if err := rpcServer.Serve(vsoaAddr); err != nil {
		log.Fatalf("vsoa server failed: %v", err)
	}
}
