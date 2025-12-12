FROM golang:1.23.2-alpine as builder
# 使用 Go 1.23.2 进行构建

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /dist

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o main .

FROM gcr.io/distroless/static-debian10

COPY --from=builder /dist/main .

# 暴露与应用监听一致的端口（main.go 为 :9000）
EXPOSE 9000
# Command to run when starting the container
CMD ["./main"]
