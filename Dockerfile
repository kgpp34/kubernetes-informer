# 阶段一：构建 (Builder Stage)
# 使用 Go 官方的 Alpine 变种作为构建环境，它比标准的 Go 镜像小得多
FROM golang:1.20.5-alpine AS builder

# 设置构建参数，确保生成一个完全静态链接的二进制文件
# CGO_ENABLED=0 是关键，它允许我们在没有 C 库的最小镜像中运行程序
ENV CGO_ENABLED=0
ENV GOOS=linux

# 设置工作目录
WORKDIR /app

# 复制依赖文件 (利用 Docker 缓存机制)
COPY go.mod go.sum ./

# 下载所有依赖
RUN go mod download

# 复制所有项目文件
COPY . .

# 构建项目
# -ldflags "-s -w" 标志用于移除调试信息和符号表，进一步减小二进制文件体积
RUN go build -ldflags "-s -w" -o main ./cmd/main.go

# ----------------------------------------------------------------------------------

# 阶段二：最终镜像 (Final Stage)
# 使用极小的 Alpine 基础镜像，只包含运行应用所需的最小环境
FROM alpine:latest

# 安装 ca-certificates，确保 Go 应用程序可以处理 HTTPS 请求
RUN apk --no-cache add ca-certificates

# 设置应用运行目录
WORKDIR /root/

# 从 'builder' 阶段复制编译好的静态二进制文件 'main'
# 这一步是体积优化的核心：我们只复制了最终结果
COPY --from=builder /app/main .

# 暴露应用端口
EXPOSE 8080

# 运行项目
CMD ["./main"]