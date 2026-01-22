package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"health-monitor/pkg/microservice"
	"golang.org/x/crypto/ssh"
)

// =================================================================================
// 接口定义
// =================================================================================

// Action 执行修复动作
type Action interface {
	Name() string
	Execute(ctx context.Context, event DiagnosisResult) error
	Verify(ctx context.Context, event DiagnosisResult) error
}

// Resolver 定义了解除故障的能力（用于恢复）
type Resolver interface {
	Resolve(ctx context.Context, event DiagnosisResult) error
}

// =================================================================================
// 运行时存储 (模拟控制面状态)
// =================================================================================

type RuntimeStore struct {
	mu           sync.RWMutex
	breakers     map[string]bool
	containers   map[string]bool
	containerImg map[string]string
	services     map[string]string
}

func NewRuntimeStore() *RuntimeStore {
	return &RuntimeStore{
		breakers:     make(map[string]bool),
		containers:   make(map[string]bool),
		containerImg: make(map[string]string),
		services:     make(map[string]string),
	}
}

func (s *RuntimeStore) SetBreaker(targetID string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.breakers[targetID] = enabled
}

func (s *RuntimeStore) IsBreakerEnabled(targetID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.breakers[targetID]
}

func (s *RuntimeStore) StartContainer(targetID string, image string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.containers[targetID] = true
	if image != "" {
		s.containerImg[targetID] = image
	}
}

func (s *RuntimeStore) IsContainerRunning(targetID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containers[targetID]
}

func (s *RuntimeStore) SetServiceID(targetID string, serviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if serviceID == "" {
		return
	}
	s.services[targetID] = serviceID
}

func (s *RuntimeStore) GetServiceID(targetID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.services[targetID]
}

func (s *RuntimeStore) ClearServiceID(targetID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.services, targetID)
}

// =================================================================================
// CircuitBreakerAction: 熔断/限流动作实现
// 策略: 使用 SylixOS netfilter (防火墙) 丢弃特定端口流量
// =================================================================================

type CircuitBreakerAction struct {
	store   *RuntimeStore
	baseURL string
	client  *http.Client
}

func NewCircuitBreakerAction(store *RuntimeStore) *CircuitBreakerAction {
	baseURL := os.Getenv("RECOVERY_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://192.168.31.127:3001"
	}

	return &CircuitBreakerAction{
		store:   store,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *CircuitBreakerAction) Name() string { return "circuit_breaker" }

// Execute 执行熔断：阻断端口流量
func (a *CircuitBreakerAction) Execute(ctx context.Context, event DiagnosisResult) error {
	sourceID := DiagnosisTargetID(event)
	if sourceID == "" {
		return errors.New("empty targetID")
	}

	// 1. 获取目标 IP 和 端口
	containerIP, err := a.resolveTargetIP(ctx, event, sourceID)
	if err != nil {
		return err
	}

	// 2. 调用 NetFilter 执行熔断 (Block = true)
	// 使用 netfilter DROP 掉该端口的包，比 flowctl 更安全，不会误杀 Host IP
	if err := a.runNetFilter(ctx, containerIP, true); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		a.store.SetBreaker(sourceID, true)
		return nil
	}
}

// Resolve 解除熔断：恢复端口流量
func (a *CircuitBreakerAction) Resolve(ctx context.Context, event DiagnosisResult) error {
	sourceID := DiagnosisTargetID(event)
	if sourceID == "" {
		return errors.New("empty targetID")
	}
	fmt.Printf("开始恢复\n")
	// 1. 获取目标 IP 和 端口
	containerIP, err := a.resolveTargetIP(ctx, event, sourceID)
	if err != nil {
		return err
	}

	// 2. 调用 NetFilter 解除熔断 (Block = false)
	if err := a.runNetFilter(ctx, containerIP, false); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		a.store.SetBreaker(sourceID, false)
		return nil
	}
}

