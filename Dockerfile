# Polka — home library server.
# Build: docker build -t polka .
# Run:   docker run -p 12791:12791 -v polka-data:/data -v /path/to/books:/books polka

# --- FB2 converter, separate executable (same pinned source as the old sidecar) ---
FROM golang:1.27.1-bookworm AS converter
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates wget curl unzip gzip && rm -rf /var/lib/apt/lists/*
WORKDIR /converter
RUN git init . && git remote add origin https://github.com/rupor-github/fb2cng.git && git fetch --depth=1 origin e526f866d30f19aabfb2134204433e1485d4965d && git checkout --detach FETCH_HEAD
COPY build/converter-task.yaml ./polka-task.yaml
RUN mkdir -p /out && TERM= CGO_ENABLED=0 GOFLAGS=-mod=mod go tool task -t polka-task.yaml build
COPY build/converter-smoke.fb2 ./polka-smoke.fb2
RUN /out/fbc --version && /out/fbc convert --help | grep -q -- '--output-file' \
    && /out/fbc convert --to epub3 --output-file /tmp/polka-smoke.epub polka-smoke.fb2 \
    && unzip -p /tmp/polka-smoke.epub mimetype | grep -qx 'application/epub+zip' \
    && unzip -p /tmp/polka-smoke.epub '*.opf' | grep -q 'version="3.0"' \
    && rm /tmp/polka-smoke.epub
# Ship the matching converter source and generated resources with the image.
RUN tar --exclude=./.git --exclude=./.task -czf /out/fb2cng-source.tar.gz .

# --- Frontend ---
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- Server (pure Go, no CGO) ---
FROM golang:1.26.11-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=0.1.1
# -tags nodynamic: decode JPEG XL covers via the pure-Go wazero backend
# (no purego/CGO), so the build stays CGO-free across all targets.
RUN CGO_ENABLED=0 go build -trimpath -tags nodynamic -ldflags "-s -w -X main.version=${VERSION}" -o /polka ./cmd/polka

# --- Runtime ---
FROM alpine:3.21
ARG VERSION=0.1.1
ARG REVISION=unknown
LABEL org.opencontainers.image.source="https://github.com/cathul/polka" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"
RUN apk add --no-cache ca-certificates && adduser -D -H polka && mkdir -p /data /books && chown polka /data /books
COPY --from=build /polka /usr/local/bin/polka
COPY --from=converter /out/fbc /usr/local/bin/fbc
COPY --from=converter /converter/LICENSE /usr/share/licenses/fb2cng/LICENSE
COPY --from=converter /out/fb2cng-source.tar.gz /usr/share/licenses/fb2cng/source.tar.gz
COPY build/CONVERTER-SOURCE.txt /usr/share/licenses/fb2cng/SOURCE.txt
USER polka
RUN /usr/local/bin/polka version && /usr/local/bin/fbc --version
# Defaults via env so any subcommand (serve, import, passwd) run with
# `docker exec` uses the data/library volumes, not the user's home dir.
ENV POLKA_DATA_DIR=/data \
    POLKA_LIBRARY_DIR=/books \
    POLKA_ADDR=:12791
VOLUME ["/data", "/books"]
EXPOSE 12791
ENTRYPOINT ["polka"]
CMD ["serve"]
