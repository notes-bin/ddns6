# 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X github.com/notes-bin/ddns6/cmd.Version=${VERSION} -X github.com/notes-bin/ddns6/cmd.Commit=${COMMIT} -X github.com/notes-bin/ddns6/cmd.buildAt=${BUILD_TIME}" \
    -o ddns6 .

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/ddns6 .

RUN adduser -D ddns6
USER ddns6

ENTRYPOINT ["/app/ddns6"]
CMD ["--help"]
