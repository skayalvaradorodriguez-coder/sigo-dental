# Imagen única de producción: la API en Go sirve también la PWA compilada (STATIC_DIR).
# Construir:  docker build -t sigo-dental .
# Probar:     docker run --rm -p 8080:8080 --env-file produccion.env sigo-dental

# 1) Frontend (React + Vite)
FROM node:22-alpine AS web
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY index.html vite.config.js pwa-assets.config.js ./
COPY public ./public
COPY src ./src
RUN npm run build

# 2) Backend (Go, binario estático)
FROM golang:1.24-alpine AS api
WORKDIR /src
COPY sigo-backend/go.mod sigo-backend/go.sum ./
RUN go mod download
COPY sigo-backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sigo-api .

# 3) Imagen final mínima, sin compiladores y con usuario sin privilegios
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 sigo \
 && mkdir -p /data/archivos && chown sigo:sigo /data/archivos
WORKDIR /app
COPY --from=api /out/sigo-api ./sigo-api
COPY --from=web /app/dist ./dist
ENV APP_ENV=prod STATIC_DIR=/app/dist ARCHIVOS_DIR=/data/archivos PORT=8080 NOTIFICATIONS_DISPATCH_SECONDS=30
USER sigo
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s CMD wget -qO- http://127.0.0.1:8080/api/health || exit 1
# Aplica las migraciones pendientes (idempotentes) y arranca. Con APP_ENV=demo carga además los datos
# ficticios (la semilla no duplica lo que ya existe) y el paciente de volumen VOL000001 (RNF-01).
CMD ["sh", "-c", "./sigo-api migrar && if [ \"$APP_ENV\" = demo ]; then ./sigo-api semilla && ./sigo-api volumen; fi && exec ./sigo-api"]
