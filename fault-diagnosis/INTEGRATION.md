# 故障诊断模块集成指南

## 与健康监测模块集成

### 1. 告警接收

健康监测模块和故障诊断模块通过消息队列进行通信。

#### 告警事件存储路径

```
/alerts/<alert_type>/<alert_id>
```

示例：
```
/alerts/business/BATTERY_VOLTAGE_ALERT
/alerts/microservice/SERVICE_P99_LATENCY_HIGH
```

#### 告警事件数据格式

```json
{
  "alert_id": "BATTERY_VOLTAGE_ALERT",
  "type": "voltage_abnormal",
  "severity": "warning",
  "source": "battery_monitor",
  "message": "蓄电池电压异常：23.5V (正常范围: 24V-28V)",
  "timestamp": 1702368000,
  "fault_code": "",
  "metric_value": 23.5,
  "related_alerts": [],
  "metadata": {
    "component": "battery",
    "threshold": "24-28V",
    "actual_value": "23.5V"
  }
}
```

### 2. 启动顺序

1. **启动 etcd 集群**
   ```bash
   etcd --listen-client-urls http://0.0.0.0:2379 \
        --advertise-client-urls http://localhost:2379
   ```

2. **启动健康监测模块**
   ```bash
   cd health-monitor
   ./build/health-monitor
   ```

3. **启动故障诊断模块**
   ```bash
   cd fault-diagnosis
   ./build/fault-diagnosis \
       -config ./configs/fault_tree_business.json \
       -etcd localhost:2379 \
       -prefix /alerts/
   ```

### 3. 告警ID映射

在故障树配置文件中，基本事件的 `alert_id` 必须与健康监测模块生成的告警ID一致。

#### 业务层告警ID映射表

| 基本事件ID | 告警ID | 描述 |
|-----------|--------|------|
| EVT-001 | BATTERY_VOLTAGE_ALERT | 蓄电池电压异常 |
| EVT-002 | BUS_VOLTAGE_ALERT | 母线电压异常 |
| EVT-003 | CPU_VOLTAGE_ALERT | CPU板电压异常 |

#### 微服务层告警ID映射表

| 基本事件ID | 告警ID | 描述 |
|-----------|--------|------|
| EVT-MS-001 | SERVICE_P99_LATENCY_HIGH | P99延迟过高 |
| EVT-MS-002 | SERVICE_ERROR_RATE_HIGH | 错误率过高 |
| EVT-MS-003 | CONTAINER_CPU_HIGH | 容器CPU使用率过高 |
| EVT-MS-004 | CONTAINER_MEMORY_HIGH | 容器内存使用率过高 |

### 4. 健康监测模块配置示例

在健康监测模块的告警生成器中配置：

```go
// pkg/alert/generator.go
func (g *Generator) GenerateAlert(metricName string, value float64) {
    var alertID string
    
    // 映射指标到告警ID
    switch metricName {
    case "battery_voltage":
        alertID = "BATTERY_VOLTAGE_ALERT"
    case "bus_voltage":
        alertID = "BUS_VOLTAGE_ALERT"
    case "cpu_voltage":
        alertID = "CPU_VOLTAGE_ALERT"
    case "service_p99_latency":
        alertID = "SERVICE_P99_LATENCY_HIGH"
    // ... 其他映射
    }
    
    alert := &model.AlertEvent{
        AlertID:   alertID,
        Type:      "threshold_exceeded",
        Severity:  g.determineSeverity(value),
        Source:    metricName,
        Message:   fmt.Sprintf("%s异常: %.2f", metricName, value),
        Timestamp: time.Now().Unix(),
        MetricValue: value,
    }
    
    // 写入etcd
    g.publishToEtcd(alert)
}
```

## 与故障修复模块集成

### 1. 诊断结果输出

故障诊断模块生成诊断结果后，需要传递给故障修复模块。

#### 方式1: 通过 etcd

```go
// cmd/diagnosis/main.go
func handleDiagnosisResult(diagnosis *models.DiagnosisResult, logger *zap.Logger) {
    // 序列化诊断结果
    data, _ := json.Marshal(diagnosis)
    
    // 写入etcd
    key := fmt.Sprintf("/diagnosis/%s", diagnosis.FaultCode)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    _, err := etcdClient.Put(ctx, key, string(data))
    if err != nil {
        logger.Error("写入诊断结果到etcd失败", zap.Error(err))
    }
}
```

