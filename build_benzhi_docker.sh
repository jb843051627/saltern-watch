#!/usr/bin/env bash
# 多架构镜像构建脚本（对齐《Go 项目打包规范》，内容固定不改逻辑）
set -euo pipefail
NAME="${1:?usage: build_benzhi_docker.sh <name> <platform>}"
PLATFORM="${2:-linux/amd64}"
DOCKERFILE="${DOCKERFILE:-benzhi.Dockerfile}"
IMAGE="benzhi/${NAME}:latest"
echo "[build] ${IMAGE} for ${PLATFORM} (dockerfile: ${DOCKERFILE})"
exec docker buildx build --platform "${PLATFORM}" -f "${DOCKERFILE}" -t "${IMAGE}" --load .
