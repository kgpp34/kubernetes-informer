FROM golang:1.20.5

WORKDIR /deployment

COPY go.mod .
COPY go.sum .

RUN go mod download && go clean -modcache

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN go build -o main ./cmd/main.go

# 清理不必要的构建文件
RUN go clean

EXPOSE 8080

CMD ["./main"]
