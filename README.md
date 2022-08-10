# TaosKeeper

TDengine Metrics Exporter, you can obtain the running status of TDengine version 3.0 by performing several simple configurations.

This tool uses TDengine RESTful API, so you could just build it without TDengine client.

## Build

### Get the source codes

```sh
git clone https://github.com/taosdata/taoskeeper
cd taoskeeper
```

### compile

```sh
go mod tidy
go build
```

## Install

If you build the tool by your self, just copy the `taoskeeper` binary to your `PATH`.

```sh
sudo install taoskeeper /usr/local/bin/
```

## Start

Before start, you should configure some options like database ip, port or the prefix and others for exported metrics.

in `/etc/taos/keeper.toml`.

```toml
# Start with debug middleware for gin
debug = false

# Listen port, default is 6043
port = 6043

# log level
loglevel = "info"

# go pool size
gopoolsize = 50000

# interval for TDengine metrics
RotationInterval = "15s"

[tdengine]
host = "127.0.0.1"
port = 6041
username = "root"
password = "taosdata"

# list of taosAdapter that need to be monitored
[taosAdapter]
address = ["127.0.0.1:6041","192.168.1.95:6041"]

[metrics]
# metrics prefix in metrics names.
prefix = "taos"

# cluster identifier for multiple TDengine clusters
cluster = "production"

# database for storing metrics data
database = "log"

# export some tables that are not super table
tables = ["normal_table"]
```

Now you could run the tool:

```sh
taoskeeper
```

If you use `systemd`, copy the `taoskeeper.service` to `/lib/systemd/system/` and start the service.

```sh
sudo cp taoskeeper.service /lib/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start taoskeeper
```

To start taoskeeper whenever os rebooted, you should enable the systemd service:

```sh
sudo systemctl enable taoskeeper
```

So if use `systemd`, you'd better install it with these lines all-in-one:

```sh
go mod tidy
go build
sudo install taoskeeper /usr/local/bin/
sudo cp taoskeeper.service /lib/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start taoskeeper
sudo systemctl enable taoskeeper
```

## Docker

Here is an example to show how to build this tool in docker:

Before building, you should configure `keeper.toml`.

```dockerfile
FROM golang:1.17.2 as builder

WORKDIR /usr/src/taoskeeper
COPY ./ /usr/src/taoskeeper/
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct
RUN go mod tidy && go build

FROM alpine:3
RUN mkdir -p /etc/taos
COPY --from=builder /usr/src/taoskeeper/taoskeeper /usr/local/bin/
COPY ./config/keeper.toml /etc/taos/keeper.toml
EXPOSE 6043
CMD ["taoskeeper"]
```

If you already have taosKeeper binary file, you can build this tool like:

```dockerfile
FROM ubuntu:18.04
RUN mkdir -p /etc/taos
COPY ./taoskeeper /usr/local/bin/
COPY ./keeper.toml /etc/taos/keeper.toml
EXPOSE 6043
CMD ["taoskeeper"]
```

### FAQ

* Error occurred: Connection refused, while taosKeeper was starting

  **Answer**: taoskeeper relies on restful interfaces to query data. Check whether the taosAdapter is running or whether
  the taosAdapter address in keeper.toml is correct.

* Why detection metrics displayed by different TDengines inconsistent with taoskeeper monitoring?

  **Answer**: If a metric is not created in TDengine, taoskeeper cannot get the corresponding test results.