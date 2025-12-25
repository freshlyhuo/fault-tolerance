# 构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fault-diagnosis ./cmd/diagnosis

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/fault-diagnosis .

# 复制配置文件
COPY --from=builder /app/configs ./configs

# 暴露端口（如果需要）
# EXPOSE 8080

# 设置默认命令
ENTRYPOINT ["/app/fault-diagnosis"]
CMD ["-config", "/app/configs/fault_tree_business.json", "-etcd", "localhost:2379"]
