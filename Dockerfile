FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o agentgate ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/agentgate /agentgate

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/agentgate"]
