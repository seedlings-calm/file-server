#! bin/bash


# 启动 MinIO
echo "⏳ Starting MinIO..."
/usr/bin/minio server /data --console-address ':9001' &


echo "⏳ Waiting for MinIO to be ready..."
until curl -s http://localhost:9000/minio/health/ready; do
  sleep 2
done

echo "✅ MinIO is ready."

# 执行初始化脚本
echo "⏳ Running init script..."
/init.sh

# 阻止容器退出
wait