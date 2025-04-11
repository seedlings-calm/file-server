# 使用 minio 作为文件存储服务

## 目录

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

## 单服务器部署步骤

1. 启动 docker 服务
2. 当前目录执行 docker-compose up -d
3. 访问 http://localhost:9000 登录 minio,看下下是否启动成功
4. curl http://localhost:9000/minio/v2/metrics/cluster 看是否返回的有指标数据，不可浏览器访问，会被重定向到 9001 控制面板服务
5. 登录 grafana， 添加 data sources （Prometheus 数据源（URL 填 http://prometheus:9090））
6. import dashboard， 输入 minIO dashboard id：13502 或上传 官方 [JSON 模板](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/grafana/minio-dashboard.json)

warning::

容器之间互相访问使用 http:///host.docker.internal:9000 或者 http://{容器名称}:port

## 集群的部署配置和启动 [multiple-service](./multiple-service/)

## minio 客户端 mc 的功能描述

1. 给 minio 服务起个别名，用于其他命令的使用

```shell
mc alias set myminio http://minio:9000 minioadmin minio123456
```

2. 在 myminio 服务中，创建一个 bucket

```shell
mc mb -p myminio/public || "public bucket already exists"
```

3. 设置 bucket 的访问权限为匿名下载访问（公共访问）

```shell
mc anonymous set download myminio/public
```

4. 列出 myminio 服务中，所有 bucket

```shell
mc ls myminio
```

5. 列出 myminio 服务中，public bucket 下的所有对象

```shell
mc ls myminio/public
```

## minIO 的权限策略理解

```
[User]
   ↓ 绑定
[Policy] ← JSON 权限规则
   ↓ 决定
[允许访问哪些 Bucket/Object、执行哪些操作]

```

### 举例策略设置：

- 我只有两个 bucket， 1. public 2. usersource
- public 中 有 images/,videos/,icon/,
- usersource 中 是 {userUnique}/{fileName}, userUnique 是用户实际操作时判断指定
- 我现在只需要两个 user， 1. apiUser, 2.sysUser

  1. apiUser 的策略是： deny 操作: public 的修改，删除，其余都 允许

  ```json
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "s3:GetObject", // 允许读取（下载）文件
          "s3:ListBucket" // 允许列出桶中的文件
        ],
        "Resource": [
          "arn:aws:s3:::public/*", // public 中的所有文件
          "arn:aws:s3:::usersource/*" // usersource 中的所有文件
        ]
      },
      {
        "Effect": "Deny",
        "Action": [
          "s3:PutObject", // 禁止上传文件
          "s3:DeleteObject" // 禁止删除文件
        ],
        "Resource": "arn:aws:s3:::public/*" // 只对 public 中的文件进行禁止
      }
    ]
  }
  ```

  2. sysUser 的策略是：deny 操作：usersource 的上传，修改，其余都允许

  ```json
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "s3:GetObject", // 允许读取（下载）文件
          "s3:ListBucket" // 允许列出桶中的文件
        ],
        "Resource": [
          "arn:aws:s3:::public/*", // public 中的所有文件
          "arn:aws:s3:::usersource/*" // usersource 中的所有文件
        ]
      },
      {
        "Effect": "Allow",
        "Action": [
          "s3:PutObject", // 允许上传文件
          "s3:DeleteObject" // 允许删除文件
        ],
        "Resource": "arn:aws:s3:::public/*" // 允许 public 中的所有文件操作
      },
      {
        "Effect": "Deny",
        "Action": [
          "s3:PutObject", // 禁止上传文件到 usersource
          "s3:DeleteObject" // 禁止删除文件在 usersource
        ],
        "Resource": "arn:aws:s3:::usersource/*" // 只对 usersource 中的文件进行禁止
      }
    ]
  }
  ```

### S3 权限指令（Action）及中文含义

| S3 权限指令（Action）         | 中文含义                                         |
| ----------------------------- | ------------------------------------------------ |
| s3:GetObject                  | 读取对象（下载文件）                             |
| s3:PutObject                  | 上传对象（新增或修改文件）                       |
| s3:DeleteObject               | 删除对象（删除文件）                             |
| s3:ListBucket                 | 列出 bucket 中的对象（查看文件列表）             |
| s3:ListAllMyBuckets           | 查看所有 bucket 名称（列出桶）                   |
| s3:GetBucketLocation          | 获取 bucket 的位置信息（用于连接等）             |
| s3:DeleteBucket               | 删除 bucket（危险操作，一般禁止）                |
| s3:CreateBucket               | 创建 bucket                                      |
| s3:AbortMultipartUpload       | 终止正在进行的分片上传                           |
| s3:ListMultipartUploadParts   | 查看一个分片上传的所有已上传部分                 |
| s3:ListBucketMultipartUploads | 查看 bucket 中所有未完成的分片上传               |
| s3:CopyObject                 | 复制对象（例如把一个文件从 A 路径复制到 B 路径） |
| s3:GetBucketPolicy            | 获取 bucket 的策略                               |
| s3:PutBucketPolicy            | 设置 bucket 的策略                               |
| s3:DeleteBucketPolicy         | 删除 bucket 的策略                               |

## MinIO 🪣 Bucket 命名规则

Bucket 名类似于域名，必须遵守 DNS 命名规范。

### ✅ 合法规则

- **长度**：必须为 **3 到 63 个字符**
- **字符集**：只能包含：
  - 小写字母 (`a-z`)
  - 数字 (`0-9`)
  - 连字符 (`-`)
- **不能**：
  - 以连字符 `-` 开头或结尾
  - 以点号 `.` 开头或结尾（不推荐）
  - 使用下划线 `_`（不兼容某些客户端）
  - 包含大写字母或特殊字符

### ✅ 示例

```text
my-bucket
images2025
data-bucket-01
```

## 📦 MinIO Object（对象）命名规则

在 MinIO（兼容 S3 协议）中，对象名称即对象键（Object Key），是存储在 Bucket 中文件的唯一标识符。对象命名灵活，但也有一定的规范和最佳实践。

---

### ✅ 基本规则

- 对象名称最大长度：**1024 字节**
- 对象名必须是 **UTF-8 编码字符串**
- 对象名可以包含任意字符（包括中文、空格、特殊符号）
- 对象名可以使用 `/` 实现“类文件夹”的逻辑结构（实际上并无真正目录）

---

### ✍️ 合法字符示例

```text
hello.jpg
2025/会议记录.pdf
user_1234/avatar.png
项目文档/设计v1.0/首页草图.psd
videos/活动/现场录音.mp3
data/日志_2025-01-01.log
```
