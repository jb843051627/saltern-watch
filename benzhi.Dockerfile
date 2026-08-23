# 盐田监控服务多架构镜像（固定 golang:1.22-bookworm，禁止更换）
FROM golang:1.22-bookworm AS build
ENV GOPROXY=https://goproxy.cn,direct
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/saltern-watch ./cmd/saltern-watch

FROM build
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY --from=build /out/saltern-watch /src/saltern-watch
ENV SALTERN_DB_PATH=/data/saltern.db
ENV SALTERN_STATIC_DIR=/src/web/static
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/src/saltern-watch"]