func (a *CircuitBreakerAction) Verify(ctx context.Context, event DiagnosisResult) error {
	targetID := DiagnosisTargetID(event)
	status := DiagnosisStatus(event)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		isEnabled := a.store.IsBreakerEnabled(targetID)
		
		if status == EventStatusResolved {
			// 如果事件已解决，熔断器应该处于关闭状态
			if isEnabled {
				return fmt.Errorf("breaker still enabled for %s after resolve", targetID)
			}
			return nil
		}
		
		// 如果正在熔断中
		if !isEnabled {
			return fmt.Errorf("breaker not enabled for %s", targetID)
		}
		return nil
	}
}

// resolveTargetIP 辅助函数：提取元数据并查询 IP
func (a *CircuitBreakerAction) resolveTargetIP(ctx context.Context, event DiagnosisResult, sourceID string) (string, error) {
	serviceName := ""
	serviceID := ""
	if event.Metadata != nil {
		if v, ok := event.Metadata["serviceName"].(string); ok {
			serviceName = v
		}
		if v, ok := event.Metadata["serviceId"].(string); ok {
			serviceID = v
		}
	}
	if serviceName == "" {
		return "", fmt.Errorf("empty service name for container %s", sourceID)
	}
	if serviceID != "" && serviceName != serviceID {
		fmt.Printf("serviceName/serviceId mismatch: serviceName=%s serviceId=%s\n", serviceName, serviceID)
	}

	return a.getContainerIPByService(ctx, serviceName, sourceID)
}

// runNetFilter 使用 SSH 调用 SylixOS 的防火墙命令
// block: true 表示熔断(DROP), false 表示恢复(DELETE rule)
func (a *CircuitBreakerAction) runNetFilter(ctx context.Context, containerAddr string, block bool) error {
	if containerAddr == "" {
		return errors.New("empty container address")
	}

	// 检查是否禁用 (用于调试)
	if getenvOrDefault("NETFILTER_DISABLED", "") == "1" {
		fmt.Printf("netfilter disabled, skip action for %s\n", containerAddr)
		return nil
	}

	// 1. 解析端口 (防火墙主要靠端口区分)
	host, port, err := net.SplitHostPort(containerAddr)
	if err != nil {
		return fmt.Errorf("invalid container address: %s", containerAddr)
	}

	// 【安全检查】绝对禁止封锁管理端口
	// 在 Host 模式下，封锁 22 或 3001 会导致失联
	if port == "22" || port == "3001" || port == "8080" {
		return fmt.Errorf("SECURITY ALERT: blocking management port %s is prohibited", port)
	}

	// 2. 构造命令列表
	var cmds []string

	if block {
		// ============ 熔断模式 (DROP) ============
		
		// 【步骤 A】白名单保命 (至关重要)
		// 显式允许 SSH 和 Agent 端口，防止防火墙默认策略变动导致断联
		// 注意：SylixOS netfilter 也是顺序匹配，匹配到即停止。所以 ACCEPT 必须在 DROP 前面。
		cmds = append(cmds, "netfilter -A INPUT -p tcp --dport 22 -j ACCEPT")
		cmds = append(cmds, "netfilter -A INPUT -p tcp --dport 3001 -j ACCEPT")
		// 如果有其他关键端口（如 DNS 53），也可以加在这里
		
		// 【步骤 B】阻断业务端口
		// 1. INPUT: 拦截外部/其他容器访问该端口
		cmds = append(cmds, fmt.Sprintf("netfilter -A INPUT -p tcp --dport %s -j DROP", port))
		cmds = append(cmds, fmt.Sprintf("netfilter -A INPUT -p udp --dport %s -j DROP", port))
		

	} else {
		// ============ 恢复模式 (DELETE) ============
		
		// 1. 删除业务端口的阻断规则 (INPUT & OUTPUT)
		cmds = append(cmds, fmt.Sprintf("netfilter -D INPUT -p tcp --dport %s -j DROP", port))
		cmds = append(cmds, fmt.Sprintf("netfilter -D INPUT -p udp --dport %s -j DROP", port))

		// 2. 清理白名单规则 (可选，为了保持规则表干净)
		cmds = append(cmds, "netfilter -D INPUT -p tcp --dport 22 -j ACCEPT")
		cmds = append(cmds, "netfilter -D INPUT -p tcp --dport 3001 -j ACCEPT")
	}

	// 3. 拼接命令
	// 使用分号 ; 连接所有命令
	fullCmd := ""
	for i, c := range cmds {
		if i == 0 {
			fullCmd = c
		} else {
			fullCmd = fullCmd + " ; " + c
		}
	}

	opName := "熔断(DROP)"
	if !block {
		opName = "恢复(ACCEPT)"
	}
	fmt.Printf("NetFilter 准备执行 [%s]: %s\n", opName, fullCmd)

	// 4. 通过 SSH 执行
	sshUser := getenvOrDefault("SSH_USER", "root")
	sshPass := getenvOrDefault("SSH_PASS", "root")
	// 默认连接本机 SSH (Loopback 地址通常最稳定，不受 eth0 规则影响)
	sshHost := getenvOrDefault("SSH_HOST", "127.0.0.1:22")

	// 简单的环境检查提示
	if host != "127.0.0.1" && host != "localhost" && sshHost == "127.0.0.1:22" {
		// Log: controlling host network via localhost ssh
	}

	output, err := executeRemoteCommand(sshHost, sshUser, sshPass, fullCmd)
	if err != nil {
		// 特殊处理：如果是“恢复”操作(-D)，且规则本来就不存在，netfilter 可能会报错 "No such rule"
		// 这种情况下我们应该忽略错误，认为成功
		if !block {
			fmt.Printf("恢复操作收到警告(通常可忽略): %v output=%s\n", err, output)
			return nil
		}
		return fmt.Errorf("netfilter failed: %w output=%s", err, output)
	}

	fmt.Printf("NetFilter 执行成功: %s\n", output)
	return nil
}

