# syntax=docker/dockerfile:1

# --- Resolve dependencies (used only by tidy.cmd) ---------------------------
# `docker build --target modfiles --output type=local,dest=. .` writes an
# up-to-date go.mod and go.sum back into this folder without needing Go on the
# host.
FROM golang:1.27-alpine AS tidy
WORKDIR /src
COPY . .
RUN go mod tidy

FROM scratch AS modfiles
COPY --from=tidy /src/go.mod /src/go.sum /

# --- Build -------------------------------------------------------------------
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# BUILD_ID is printed by the worker at start-up. build.cmd sets it to the
# image tag.
ARG BUILD_ID=dev
RUN CGO_ENABLED=0 go vet ./... \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-X main.buildID=${BUILD_ID}" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -o /out/loadgen ./cmd/loadgen

# --- Runtime -----------------------------------------------------------------
FROM alpine:3.22
COPY --from=build /out/ /app/
USER 65532
ENTRYPOINT ["/app/worker"]
