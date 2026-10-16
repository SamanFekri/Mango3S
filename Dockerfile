# syntax=docker/dockerfile:1
# check=skip=SecretsUsedInArgOrEnv
# (MANGO3S_SECRET_BACKEND names a storage backend, not a secret value.)
#
# Multi-stage build for Mango3S (Linux).
#
#   Build + unit tests, export the Linux binary to ./dist:
#     docker build --target artifact --output type=local,dest=dist .
#
#   Integration tests against MongoDB (see docker-compose.yml):
#     docker compose --profile test up --build --exit-code-from integration-tests
#
#   Run the GUI from a container (Linux host with X11 only, experimental):
#     docker compose --profile gui up --build app
#
# Desktop apps for macOS and Windows must be built natively with `wails build`.

ARG GO_VERSION=1.26
ARG NODE_VERSION=22

# ---- Frontend: install, test, build ----------------------------------------
FROM node:${NODE_VERSION}-bookworm AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json frontend/.npmrc ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm test && npm run build

# ---- Go: vet, unit tests, compile ------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/frontend/dist ./frontend/dist
ARG VERSION=0.1.0
# Unit tests must not depend on an OS keyring inside the container.
ENV MANGO3S_SECRET_BACKEND=memory
RUN go vet ./... && go test -count=1 ./...
RUN CGO_ENABLED=1 go build \
      -tags desktop,production,webkit2_41 \
      -trimpath \
      -ldflags "-s -w -X mango3s/internal/api.Version=${VERSION}" \
      -o /out/mango3s .

# ---- Integration tests (needs MONGO_URI) ------------------------------------
FROM build AS integration
ENV MONGO_URI=mongodb://mongo:27017
CMD ["go", "test", "-tags", "integration", "-count=1", "-v", "./internal/integration/..."]

# ---- Binary only, for `docker build --output` --------------------------------
FROM scratch AS artifact
COPY --from=build /out/mango3s /mango3s

# ---- Runtime image (Linux GUI via X11) --------------------------------------
FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgtk-3-0 libwebkit2gtk-4.1-0 ca-certificates fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 1000 app
USER app
# No OS keyring in a container: passwords go to the AES-GCM encrypted file in the config dir.
ENV MANGO3S_CONFIG_DIR=/home/app/.config/Mango3S \
    MANGO3S_SECRET_BACKEND=file
COPY --from=build /out/mango3s /usr/local/bin/mango3s
ENTRYPOINT ["/usr/local/bin/mango3s"]