// executeRemoteCommand 建立 SSH 连接并执行命令 (支持 PTY)
func executeRemoteCommand(addr, user, password, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 忽略 HostKey 检查
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// 【关键】请求伪终端 (PTY)
	// SylixOS 内核命令通常需要交互式环境才能加载和执行
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // 禁止回显
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		return "", fmt.Errorf("request pty failed: %w", err)
	}

	var b bytes.Buffer
	session.Stdout = &b
	session.Stderr = &b

	// 执行命令
	if err := session.Run(cmd); err != nil {
		return b.String(), err
	}

	return b.String(), nil
}

// =================================================================================
// 辅助函数与 API 模型
// =================================================================================

type instanceListResponse struct {
	Status  int                 `json:"status"`
	Message string              `json:"message"`
	Data    serviceInstanceList `json:"data"`
}

type serviceInstanceList struct {
	List     []serviceInstance `json:"list"`
	Total    int               `json:"total"`
	PageNum  int               `json:"pageNum"`
	PageSize int               `json:"pageSize"`
}

type serviceInstance struct {
	ID       string `json:"id"`
	TaskID   string `json:"taskId"`
	IP       string `json:"ip"`
	VSOAPort int    `json:"vsoaPort"`
}

func (a *CircuitBreakerAction) getContainerIPByService(ctx context.Context, serviceName, containerID string) (string, error) {
	fmt.Printf("正在获取容器IP: Service=%s ContainerID=%s\n", serviceName, containerID)
	pageNum := 1
	pageSize := 50

	for {
		u, err := url.Parse(a.baseURL + "/api/v1/micro-service/instance")
		if err != nil {
			return "", err
		}
		q := u.Query()
		q.Set("pageNum", fmt.Sprintf("%d", pageNum))
		q.Set("pageSize", fmt.Sprintf("%d", pageSize))
		q.Set("id", serviceName)
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return "", err
		}

		resp, err := a.client.Do(req)
		if err != nil {
			return "", err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("instance list http status=%d body=%s", resp.StatusCode, string(body))
		}

		var result instanceListResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}
		if result.Status != 200 {
			// 某些API设计非200也是成功，需根据实际情况调整
			fmt.Printf("API Response: %v\n", result)
			if result.Status != 0 && result.Status != 200 { // 假设0或200为成功
				return "", fmt.Errorf("instance list api error status=%d msg=%s", result.Status, result.Message)
			}
		}

		for _, item := range result.Data.List {
			if item.TaskID == containerID || item.ID == containerID {
				if item.IP == "" || item.VSOAPort == 0 {
					return "", fmt.Errorf("invalid instance address for container=%s service=%s", containerID, serviceName)
				}
				return fmt.Sprintf("%s:%d", item.IP, item.VSOAPort), nil
			}
		}

		if len(result.Data.List) == 0 || pageNum*pageSize >= result.Data.Total {
			break
		}
		pageNum++
	}

	return "", fmt.Errorf("container ip not found for container=%s service=%s", containerID, serviceName)
}

