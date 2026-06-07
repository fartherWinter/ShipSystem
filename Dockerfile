FROM node:20-alpine AS web-build

WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
ARG FRONTEND_API_BASE=http://localhost:8080
ARG FRONTEND_MODE=off
ARG FRONTEND_MAP_TILE_URL=https://tile.openstreetmap.org/{z}/{x}/{y}.png
ARG FRONTEND_MAP_TILE_ATTRIBUTION=OpenStreetMap
RUN VITE_API_BASE="$FRONTEND_API_BASE" VITE_AUTH_MODE="$FRONTEND_MODE" VITE_MAP_TILE_URL="$FRONTEND_MAP_TILE_URL" VITE_MAP_TILE_ATTRIBUTION="$FRONTEND_MAP_TILE_ATTRIBUTION" npm run build

FROM golang:1.23-alpine AS backend-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/sim-server ./cmd/sim-server

FROM alpine:3.20

RUN adduser -D -H shipsim
WORKDIR /app
COPY --from=backend-build /out/sim-server /app/sim-server
COPY --from=web-build /src/web/dist /app/web
COPY migrations/ /app/migrations/
COPY scenarios/ /app/scenarios/

ENV SHIP_SIM_ADDR=:8080
ENV SHIP_SIM_STATIC_DIR=/app/web
ENV SHIP_SIM_SCENARIO_DIR=/app/scenarios

USER shipsim
EXPOSE 8080
ENTRYPOINT ["/app/sim-server"]
