# 故障诊断模块部署指南

## 部署方式

本文档提供多种部署方式，可根据实际需求选择。

## 方式1：本地直接部署

### 前置条件

- Go 1.24.5+
- etcd 3.5+

### 步骤

1. **安装依赖**

```bash
# 安装etcd（Ubuntu/Debian）
sudo apt-get update
sudo apt-get install etcd

# 或使用Homebrew（macOS）
brew install etcd
```

2. **启动etcd**

```bash
etcd --listen-client-urls http://0.0.0.0:2379 \
     --advertise-client-urls http://localhost:2379
```

3. **编译项目**

```bash
cd fault-diagnosis
make build
# 或
./build.sh
```

4. **启动服务**

```bash
# 启动业务层诊断
./build/fault-diagnosis \
  -config ./configs/fault_tree_business.json \
  -etcd localhost:2379 \
  -prefix /alerts/business/

# 启动微服务层诊断（另开终端）
./build/fault-diagnosis \
  -config ./configs/fault_tree_microservice.json \
  -etcd localhost:2379 \
  -prefix /alerts/microservice/
```

## 方式2：Docker部署

### 单容器部署

1. **构建镜像**

```bash
docker build -t fault-diagnosis:latest .
```

2. **运行容器**

```bash
docker run -d \
  --name fault-diagnosis \
  --network host \
  fault-diagnosis:latest \
  -config /app/configs/fault_tree_business.json \
  -etcd localhost:2379 \
  -prefix /alerts/
```

### Docker Compose部署（推荐）

1. **启动所有服务**

```bash
make docker-up
# 或
docker-compose up -d
```

这将启动：
- etcd服务
- 业务层故障诊断服务
- 微服务层故障诊断服务

2. **查看日志**

```bash
make docker-logs
# 或
docker-compose logs -f
```

3. **停止服务**

```bash
make docker-down
# 或
docker-compose down
```

## 方式3：Kubernetes部署

### 1. 创建ConfigMap

```bash
kubectl create configmap fault-tree-business \
  --from-file=configs/fault_tree_business.json

kubectl create configmap fault-tree-microservice \
  --from-file=configs/fault_tree_microservice.json
```

### 2. 创建Deployment

创建 `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fault-diagnosis-business
  labels:
    app: fault-diagnosis
    layer: business
spec:
  replicas: 2
  selector:
    matchLabels:
      app: fault-diagnosis
      layer: business
  template:
    metadata:
      labels:
        app: fault-diagnosis
        layer: business
    spec:
      containers:
      - name: fault-diagnosis
        image: fault-diagnosis:latest
        args:
        - -config=/etc/fault-diagnosis/fault_tree_business.json
        - -etcd=$(ETCD_ENDPOINTS)
        - -prefix=/alerts/business/
        - -log-level=info
        env:
        - name: ETCD_ENDPOINTS
          value: "etcd-service:2379"
        volumeMounts:
        - name: config
          mountPath: /etc/fault-diagnosis
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
      volumes:
      - name: config
        configMap:
          name: fault-tree-business
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fault-diagnosis-microservice
  labels:
    app: fault-diagnosis
    layer: microservice
spec:
  replicas: 2
  selector:
    matchLabels:
      app: fault-diagnosis
      layer: microservice
  template:
    metadata:
      labels:
        app: fault-diagnosis
        layer: microservice
    spec:
      containers:
      - name: fault-diagnosis
        image: fault-diagnosis:latest
        args:
        - -config=/etc/fault-diagnosis/fault_tree_microservice.json
        - -etcd=$(ETCD_ENDPOINTS)
        - -prefix=/alerts/microservice/
        - -log-level=info
        env:
        - name: ETCD_ENDPOINTS
          value: "etcd-service:2379"
        volumeMounts:
        - name: config
          mountPath: /etc/fault-diagnosis
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
      volumes:
      - name: config
        configMap:
          name: fault-tree-microservice
```

### 3. 部署

```bash
kubectl apply -f k8s/deployment.yaml
```

### 4. 查看状态

