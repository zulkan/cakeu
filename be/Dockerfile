FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/resetpassword ./cmd/resetpassword

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app

WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=build /out/resetpassword /app/resetpassword
COPY --from=build /src/internal/migrations /app/internal/migrations

RUN mkdir -p /app/logs && chown -R app:app /app

USER app

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/server"]
