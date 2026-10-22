# Stage 1: Сборка бинарного файла
FROM golang:alpine AS builder

WORKDIR /src

# Установка необходимых системных утилит
RUN apk add --no-cache ca-certificates tzdata

# Кэширование Go модулей
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Копирование исходного кода бэкенда
COPY backend/ ./

# Компиляция статического бинарника без CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/server ./cmd/server

# Stage 2: Минимальный рантайм образ
FROM alpine:3.19

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

# Копирование бинарника из builder
COPY --from=builder /bin/server /app/server

# Копирование тестовых данных scoring и конфигурации модели
COPY scoring/testdata/cases.json /app/scoring/testdata/cases.json
COPY scoring/model.json /app/scoring/model.json

EXPOSE 8080

ENTRYPOINT ["/app/server"]