```bash
kubectl get pods -l app=fault-diagnosis
kubectl logs -f -l app=fault-diagnosis
```

## 方式4：Systemd服务（生产环境）

### 1. 安装二进制

```bash
make install
# 或
sudo cp build/fault-diagnosis /usr/local/bin/
sudo chmod +x /usr/local/bin/fault-diagnosis
```

### 2. 创建配置目录

```bash
sudo mkdir -p /etc/fault-diagnosis
sudo cp configs/*.json /etc/fault-diagnosis/
```

### 3. 创建Systemd服务文件

业务层服务 `/etc/systemd/system/fault-diagnosis-business.service`:

```ini
[Unit]
Description=Fault Diagnosis Module - Business Layer
After=network.target etcd.service
Wants=etcd.service

[Service]
Type=simple
User=fault-diagnosis
Group=fault-diagnosis
ExecStart=/usr/local/bin/fault-diagnosis \
  -config=/etc/fault-diagnosis/fault_tree_business.json \
  -etcd=localhost:2379 \
  -prefix=/alerts/business/ \
  -log-level=info \
  -output=/var/log/fault-diagnosis/business-diagnosis.log
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

微服务层服务 `/etc/systemd/system/fault-diagnosis-microservice.service`:

```ini
[Unit]
Description=Fault Diagnosis Module - Microservice Layer
After=network.target etcd.service
Wants=etcd.service

[Service]
Type=simple
User=fault-diagnosis
Group=fault-diagnosis
ExecStart=/usr/local/bin/fault-diagnosis \
  -config=/etc/fault-diagnosis/fault_tree_microservice.json \
  -etcd=localhost:2379 \
  -prefix=/alerts/microservice/ \
  -log-level=info \
  -output=/var/log/fault-diagnosis/microservice-diagnosis.log
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

### 4. 创建用户和日志目录

```bash
sudo useradd -r -s /bin/false fault-diagnosis
sudo mkdir -p /var/log/fault-diagnosis
sudo chown fault-diagnosis:fault-diagnosis /var/log/fault-diagnosis
```

### 5. 启动服务

```bash
# 重载systemd配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start fault-diagnosis-business
sudo systemctl start fault-diagnosis-microservice

# 设置开机自启
sudo systemctl enable fault-diagnosis-business
sudo systemctl enable fault-diagnosis-microservice

# 查看状态
sudo systemctl status fault-diagnosis-business
sudo systemctl status fault-diagnosis-microservice

# 查看日志
sudo journalctl -u fault-diagnosis-business -f
sudo journalctl -u fault-diagnosis-microservice -f
```

## 配置管理

### 环境变量

可以通过环境变量覆盖命令行参数：

```bash
export FAULT_DIAGNOSIS_CONFIG=/path/to/config.json
export FAULT_DIAGNOSIS_ETCD=localhost:2379
export FAULT_DIAGNOSIS_PREFIX=/alerts/
export FAULT_DIAGNOSIS_LOG_LEVEL=debug
```

### 配置文件热加载

当前版本不支持配置文件热加载，修改配置后需要重启服务：

```bash
# 本地部署
# 重启进程

# Docker Compose
docker-compose restart fault-diagnosis-business

# Kubernetes
kubectl rollout restart deployment/fault-diagnosis-business

# Systemd
sudo systemctl restart fault-diagnosis-business
```

## 健康检查

### 检查服务状态

```bash
# 检查etcd连接
etcdctl --endpoints=localhost:2379 endpoint health

# 查看告警事件
etcdctl get /alerts/ --prefix --keys-only

# 查看诊断结果（如果写入etcd）
etcdctl get /diagnosis/ --prefix
```

### 监控指标

建议监控以下指标：

- 服务进程是否运行
- etcd连接状态
- 告警接收速率
- 诊断触发频率
- 内存使用情况
- CPU使用情况

可以集成Prometheus + Grafana进行监控（需要添加metrics端点）。

## 故障排查

### 1. 服务无法启动

检查：
- etcd是否正常运行
- 配置文件路径是否正确
- 配置文件格式是否有效（JSON语法）

