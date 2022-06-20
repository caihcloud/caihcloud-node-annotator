#0 ----------------------------
FROM golang:1.18 as builder
WORKDIR /go/src/caihcloud-node-annotator
COPY . /go/src/caihcloud-node-annotator

ENV GOPROXY=https://goproxy.cn,direct
ENV PATH $GOPATH/bin:$PATH

RUN GO111MODULE="on" go build -o node-annotator ./main.go && \
    chmod -R 777 node-annotator

#1 ----------------------------
FROM debian:stretch-slim
COPY --from=builder /go/src/caihcloud-node-annotator/node-annotator /usr/local/bin/node-annotator
ENTRYPOINT ["/usr/local/bin/node-annotator"]
