# BENZHI 打包说明（saltern-watch）

## 镜像构建

```bash
./build_benzhi_docker.sh saltern-run linux/amd64
./build_benzhi_docker.sh saltern-run linux/arm64
```

镜像名模板：`benzhi/<name>:latest`，基础镜像固定 `golang:1.22-bookworm`，
代理 `GOPROXY=https://goproxy.cn,direct`，工具链钉死 `GOTOOLCHAIN=local`。

## 运行

```bash
docker run --rm -p 8080:8080 -v saltern-data:/data benzhi/saltern-run:latest
```

健康检查：`curl http://localhost:8080/api/v1/health`
看板：`http://localhost:8080/`（内嵌静态页）

## 注意

- 数据库落盘 `/data/saltern.db`（SQLite WAL），容器重启数据保留。
- 演示种子数据：环境变量 `SALTERN_SEED_DEMO=1`。
- `-race` 只在原生架构运行；QEMU 模拟架构仅做 `go test`/`go build` 验证。
