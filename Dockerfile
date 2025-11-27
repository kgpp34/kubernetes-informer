# 第一阶段：构建
ARG GO_IMAGE=golang:1.20.5
FROM ${GO_IMAGE} AS builder

# 设置工作目录
WORKDIR /app

# 将 go.mod 和 go.sum 复制到工作目录
COPY go.mod go.sum ./

# 下载所有依赖并清理模块缓存
RUN go mod download && go clean -modcache

# 将项目文件复制到工作目录
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY api/ api/

# 构建项目
RUN go build -o main ./cmd/main.go

# 暴露应用端口
EXPOSE 8080

# 运行项目
CMD ["./main"]