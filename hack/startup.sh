#!/bin/bash

# 设置镜像名称和标签
IMAGE_NAME="rkvd"
IMAGE_TAG="v0.0.1"

# 构建 Docker 镜像
echo "build Docker image..."
docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .

# 检查构建是否成功
if [ $? -ne 0 ]; then
    echo "build image failed"
    exit 1
fi

# 启动 Docker Compose
echo "Start Docker Compose..."
docker-compose up -d

# 检查 Docker Compose 是否启动成功
if [ $? -ne 0 ]; then
    echo "Docker Compose start failed"
    exit 1
fi

echo "success"