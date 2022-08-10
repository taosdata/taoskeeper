FROM golang:1.17.6-alpine as builder
LABEL maintainer = "Linhe Huo <linhe.huo@gmail.com>"

WORKDIR /usr/src/taoskeeper
COPY ./ /usr/src/taoskeeper/
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct
RUN go mod tidy && go build

FROM alpine:3
RUN mkdir -p /etc/taos
COPY --from=builder /usr/src/taoskeeper/taoskeeper /usr/local/bin/
COPY ./config/keeper.toml /etc/taos/keeper.toml
RUN chmod u+rw /etc/taos/keeper.toml
EXPOSE 6043
CMD ["taoskeeper"]
