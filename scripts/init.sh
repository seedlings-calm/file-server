#!/bin/bash

set -e

# 使用 mc 工具初始化 MinIO 资源
echo "⏳  Setting up MinIO..."

mc alias set local http://localhost:9000 minioadmin minioadmin

  #创建bucket
  mc mb -p local/public || echo "public bucket exists"
  mc mb -p local/usersource || echo "usersource bucket exists"
  # 设置 bucket 为匿名可读 公共访问的 (默认是私有的)
  # mc anonymous set download local/public


echo "✅  Init complete. Buckets ready: public,usersource."

echo "⏳  Running policy script..."
# 设置策略，bucket必须创建好
/scripts/policy.sh