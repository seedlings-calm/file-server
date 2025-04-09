#! bin/bash


# 启动 MinIO
echo "⏳ Starting MinIO..."
/usr/bin/minio server http://minio{1...4}/data --console-address ':9001' &

sleep 6

echo "✅ Multiple MinIO is ready."

# 执行初始化脚本
echo "⏳ Running init script..."
/init.sh

# 阻止容器退出
wait