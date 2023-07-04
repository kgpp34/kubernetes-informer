FROM golang:1.20.5

WORKDIR /deployment

COPY go.mod go.mod
COPY go.sum go.sum

ARG GOPROXY
ENV GOPROXY=${GOPROXY:}

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN GO111MODULE=on CGO_ENABLE=0 GOOS=linux \
    go build -o cmd/main.go

EXPOSE 8080

CMD ["./main"]