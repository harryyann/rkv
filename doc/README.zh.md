# rkv
rkv是一个基于Raft共识的分布式KV系统。它足够可靠，并且十分易于使用、扩展和开发。

## 快速开始
1. 首先构建可执行的二进制文件

```bash
cd rkv/
go build -o bin/rkvd cmd/server/main.go
```

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

4. 尝试连接到领导者并设置一个键

设置一个键。
```bash
curl -X POST 'http://127.0.0.1:10002/keys/foo?val=bar'
```
连接任意节点查询该键。
```bash
# curl 'http://127.0.0.1:10004/keys/foo'
bar
```
5. 还可以查询集群中的所有节点
```bash
# curl 'http://127.0.0.1:10002/servers'
[{"addr":"127.0.0.1:10001","id":"1"},{"addr":"127.0.0.1:10003","id":"2"},{"addr":"127.0.0.1:10005","id":"3"}]
```