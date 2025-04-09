#!/bin/bash

set -e

# 使用 mc 工具初始化 MinIO 资源
echo "⏳ Setting up MinIO..."

# 等待 MinIO 就绪
until curl -s http://localhost:9000/minio/health/ready; do
  echo "⏳ Waiting for MinIO to be ready..."
  sleep 2
done

 # ✅ 只有在 minio1 节点上执行初始化逻辑,minio各节点会自动同步
 echo $CONTAINER_NAME ">>>>"
if [ "$CONTAINER_NAME" = "minio1" ]; then
  echo "🚀 Running MinIO bucket and policy setup..."

mc alias set local http://localhost:9000 minioadmin minioadmin

  mc mb -p local/public || echo "public bucket exists"
    # 设置 bucket 为匿名可读 公共访问的 (默认是私有的)
  mc anonymous set download local/public

  
  echo "🟡 This is $CONTAINER_NAME, skipping bucket init (handled by minio1)"
fi