#### 方式2: 通过消息队列

```go
// 使用Kafka、RabbitMQ等消息队列
func publishToKafka(diagnosis *models.DiagnosisResult) {
    message := &kafka.Message{
        Topic: "fault-diagnosis",
        Key:   []byte(diagnosis.FaultCode),
        Value: marshalJSON(diagnosis),
    }
    producer.Send(message)
}
```

#### 方式3: 通过HTTP API

```go
func postToRepairModule(diagnosis *models.DiagnosisResult) {
    url := "http://fault-repair-service/api/diagnosis"
    data, _ := json.Marshal(diagnosis)
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
    if err != nil {
        // 处理错误
    }
    defer resp.Body.Close()
}
```

### 2. 诊断结果数据格式

```json
{
  "diagnosis_id": "DIAG-20231212150405",
  "fault_tree_id": "business_battery_fault",
  "top_event_id": "TOP-001",
  "top_event_name": "CJB-RG-ZD-3",
  "fault_code": "CJB-RG-ZD-3",
  "fault_reason": "蓄电池、母线电压遥测异常",
  "timestamp": "2023-12-12T15:04:05Z",
  "trigger_path": ["TOP-001", "MID-001", "EVT-001", "EVT-002"],
  "basic_events": ["EVT-001", "EVT-002"],
  "metadata": {}
}
```

### 3. 故障修复模块监听

故障修复模块应监听诊断结果，并根据故障码执行相应的修复动作。

```go
// 故障修复模块示例
type RepairModule struct {
    etcdClient *clientv3.Client
}

func (r *RepairModule) Start() {
    watchChan := r.etcdClient.Watch(context.Background(), "/diagnosis/", clientv3.WithPrefix())
    
    for watchResp := range watchChan {
        for _, event := range watchResp.Events {
            if event.Type == clientv3.EventTypePut {
                var diagnosis models.DiagnosisResult
                json.Unmarshal(event.Kv.Value, &diagnosis)
                
                // 根据故障码执行修复动作
                r.executeRepair(diagnosis.FaultCode)
            }
        }
    }
}

func (r *RepairModule) executeRepair(faultCode string) {
    switch faultCode {
    case "CJB-RG-ZD-3":
        // 执行蓄电池异常的修复动作
        r.repairBatteryFault()
    case "SVC-PERF-001":
        // 重启服务或扩容
        r.restartService()
    // ... 其他故障码
    }
}
```

## 完整系统部署

### 1. Docker Compose 部署

```yaml
version: '3.8'

services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.11
    ports:
      - "2379:2379"
    command:
      - /usr/local/bin/etcd
      - --listen-client-urls=http://0.0.0.0:2379
      - --advertise-client-urls=http://etcd:2379

  health-monitor:
    build: ./health-monitor
    depends_on:
      - etcd
    environment:
      - ETCD_ENDPOINTS=etcd:2379
    volumes:
      - ./health-monitor/configs:/app/configs

  fault-diagnosis-business:
    build: ./fault-diagnosis
    depends_on:
      - etcd
      - health-monitor
    command:
      - /app/fault-diagnosis
      - -config=/app/configs/fault_tree_business.json
      - -etcd=etcd:2379
      - -prefix=/alerts/business/
    volumes:
      - ./fault-diagnosis/configs:/app/configs

  fault-diagnosis-microservice:
    build: ./fault-diagnosis
    depends_on:
      - etcd
      - health-monitor
    command:
      - /app/fault-diagnosis
      - -config=/app/configs/fault_tree_microservice.json
      - -etcd=etcd:2379
      - -prefix=/alerts/microservice/
    volumes:
      - ./fault-diagnosis/configs:/app/configs

  fault-repair:
    build: ./fault-repair
    depends_on:
      - etcd
      - fault-diagnosis-business
      - fault-diagnosis-microservice
    environment:
      - ETCD_ENDPOINTS=etcd:2379
```

### 2. Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fault-diagnosis
spec:
  replicas: 2
  selector:
    matchLabels:
      app: fault-diagnosis
  template:
    metadata:
      labels:
        app: fault-diagnosis
    spec:
      containers:
      - name: fault-diagnosis
        image: fault-diagnosis:latest
        args:
        - -config=/etc/fault-diagnosis/fault_tree.json
        - -etcd=$(ETCD_ENDPOINTS)
        - -prefix=/alerts/
        env:
        - name: ETCD_ENDPOINTS
          value: "etcd-service:2379"
        volumeMounts:
        - name: config
          mountPath: /etc/fault-diagnosis
      volumes:
      - name: config
        configMap:
          name: fault-tree-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: fault-tree-config
