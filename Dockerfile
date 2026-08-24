# 第一阶段：编译
# 使用包含 Go 1.26.3 的轻量级 alpine 镜像作为构建环境
FROM golang:1.26.3-alpine AS builder

# 设置时区
ENV TZ=Asia/Shanghai

# 设置国内地址
ENV GOPROXY=https://goproxy.cn,direct

# 设置编译目录
WORKDIR /build

# 复制Go模块并下载依赖
COPY go.mod go.sum* ./
RUN go mod download

# 复制所有go代码
COPY . .

# 声明一个构建参数，用于接收构建时间
ARG BUILD_DATE

# 可选：将这个参数持久化为环境变量，供容器运行时使用
ENV BUILD_DATE=${BUILD_DATE}

# 编译为静态二进制文件(禁用CGO,确保兼容性)
# 必须显式指定 GOARCH=amd64:gnet 库的 WsConnection.lastRecvPacketTick 字段在 32 位(GOARCH=386)下
# 无法保证 8 字节对齐,导致 atomic.StoreInt64 panic(unaligned 64-bit atomic operation)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X github.com/fish-tennis/gserver/internal.BuildType=docker -X 'github.com/fish-tennis/gserver/internal.BuildTime=${BUILD_DATE}'" -o gserver main.go

# 第二阶段：运行
# 使用极小的alpine镜像作为运行环境
FROM alpine:latest

# 安装 CA 证书（如果需要 HTTPS 调用）
#RUN apk --no-cache add ca-certificates

# 创建工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /build/gserver .
# 复制配置数据
COPY cfgdata ./cfgdata

RUN chmod +x ./gserver

# 设置容器启动入口
ENTRYPOINT ["./gserver"]