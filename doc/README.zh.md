# rkv
rkv是一个基于Raft共识的分布式KV系统。它足够可靠，并且十分易于使用、扩展和开发。

## 快速开始

以下示例可以启动一个三节点的 rkv 集群。

`注意：请确保端口 10001、10002、10003、10004、10005 和 10006 没有被占用。您也可以在启动参数中指定其他端口。`

### 使用docker-compose启动

调用startup.sh脚本即可快速启动一个三节点rkv集群。
```bash
chmod +x hack/startup.sh
./hack/startup.sh
```
容器如下
```bash
# docker ps
CONTAINER ID   IMAGE                  COMMAND                   CREATED         STATUS                          PORTS                                  NAMES
f4572975d916   rkvd:v0.0.1            "/app/rkvd --id 3 --…"    3 minutes ago   Up 1 minutes (healthy)          0.0.0.0:10005-10006->10005-10006/tcp   rkvd-node3
0bc09339b783   rkvd:v0.0.1            "/app/rkvd --id 2 --…"    4 minutes ago   Up 2 minutes (healthy)          0.0.0.0:10003-10004->10003-10004/tcp   rkvd-node2
fda902aff12a   rkvd:v0.0.1            "/app/rkvd --id 1"        4 minutes ago   Up 3 minutes (healthy)          0.0.0.0:10001-10002->10001-10002/tcp   rkvd-node1
```
你可以进入容器通过curl命令连接集群。

设置一个键
```bash
curl -X POST 'http://127.0.0.1:10002/keys/foo?val=bar'
```

查询该键
```bash
# curl 'http://127.0.0.1:10002/keys/foo'
bar
```

也可以查看集群中的节点信息

```bash
# curl 'http://127.0.0.1:10002/servers'
[{"addr":"127.0.0.1:10001","id":"1"},{"addr":"127.0.0.1:10003","id":"2"},{"addr":"127.0.0.1:10005","id":"3"}]
```

### 手工二进制方式运行
1. 首先构建可执行的二进制文件

```bash
cd rkv/
go build -o bin/rkvd cmd/server/main.go
```

`请确保端口10001、10002、10003、10004、10005、10006未被占用，你也可以在启动参数中指定其他端口`

2. 启动第一个节点
```bash
./bin/rkvd --id 1
```
3. 然后启动第二个和第三个节点
```bash
./bin/rkvd --id 2 --join 127.0.0.1:10002 --raft-addr 127.0.0.1:10003 --server-addr 127.0.0.1:10004 --data-dir /tmp/rkv2/
```

```bash
./bin/rkvd --id 3 --join 127.0.0.1:10002 --raft-addr 127.0.0.1:10005 --server-addr 127.0.0.1:10006 --data-dir /tmp/rkv3/
```
---
可以查看[设计文档]()和代码了解更多功能。