data:
  fault_tree.json: |
    {
      "fault_tree_id": "production_fault_tree",
      ...
    }
```

## 监控和调试

### 1. 日志级别配置

```bash
# 调试模式：输出详细日志
./fault-diagnosis -log-level debug

# 生产模式：仅输出重要日志
./fault-diagnosis -log-level info
```

### 2. 诊断结果输出到文件

```bash
./fault-diagnosis -output ./diagnosis-results.json
```

### 3. etcd 数据检查

```bash
# 查看所有告警
etcdctl get /alerts/ --prefix

# 查看诊断结果
etcdctl get /diagnosis/ --prefix

# 监听告警事件
etcdctl watch /alerts/ --prefix
```

### 4. 测试告警注入

使用 etcdctl 手动注入测试告警：

```bash
# 注入蓄电池电压异常告警
etcdctl put /alerts/business/BATTERY_VOLTAGE_ALERT '{
  "alert_id": "BATTERY_VOLTAGE_ALERT",
  "type": "voltage_abnormal",
  "severity": "warning",
  "source": "battery_monitor",
  "message": "蓄电池电压异常",
  "timestamp": 1702368000,
  "metric_value": 23.5
}'

# 注入母线电压异常告警
etcdctl put /alerts/business/BUS_VOLTAGE_ALERT '{
  "alert_id": "BUS_VOLTAGE_ALERT",
  "type": "voltage_abnormal",
  "severity": "warning",
  "source": "bus_monitor",
  "message": "母线电压异常",
  "timestamp": 1702368001,
  "metric_value": 22.8
}'
```

## 常见问题

### Q: 故障诊断模块无法接收告警？

**A:** 检查以下几点：
1. etcd 服务是否正常运行
2. `-etcd` 参数配置的地址是否正确
3. `-prefix` 参数是否与健康监测模块一致
4. 网络连接是否畅通

### Q: 告警映射失败？

**A:** 确保：
1. 故障树配置中的 `alert_id` 与健康监测模块生成的告警ID完全一致
2. 大小写敏感，检查ID是否匹配
3. 查看日志中是否有 "告警ID未映射到任何基本事件" 的警告

### Q: 诊断结果不符合预期？

**A:** 
1. 检查故障树配置的逻辑门是否正确
2. 使用 `-log-level debug` 查看详细的求值过程
3. 运行集成测试验证故障树逻辑
4. 使用演示程序进行场景模拟

### Q: 如何支持多个故障树？

**A:** 启动多个故障诊断模块实例，每个实例加载不同的故障树配置：

```bash
# 业务层
./fault-diagnosis -config ./configs/fault_tree_business.json \
                  -prefix /alerts/business/

# 微服务层  
./fault-diagnosis -config ./configs/fault_tree_microservice.json \
                  -prefix /alerts/microservice/
```

## 性能调优

### 1. etcd 连接池

对于高频告警场景，可以配置 etcd 连接池：

```go
cli, err := clientv3.New(clientv3.Config{
    Endpoints:   endpoints,
    DialTimeout: 5 * time.Second,
    MaxCallRecvMsgSize: 10 * 1024 * 1024, // 10MB
})
```

### 2. 批量处理

在 `DiagnosisEngine` 中实现批量处理机制，减少求值频率：

```go
type DiagnosisEngine struct {
    // ...
    alertQueue chan *models.AlertEvent
    batchSize  int
    batchTime  time.Duration
}

func (e *DiagnosisEngine) batchProcess() {
    ticker := time.NewTicker(e.batchTime)
    alerts := make([]*models.AlertEvent, 0, e.batchSize)
    
    for {
        select {
        case alert := <-e.alertQueue:
            alerts = append(alerts, alert)
            if len(alerts) >= e.batchSize {
                e.processBatch(alerts)
                alerts = alerts[:0]
            }
        case <-ticker.C:
            if len(alerts) > 0 {
                e.processBatch(alerts)
                alerts = alerts[:0]
            }
        }
    }
}
```

### 3. 增量求值

优化为仅对受影响的子树进行求值，避免全量求值。
