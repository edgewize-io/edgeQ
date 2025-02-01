# 简介

`Edgewize ModelMesh` 是一款支持多模型服务的推理服务治理框架。`Edgewize ModelMesh` 通过提供多模型共享、流量治理、审计、安全和扩展性等能力。

## 概述

`Edgewize ModelMesh` 关注如下几个问题:

* 多模型硬件共享
* 分级算力供应
* 模型私密性
* 业务的融合
* 云边算力协同



`Edgewize ModelMesh` 的架构图如下：

![`Edgewize ModelMesh` Intro](docs/static/modelmesh-intro.png)


## 架构

![`Edgewize ModelMesh` Arch](docs/static/modelmesh-arch.png)




三个组件的功能分别为：

* ***ModelMesh-Controller***: 用 Go 实现的控制面，提供对数据面组件的管控，如 Sidecar 注入、配置转换和下发，是 `Edgewize ModelMesh` 所有配置的入口。

* ***ModelMesh-Proxy***: 用 Go 实现的高性能量代理，获取应用的推理服务访问流量，并基于此实现流量治理、访问控制、防火墙、可观测性等各种治理能力。

* ***ModelMesh-Broker***: 推理服务数据面，部署在集群中每个节点上并通过宿主机内核的各种能力提供可编程资源管理，如 TrafficQoS 等。

### Broker
![`Edgewize ModelMesh` Arch](docs/static/modelmesh-arch-broker.png)

## 特性

### 推理服务流量治理

应用访问推理服务，`Edgewize ModelMesh` 可以劫持所有的流量。

### 可观测性

推理服务的监控指标通常从相关实例处获取，借助 `Edgewize ModelMesh` 可以透视多种推理服务访问指标。

### 多模式

`Edgewize ModelMesh` 支持多种方式访问，未来还会支持 `MQTT` 等方式。





## 开发手册

### 测试

```
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...@v1.27.7
```

### 本地调试

```
cd tests/e2e
go test

ginkgo bootstrap
```

### 修改 proto

如果修改了 `api/modelfulx/v1alpha` 下的 protobuf 文件，需要用以下命令重新生成对应的 go 文件

```
make proto
```


***注意：修改完 proto 后需要修改`import`***

- `api/modelfulx/v1alpha/modelmesh_service.pb.go`
```
import {
    ...
    proto1 "github.com/edgewize/modelmesh/mindspore_serving/proto"
    ...
}
```

- `api/modelfulx/v1alpha/modelmesh_service.validator.pb.go`
```
import {
    ...
    _ "github.com/edgewize/modelmesh/mindspore_serving/proto"
    ...
}
```