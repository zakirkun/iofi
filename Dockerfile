# --- frontend -----------------------------------------------------------
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- binary -------------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/iofi ./cmd/iofi

# --- runtime ------------------------------------------------------------
# git is required: agent workspaces are worktrees. node is here so the coding
# CLIs can be installed into the image — iofi spawns them as child processes,
# so they must exist in this container, not on the host.
FROM node:22-alpine
RUN apk add --no-cache git ca-certificates openssh-client && \
    adduser -D -u 10001 iofi
WORKDIR /app
COPY --from=build /out/iofi /usr/local/bin/iofi
COPY skills/ /app/skills/

# Add the coding tools you actually use, for example:
#   RUN npm install -g @anthropic-ai/claude-code
# Their credentials must be mounted at runtime; none are baked in.

USER iofi
ENV IOFI_ADDR=:8080 IOFI_DATA_DIR=/data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["iofi"]
