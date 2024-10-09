# rkv

[中文文档](https://github.com/harryyann/rkv/blob/main/doc/README.zh.md)

rkv is a distributed KV system based on raft consensus. It's eliable enough，easy to use、expand ond develop.

## Quick Start

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

You can view the code for further functions.