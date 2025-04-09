#!/bin/bash

set -e

# 使用 mc 工具初始化 MinIO 资源
echo "⏳ Setting up MinIO..."

mc alias set local http://localhost:9000 minioadmin minioadmin

  mc mb -p local/public || echo "public bucket exists"
    # 设置 bucket 为匿名可读 公共访问的 (默认是私有的)
  mc anonymous set download local/public


echo "✅ Init complete. Buckets ready: public."
