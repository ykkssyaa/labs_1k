# сборка и тесты бэкенда
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go test ./... && CGO_ENABLED=0 go build -o /places-server .

# итоговый образ: сервер отдаёт API и статику фронтенда
FROM alpine:3.20
WORKDIR /app
COPY --from=build /places-server /app/places-server
COPY web /app/web
ENV STATIC_DIR=/app/web DB_PATH=/data/reviews.db PORT=8888
VOLUME /data
EXPOSE 8888
CMD ["/app/places-server"]
