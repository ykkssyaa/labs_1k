FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vestnik ./cmd/vestnik

FROM alpine:3.22
RUN adduser -D -u 1000 vestnik
WORKDIR /app
COPY --from=build /out/vestnik /usr/local/bin/vestnik
COPY content_types ./content_types
COPY config ./config
COPY themes ./themes
RUN mkdir -p /app/data && chown vestnik /app/data
USER vestnik
EXPOSE 8080
VOLUME /app/data
CMD ["vestnik", "serve"]
