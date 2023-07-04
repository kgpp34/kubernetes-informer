FROM golang:1.20.5

WORKDIR /deployment

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN go build -o main ./cmd/main.go

EXPOSE 8080

CMD ["./main"]
