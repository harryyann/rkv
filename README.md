# rkv

[中文文档](https://github.com/harryyann/rkv/blob/main/doc/README.zh.md)

rkv is a distributed KV system based on raft consensus. It's reliable enough, easy to use and expand.

## Quick Start

The following example can start a three-node rkv cluster.

`Note: Please ensure that ports 10001, 10002, 10003, 10004, 10005, and 10006 are not occupied. You can also specify other ports in the startup parameters.`

### Quick start by docker-compose

You can quickly start a three-node rkv cluster by calling the startup.sh script.
```bash
chmod +x hack/startup.sh
./hack/startup.sh
```

```bash
# docker ps
CONTAINER ID   IMAGE                  COMMAND                   CREATED         STATUS                          PORTS                                  NAMES
f4572975d916   rkvd:v0.0.1            "/app/rkvd --id 3 --…"    3 minutes ago   Up 1 minutes (healthy)          0.0.0.0:10005-10006->10005-10006/tcp   rkvd-node3
0bc09339b783   rkvd:v0.0.1            "/app/rkvd --id 2 --…"    4 minutes ago   Up 2 minutes (healthy)          0.0.0.0:10003-10004->10003-10004/tcp   rkvd-node2
fda902aff12a   rkvd:v0.0.1            "/app/rkvd --id 1"        4 minutes ago   Up 3 minutes (healthy)          0.0.0.0:10001-10002->10001-10002/tcp   rkvd-node1
```
You can try to connect the cluster by curl.

Set a key.
```bash
curl -X POST 'http://127.0.0.1:10002/keys/foo?val=bar'
```

Get the key.
```bash
# curl 'http://127.0.0.1:10002/keys/foo'
bar
```

You also can check how many nodes in cluster.

```bash
# curl 'http://127.0.0.1:10002/servers'
[{"addr":"127.0.0.1:10001","id":"1"},{"addr":"127.0.0.1:10003","id":"2"},{"addr":"127.0.0.1:10005","id":"3"}]
```

Get node's role
```bash
# curl 'http://127.0.0.1:10002/state'
Leader
```

You can check the [design documents](https://yanghairui.life/archives/ji-yu-hashicorp-raftshi-xian-yi-ge-fen-bu-shi-kvxi-tong) and source code to learn more APIs and features.


### Manual binary mode

**1. First build the binary excutable file**
```bash
cd rkv/
go build -o bin/rkvd cmd/server/main.go
```

**2. Start the first node**

```bash
./bin/rkvd --id 1
```

**3. Then start the second and third node**

```bash
./bin/rkvd --id 2 --join 127.0.0.1:10002 --raft-addr 127.0.0.1:10003 --server-addr 127.0.0.1:10004 --data-dir /tmp/rkv2/
```
```bash
./bin/rkvd --id 3 --join 127.0.0.1:10002 --raft-addr 127.0.0.1:10005 --server-addr 127.0.0.1:10006 --data-dir /tmp/rkv3/
```
---
