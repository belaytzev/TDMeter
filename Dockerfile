# Stage 1: Build TDLib from source
FROM alpine:3.23 AS tdlib-builder

RUN apk add --no-cache \
    cmake \
    g++ \
    make \
    git \
    gperf \
    linux-headers \
    openssl-dev \
    zlib-dev

# Pin to the commit expected by go-tdlib v0.7.6
ARG TDLIB_COMMIT=22d49d5b87a4d5fc60a194dab02dd1d71529687f

RUN git clone https://github.com/tdlib/td.git /td && \
    cd /td && \
    git checkout ${TDLIB_COMMIT}

RUN cd /td && \
    mkdir build && cd build && \
    cmake -DCMAKE_BUILD_TYPE=Release \
          -DCMAKE_INSTALL_PREFIX=/usr/local \
          -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
          -DTD_ENABLE_BENCHMARK=OFF \
          -DTD_ENABLE_JNI=OFF \
          -DTD_ENABLE_DOTNET=OFF \
          .. && \
    cmake --build . --target tdjson_static -j "$(nproc)" && \
    find . -name '*.a' -exec cp {} /usr/local/lib/ \; && \
    cd /td && \
    find td -name '*.h' -o -name '*.hpp' | while read f; do \
      mkdir -p "/usr/local/include/$(dirname "$f")"; \
      cp "$f" "/usr/local/include/$f"; \
    done && \
    find build/td -name '*.h' -o -name '*.hpp' | while read f; do \
      dest="/usr/local/include/${f#build/}"; \
      mkdir -p "$(dirname "$dest")"; \
      cp "$f" "$dest"; \
    done

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache \
    g++ \
    linux-headers \
    openssl-dev \
    zlib-dev \
    ca-certificates

# Copy TDLib headers and libraries from builder
COPY --from=tdlib-builder /usr/local/include/td /usr/local/include/td
COPY --from=tdlib-builder /usr/local/lib/*.a /usr/local/lib/

WORKDIR /src

# Download dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Run tests (without tdlib tag since TDLib integration tests need network)
RUN go test ./...

# Build with TDLib support, static linking
RUN CGO_ENABLED=1 go build \
    -tags=tdlib \
    -ldflags="-s -w" \
    -o /tdmeter .

# Stage 3: Minimal runtime image
FROM alpine:3.23

RUN apk add --no-cache \
    ca-certificates \
    libstdc++ \
    openssl \
    zlib

RUN adduser -D -u 1000 tdmeter

COPY --from=builder /tdmeter /usr/local/bin/tdmeter

RUN mkdir -p /tmp/tdmeter-tdlib && chown tdmeter:tdmeter /tmp/tdmeter-tdlib

USER tdmeter

EXPOSE 2112

ENTRYPOINT ["tdmeter"]
CMD ["--config", "/etc/tdmeter/config.yaml"]
