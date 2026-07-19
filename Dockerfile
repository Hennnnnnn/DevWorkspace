# Build stage
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/devsync-server ./cmd/devsync-server

# Run stage
FROM alpine:3.20
RUN adduser -D -u 10001 devsync
COPY --from=build /out/devsync-server /usr/local/bin/devsync-server
USER devsync
EXPOSE 8080
ENTRYPOINT ["devsync-server", "serve"]
