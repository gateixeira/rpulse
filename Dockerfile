FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Setup cross-compilation
ARG TARGETARCH TARGETOS

# Disable CGO and build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o rpulse

# Final stage
FROM alpine:3.21

RUN apk add --no-cache postgresql-client

WORKDIR /app

COPY --from=builder /app/rpulse /usr/local/bin/
COPY templates/ /app/templates/
COPY migrations/ /app/migrations/

COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

RUN mkdir -p /app/data

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["rpulse"]