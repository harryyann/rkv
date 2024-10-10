# rkv

[中文文档](https://github.com/harryyann/rkv/blob/main/doc/README.zh.md)

rkv is a distributed KV system based on raft consensus. It's eliable enough，easy to use、expand ond develop.

## Quick Start

The following example can start a three-node rkv cluster.

`Note: Please ensure that ports 10001, 10002, 10003, 10004, 10005, and 10006 are not occupied. You can also specify other ports in the startup parameters.`

### Manual binary mode

1. First build the binary excutable file
```bash
cd rkv/
go build -o bin/rkvd cmd/server/main.go
```

2. Start the filed node

```bash
./bin/rkvd --id 1
```

3. Then start the second and third node

```bash
./bin/rkvd --id 2 --join 127.0.0.1:10002 --raft-addr 127.0.0.1:10003 --server-addr 127.0.0.1:10004 --data-dir /tmp/rkv2/
```
```bash
./bin/rkvd --id 3 --join 127.0.0.1:10002 --raft-addr 127.0.0.1:10005 --server-addr 127.0.0.1:10006 --data-dir /tmp/rkv3/
```

4. Try set a key connect to the leader

set a key
```bash
curl -X POST 'http://127.0.0.1:10002/keys/foo?val=bar'
```

get the key
```bash
# curl 'http://127.0.0.1:10002/keys/foo'
bar
```
5. You can check the servers in cluster

```bash
# curl 'http://127.0.0.1:10002/servers'
[{"addr":"127.0.0.1:10001","id":"1"},{"addr":"127.0.0.1:10003","id":"2"},{"addr":"127.0.0.1:10005","id":"3"}]
```

### Manual docker mode 

1. Build the image
```bash
docker build -t rkvd:v0.0.1 .
```
2. Start the first rkv node
```bash
docker run -di --network=host --name rkvd-node-1 rkvd:v0.0.1 --id 1
```
3. Start the second and third nodes
```bash
docker run -di --network=host --name rkvd-node-2 rkvd:v0.0.1 --id 2 --raft-addr 127.0.0.1:10003 --server-addr 127.0.0.1:10004 --join 127.0.0.1:10002
```

```bash
docker run -di --network=host --name rkvd-node-3 rkvd:v0.0.1 --id 3 --raft-addr 127.0.0.1:10005 --server-addr 127.0.0.1:10006 --join 127.0.0.1:10002
```

All containers are as follows:
```bash
# docker ps                                                                                                                                           
CONTAINER ID   IMAGE                  COMMAND                  CREATED              STATUS              PORTS                       NAMES
b3dfe0b31e83   rkvd:v0.0.1            "/app/rkvd --id 3 --…"   16 seconds ago       Up 15 seconds                                   rkvd-node-3
0c50af113c9d   rkvd:v0.0.1            "/app/rkvd --id 2 --…"   About a minute ago   Up About a minute                               rkvd-node-2
3c29bd84e48c   rkvd:v0.0.1            "/app/rkvd --id 1"       3 minutes ago        Up 3 minutes                                    rkvd-node-1
```
---

You can check the [design documents]() and source code to learn more about the features.