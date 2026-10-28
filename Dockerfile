# syntax=docker/dockerfile:1
# check=skip=SecretsUsedInArgOrEnv

# The skipped check flags TIIME_TOKEN_CACHE, which holds a file path, not a secret.

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/mcp-tiime ./cmd/mcp-tiime

# Created here because the distroless image has no shell: a named volume
# mounted on /data inherits this ownership.
RUN mkdir -p /out/data/downloads

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/mcp-tiime /usr/local/bin/mcp-tiime
COPY --from=build --chown=nonroot:nonroot /out/data /data

ENV TIIME_TOKEN_CACHE=/data/token.json \
    TIIME_PDF_DIR=/data/downloads

VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/mcp-tiime"]
