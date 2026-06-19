FROM golang:1.23-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/ling-shu \
    ./cmd/server

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S ling-shu \
    && adduser -S ling-shu -G ling-shu

COPY --from=builder /out/ling-shu /app/ling-shu
COPY configs /app/configs
COPY prompts /app/prompts

ENV TZ=Asia/Shanghai

EXPOSE 8080

USER ling-shu

ENTRYPOINT ["/app/ling-shu"]
CMD ["-config", "configs/config.example.yaml"]
