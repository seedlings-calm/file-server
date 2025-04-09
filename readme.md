### 使用 minio 作为文件存储服务

#### 目录

```dir
├── data //minio数据映射目录
├── docker-compose.yml //容器配置文件
├── go.mod
├── go.sum
├── main.go //gin框架实现了一个上传文件的接口
├── nginx
│   └── nginx.conf //Nginx 配置文件，代理minio文件资源
├── pkg
│   └── minio.go //minio 操作封装
├── prometheus.yml //Prometheus 配置文件，监控minio
├── readme.md
└── scripts //启动脚本
    ├── init.sh //初始化 minio的bucket，设置访问权限
    └── start.sh //启动 minio 服务
```

#### 步骤

1. 启动 docker 服务
2. 当前目录执行 docker-compose up -d
3. 访问 http://localhost:9000 登录 minio,看下下是否启动成功
4. curl http://localhost:9000/minio/v2/metrics/cluster 看是否返回的有指标数据，不可浏览器访问，会被重定向到 9001 控制面板服务
5. 登录 grafana， 添加 data sources （Prometheus 数据源（URL 填 http://prometheus:9090））
6. import dashboard， 输入 minIO dashboard id：13502 或上传 官方 [JSON 模板](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/grafana/minio-dashboard.json)