```bash
# 验证JSON格式
jq . configs/fault_tree_business.json
```

### 2. 无法接收告警

检查：
- etcd连接是否正常
- watch prefix是否正确
- 健康监测模块是否正常写入告警

```bash
# 手动写入测试告警
etcdctl put /alerts/business/TEST_ALERT '{"alert_id":"TEST_ALERT",...}'

# 监听etcd变化
etcdctl watch /alerts/ --prefix
```

### 3. 诊断结果不正确

检查：
- 故障树配置逻辑是否正确
- 告警ID映射是否正确
- 使用 `-log-level debug` 查看详细日志

```bash
./fault-diagnosis -log-level debug -config ...
```

### 4. 性能问题

优化建议：
- 增加服务实例（K8s replicas）
- 优化故障树结构，减少嵌套层级
- 使用批量处理机制
- 增加etcd集群节点

## 升级和回滚

### 滚动升级（Kubernetes）

```bash
# 更新镜像
kubectl set image deployment/fault-diagnosis-business \
  fault-diagnosis=fault-diagnosis:v2.0.0

# 查看升级状态
kubectl rollout status deployment/fault-diagnosis-business

# 回滚
kubectl rollout undo deployment/fault-diagnosis-business
```

### 蓝绿部署

```bash
# 部署新版本（绿色环境）
kubectl apply -f k8s/deployment-v2.yaml

# 验证新版本
# ...

# 切换流量（更新Service selector）
kubectl patch service fault-diagnosis -p '{"spec":{"selector":{"version":"v2"}}}'

# 删除旧版本（蓝色环境）
kubectl delete deployment fault-diagnosis-v1
```

## 备份和恢复

### 配置备份

```bash
# 备份故障树配置
tar -czf fault-tree-backup-$(date +%Y%m%d).tar.gz configs/

# 备份etcd数据
etcdctl snapshot save fault-diagnosis-snapshot.db
```

### 恢复

```bash
# 恢复配置
tar -xzf fault-tree-backup-20231212.tar.gz

# 恢复etcd
etcdctl snapshot restore fault-diagnosis-snapshot.db
```

## 安全加固

### 1. etcd认证

```bash
# 启用etcd认证
etcdctl user add root
etcdctl auth enable

# 使用认证连接
./fault-diagnosis \
  -etcd localhost:2379 \
  -etcd-user root:password
```

### 2. TLS加密

```bash
# 启用TLS
./fault-diagnosis \
  -etcd https://localhost:2379 \
  -etcd-ca-cert /path/to/ca.crt \
  -etcd-cert /path/to/client.crt \
  -etcd-key /path/to/client.key
```

### 3. 最小权限原则

运行服务的用户应该只有必要的权限，不应该使用root用户。

## 性能调优

### 1. Go运行时参数

```bash
# 设置GOMAXPROCS
export GOMAXPROCS=4

# 设置内存限制
export GOMEMLIMIT=1GiB
```

### 2. etcd优化

```ini
# etcd配置
--max-request-bytes=10485760
--quota-backend-bytes=8589934592
```

### 3. 资源限制（Kubernetes）

根据实际负载调整资源配额：

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "200m"
  limits:
    memory: "256Mi"
    cpu: "500m"
```

## 生产环境检查清单

部署前检查：

- [ ] etcd集群已部署且高可用（3或5节点）
- [ ] 故障树配置已验证
- [ ] 集成测试已通过
- [ ] 监控和告警已配置
- [ ] 日志收集已配置
- [ ] 资源配额已设置
- [ ] 健康检查已配置
- [ ] 备份策略已制定
- [ ] 灾难恢复方案已准备
- [ ] 文档已更新

## 参考资料

- [README.md](README.md) - 项目概述和使用说明
- [QUICKSTART.md](QUICKSTART.md) - 快速开始指南
- [ARCHITECTURE.md](ARCHITECTURE.md) - 架构设计文档
- [INTEGRATION.md](INTEGRATION.md) - 集成指南
