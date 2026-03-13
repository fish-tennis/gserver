# 第一阶段：编译
# 使用包含 Go 1.24.6 的轻量级 alpine 镜像作为构建环境
FROM golang:1.24.6-alpine AS builder

# 设置国内地址
ENV GOPROXY=https://goproxy.cn,direct

# 设置编译目录
WORKDIR /build

# 复制Go模块并下载依赖
COPY go.mod go.sum* ./
RUN go mod download

# 复制所有go代码
COPY . .

# 编译为静态二进制文件(禁用CGO,确保兼容性)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X github.com/fish-tennis/gserver/internal.BuildType=docker -X github.com/fish-tennis/gserver/internal.BuildTime=123" -o gserver main.go

# 第二阶段：运行
# 使用极小的alpine镜像作为运行环境
FROM alpine:latest

# 安装 CA 证书（如果需要 HTTPS 调用）
#RUN apk --no-cache add ca-certificates

# 设置时区
ENV TZ=Asia/Shanghai

# 创建工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /build/gserver .
# 复制配置数据
COPY cfgdata ./cfgdata

RUN chmod +x ./gserver

# 设置容器启动入口
ENTRYPOINT ["./gserver"]