func getenvOrDefault(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func getMetaString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	if v, ok := meta[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getMetaStringSlice(meta map[string]interface{}, key string) []string {
	if meta == nil {
		return nil
	}
	if v, ok := meta[key]; ok {
		switch vv := v.(type) {
		case []string:
			return vv
		case []interface{}:
			out := make([]string, 0, len(vv))
			for _, item := range vv {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func getMetaInt(meta map[string]interface{}, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	if v, ok := meta[key]; ok {
		switch vv := v.(type) {
		case int:
			return vv, true
		case int32:
			return int(vv), true
		case int64:
			return int(vv), true
		case float64:
			return int(vv), true
		}
	}
	return 0, false
}

func getMetaBool(meta map[string]interface{}, key string) (bool, bool) {
	if meta == nil {
		return false, false
	}
	if v, ok := meta[key]; ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}

func decodeEcsImageConfig(raw interface{}) (*microservice.EcsImageConfig, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var cfg microservice.EcsImageConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func decodeImageVSOA(raw interface{}) (*microservice.ImageVSOA, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var cfg microservice.ImageVSOA
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type RecoveryServiceConfig struct {
	FaultCodes   map[string]ServiceCreatePreset `json:"fault_codes"`
	EventPresets map[string]ServiceCreatePreset `json:"event_presets"`
}

type ServiceCreatePreset struct {
	Name   string                `json:"name"`
	Image  ServiceImagePreset    `json:"image"`
	Node   microservice.NodeSpec `json:"node"`
	Factor *int                  `json:"factor,omitempty"`
	Policy string                `json:"policy,omitempty"`
	Prepull *bool                `json:"prepull,omitempty"`
}

type ServiceImagePreset struct {
	Ref         string                    `json:"ref"`
	Action      string                    `json:"action"`
	Config      *microservice.EcsImageConfig `json:"config"`
	VSOA        *microservice.ImageVSOA   `json:"vsoa,omitempty"`
	PullPolicy  string                    `json:"pullPolicy,omitempty"`
	AutoUpgrade string                    `json:"autoUpgrade,omitempty"`
}

func LoadRecoveryServiceConfig(path string) (*RecoveryServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg RecoveryServiceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (a *StartContainerAction) selectPreset(event DiagnosisResult) *ServiceCreatePreset {
	if a.config == nil {
		return nil
	}
	// 优先按触发路径/基本事件匹配
	for _, id := range event.TriggerPath {
		if p, ok := a.config.EventPresets[id]; ok {
			return &p
		}
	}
	for _, id := range event.BasicEvents {
		if p, ok := a.config.EventPresets[id]; ok {
			return &p
		}
	}
	// 兜底按故障码匹配
	if p, ok := a.config.FaultCodes[event.FaultCode]; ok {
		return &p
	}
	return nil
}

// =================================================================================
// StartContainerAction (保持不变)
// =================================================================================

type StartContainerAction struct {
	store *RuntimeStore
	baseURL string
	client  *http.Client
	config  *RecoveryServiceConfig
}

func NewStartContainerAction(store *RuntimeStore) *StartContainerAction {
	baseURL := os.Getenv("RECOVERY_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://192.168.31.127:3001"
	}
	config, err := loadRecoveryServiceConfigWithFallback()
	if err != nil {
		fmt.Printf("[recovery] load service config failed: %v\n", err)
	}

	return &StartContainerAction{
		store:  store,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		config:  config,
	}
}

func (a *StartContainerAction) Name() string { return "start_container" }

func (a *StartContainerAction) Resolve(ctx context.Context, event DiagnosisResult) error {
	serviceID := a.store.GetServiceID(DiagnosisTargetID(event))
	if serviceID == "" {
		serviceID = getMetaString(event.Metadata, "serviceId")
	}
	if serviceID == "" {
		return errors.New("missing serviceId for delete")
	}
	if err := a.callServiceCommand(ctx, "destroy", []string{serviceID}); err != nil {
		return err
	}

	a.store.ClearServiceID(DiagnosisTargetID(event))
	return nil
}

func (a *StartContainerAction) callServiceCommand(ctx context.Context, cmd string, ids []string) error {
	if cmd == "" {
		return errors.New("empty service command")
	}
	if len(ids) == 0 {
		return errors.New("empty service ids")
	}

	payload, err := json.Marshal(map[string][]string{"ids": ids})
	if err != nil {
		return fmt.Errorf("marshal service command payload failed: %w", err)
	}
	url := fmt.Sprintf("%s/api/v1/service/%s/ids", a.baseURL, cmd)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service command %s http status=%d body=%s", cmd, resp.StatusCode, string(body))
	}

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		if result.Status != 0 && result.Status != 200 {
			return fmt.Errorf("service command %s api error status=%d msg=%s", cmd, result.Status, result.Message)
		}
	}
	return nil
}

func loadRecoveryServiceConfigWithFallback() (*RecoveryServiceConfig, error) {
	if path := os.Getenv("RECOVERY_SERVICE_CONFIG"); path != "" {
		return LoadRecoveryServiceConfig(path)
	}

	var candidates []string
	candidates = append(candidates,
		"fault-recovery/configs/recovery_service_config.json",
		"configs/recovery_service_config.json",
		"../fault-recovery/configs/recovery_service_config.json",
	)

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "recovery_service_config.json"),
			filepath.Join(exeDir, "configs", "recovery_service_config.json"),
			filepath.Join(exeDir, ".", "configs", "recovery_service_config.json"),
		)
	}

	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			lastErr = err
			continue
		}
		cfg, err := LoadRecoveryServiceConfig(path)
		if err != nil {
			lastErr = err
			continue
		}
		return cfg, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("service config not found")
	}
	return nil, lastErr
}

func (a *StartContainerAction) fetchAvailableNodeNames(ctx context.Context) ([]string, error) {
	fetcher := microservice.NewFetcher(a.baseURL)
	page, err := fetcher.ListNode(ctx, microservice.NodeListOptions{
		PageNum:  1,
		PageSize: -1,
	})
	if err != nil {
		return nil, err
	}

	var online []string
	var all []string
	for _, node := range page.Items {
		if node.Name == "" {
			continue
		}
		all = append(all, node.Name)
		if strings.EqualFold(node.Status, "online") {
			online = append(online, node.Name)
		}
	}
	if len(online) > 0 {
		return online, nil
	}
	return all, nil
}

func (a *StartContainerAction) Execute(ctx context.Context, event DiagnosisResult) error {
	fmt.Printf("启动镜像容器，%v\n", event)
	if DiagnosisTargetID(event) == "" {
		return errors.New("empty targetID")
	}

	// 允许从配置文件按子节点/故障码加载参数
	preset := a.selectPreset(event)
	if preset == nil {
		return errors.New("missing preset config")
	}

	serviceName := preset.Name
	if serviceName == "" {
		return errors.New("missing serviceName")
	}

	imageRef := preset.Image.Ref
	if imageRef == "" {
		return errors.New("missing imageRef")
	}

	imageAction := preset.Image.Action
	if imageAction == "" {
		imageAction = "run"
	}

	nodeNames := getMetaStringSlice(event.Metadata, "nodeNames")
	if len(nodeNames) == 0 {
		nodeNames = preset.Node.Names
	}
	if len(nodeNames) == 0 {
		list, err := a.fetchAvailableNodeNames(ctx)
		if err != nil {
			return fmt.Errorf("fetch node list failed: %w", err)
		}
		nodeNames = list
	}
	if len(nodeNames) == 0 {
		return errors.New("missing nodeNames")
	}
	if len(nodeNames) > 1 {
		for _, name := range nodeNames {
			if strings.EqualFold(name, "master") {
				nodeNames = []string{"master"}
				break
			}
		}
	}

	imageConfig := preset.Image.Config
	if imageConfig == nil {
		return errors.New("missing imageConfig")
	}

	imageSpec := microservice.ImageSpec{
		Ref:         imageRef,
		Action:      imageAction,
		Config:      imageConfig,
		PullPolicy:  preset.Image.PullPolicy,
		AutoUpgrade: preset.Image.AutoUpgrade,
	}
	if preset.Image.VSOA != nil {
		imageSpec.VSOA = preset.Image.VSOA
	}

	reqBody := struct {
		Name    string               `json:"name"`
		Image   microservice.ImageSpec `json:"image"`
		Node    microservice.NodeSpec  `json:"node"`
		Factor  *int                 `json:"factor,omitempty"`
		Policy  string               `json:"policy,omitempty"`
		Prepull *bool                `json:"prepull,omitempty"`
	}{
		Name:  serviceName,
		Image: imageSpec,
		Node:  microservice.NodeSpec{Names: nodeNames},
	}

	if preset.Factor != nil {
		reqBody.Factor = preset.Factor
	}
	if preset.Policy != "" {
		reqBody.Policy = preset.Policy
	}
	if preset.Prepull != nil {
		reqBody.Prepull = preset.Prepull
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal create service payload failed: %w", err)
	}

	url := a.baseURL + "/api/v1/service"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create service http status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		if result.Status != 0 && result.Status != 200 {
			return fmt.Errorf("create service api error status=%d msg=%s", result.Status, result.Message)
		}
		if result.Data.ID != "" {
			a.store.SetServiceID(DiagnosisTargetID(event), result.Data.ID)
		}
	}

	image := imageRef

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		a.store.StartContainer(DiagnosisTargetID(event), image)
		return nil
	}
}

func (a *StartContainerAction) Verify(ctx context.Context, event DiagnosisResult) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		if !a.store.IsContainerRunning(DiagnosisTargetID(event)) {
			return fmt.Errorf("container not running: %s", DiagnosisTargetID(event))
		}
		return nil
	}
}

// 辅助类型定义 (假设 DiagnosisResult 在其他文件定义，为了代码完整性补充在这里注释)
// 实际使用时请删除或注释掉这些 mock 定义，使用您项目中真实的定义
/*
const EventStatusResolved = "resolved"

type DiagnosisResult interface {
	// ...
}

func DiagnosisTargetID(event DiagnosisResult) string {
	// mock
	return "mock-id"
}

func DiagnosisStatus(event DiagnosisResult) string {
	// mock
	return "firing"
}
*/