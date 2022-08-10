# TaosKeeper

taosKeeper 是 TDengine 各项监控指标的导出工具，通过简单的几项配置即可获取 TDengine 的运行状态。

taosKeeper 使用 TDengine RESTful 接口，所以不需要安装 TDengine 客户端即可使用。

## 构建

### 获取源码

从 GitHub 克隆源码：

```sh
git clone https://github.com/taosdata/taoskeeper
cd taoskeeper
```

### 编译

taosKeeper 使用 `GO` 语言编写，在构建前需要配置好 `GO` 语言开发环境。

```sh
go mod tidy
go build
```

## 安装

如果是自行构建的项目，仅需要拷贝 `taoskeeper` 文件到你的 `PATH` 中。

```sh
sudo install taoskeeper /usr/local/bin/
```

## 启动

在启动前，应该做好如下配置：
在 `/etc/taos/keeper.toml` 配置 TDengine 连接参数以及监控指标前缀等其他信息。

```toml
# gin 框架是否启用 debug
debug = false

# 服务监听端口, 默认为 6043
port = 6043

# 日志级别，包含 panic、error、info、debug、trace等
loglevel = "info"

# 程序中使用协程池的大小
gopoolsize = 50000

# 查询 TDengine 监控数据轮询间隔
RotationInterval = "15s"

[tdengine]
host = "127.0.0.1"
port = 6041
username = "root"
password = "taosdata"

# 需要被监控的 taosAdapter
[taosAdapter]
address = ["127.0.0.1:6041","192.168.1.95:6041"]

[metrics]
# 监控指标前缀
prefix = "taos"

# 集群数据的标识符
cluster = "production"

# 存放监控数据的数据库
database = "log"

# 指定需要监控的普通表
tables = ["normal_table"]
```

现在可以启动服务，输入：

```sh
taoskeeper
```

如果你使用 `systemd`，复制 `taoskeeper.service` 到 `/lib/systemd/system/`，并启动服务。

```sh
sudo cp taoskeeper.service /lib/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start taoskeeper
```

让 taosKeeper 随系统开机自启动。

```sh
sudo systemctl enable taoskeeper
```

如果使用 `systemd`，你可以使用如下命令完成安装

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

如下介绍了如何在 docker 中构建 taosKeeper： 

在构建前请配置好 `keeper.toml`。

```dockerfile
FROM golang:1.17.6-alpine as builder

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

如果已经有 taosKeeper 可执行文件，在配置好 `keeper.toml` 后你可以使用如下方式构建:

```dockerfile
FROM ubuntu:18.04
RUN mkdir -p /etc/taos
COPY ./taoskeeper /usr/local/bin/
COPY ./keeper.toml /etc/taos/keeper.toml
EXPOSE 6043
CMD ["taoskeeper"]
```

### 常见问题

* 启动报错，显示connection refused

  **解析**：taosKeeper 依赖 restful 接口查询数据，请检查 taosAdapter 是否正常运行或 keeper.toml 中 taosAdapter 地址是否正确。

* taosKeeper 监控不同 TDengine 显示的检测指标数目不一致？

  **解析**：如果 TDengine 中未创建某项指标，taoskeeper 不能获取对应的检测结果。