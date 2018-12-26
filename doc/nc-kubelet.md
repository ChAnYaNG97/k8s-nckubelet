# NC-Kubelet

## 简介

在 Kubernetes 中，kubelet 是在每个节点上运行的主要 “节点代理”，主要通过 master 上的 API Server 获取需要创建的 Pod 清单，执行创建 Pod 并启动容器的相关操作，并对运行的容器进行生命周期管理。

而这里实现的 NC-Kubelet 可以理解为 Non-Containerized Application Kubelet，它可以定时从 Kubernetes API Server 获取非容器化应用对象的清单，在节点上部署非容器化应用，并对部署的应用进行生命周期管理。

## 自定义资源类型

对于 Kubernetes 中的 kubelet 而言，其管理的对象是 Pod 类型的，而对于 NC-Kubelet 而言，其管理的对象是非容器化应用，因此我们需要先将非容器化应用抽象成一种新的资源类型，通过 CustomResourceDefinition (CRD) 实现，资源类型名称为 NCApp，对应的 `crd.yaml` 文件内容如下：

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: ncapps.app.example.com
spec:
  group: ncapp.example.com
  names:
    kind: NCApp
    listKind: NCAppList
    plural: ncapps
    singular: ncapp
  scope: Namespaced
  version: v1alpha1
```

如果要创建一个类型为 NCApp 对象，对象名为 test-ncapp，对应的 `test-ncapp1.yaml` 文件内容如下：

```yaml
apiVersion: app.example.com/v1alpha1
kind: NCApp
metadata:
  name: test-ncapp1
spec:
  # Add fields here
  shellFile: ""
  cpuPCT: ""
  memPCT: ""
```

- `shellFile`表示部署并启动非容器化应用的 shell 脚本，可以是本地文件系统的路径地址，也可以是 URL 地址
- `cpuPCT`表示当前 NCApp 对象占用系统 CPU 资源的百分比
- `memPCT`表示当前 NCApp 对象占用系统内存资源的百分比

可以根据实际场景添加或删除 spec 中的字段。

## 非容器化应用管理

### 创建 NCApp 对象

有了`crd.yaml`，就可以通过 kubectl 创建自定义资源类型：

```bash
kubectl create -f crd.yaml
```

创建成功后，Kubernetes API Server 就会产生一个新的 RESTful API 端点：

```bash
/apis/ncapp.example.com/v1alpha1/namespaces/default/ncapps
```

然后我们就可以用 kubectl 像操作 Pod 对象那样操作 NCApp 对象，首先创建一个 NCApp 对象，在创建对象之前，我们需要先在 master 上启动 [CRD Scheduler](https://github.com/wsszh/k8s-nckubelet/blob/master/doc/crd-scheduler.md)，因为 Kubernetes 原生的 scheduler 支持 Pod 对象的调度，所以我们单独实现了一个基于 CRD 对象的 scheduler。CRD Scheduler 启动成功后，就可以创建一个新的 NCApp 对象：

```bash
kubectl create -f test-ncapp1.yaml
```

创建成功后，CRD Scheduler 就会从 API Server 获取到新创建对象的信息，如果该对象还没有被调度到任何节点上，CRD Scheduler 就会根据调度策略将新对象到某个节点上。

### 获取 NCApp 对象清单

通过 kubectl 创建的所有对象的数据都保存在 etcd 中，而 Kubernetes 中的所有组件都是通过 API Server 访问 etcd 中的数据的，NC-Kubelet 也遵循这一规范，通过刚刚创建的 RESTful API 端点 `/apis/ncapp.example.com/v1alpha1/namespaces/default/ncapps` 获取集群中 NCApp 对象的清单。

在默认配置中，NC-Kubelet 每隔十秒想 API Server 发起 HTTP GET 请求，获取最新的 NCApp 对象清单。

### NCApp 对象的生命周期管理

NC-Kubelet 在得到 NCApp 对象清单后，其处理逻辑如下：

- 会过滤那些绑定在其他节点上的 NCApp 对象
- 如果本地运行的 NCApp 对象不在获取的清单中，则停止该对象的运行，删除本地对应的状态信息
- 如果清单中的 NCApp 对象还没有在本地运行，则根据对象中的信息得到部署脚本，部署并启动该对象对应的非容器化应用，将应用相关的状态信息保存在本地
- 如果清单中的 NCApp 对象已经在本地运行，但清单中的信息与本地不一致，则根据清单中的信息对应用进行相应调整，并更新本地的状态信息

### NCApp 对象的资源监控

NC-Kubelet 会定时获取本地运行的 NCApp 对象系统资源的占用情况，并将相关数据汇总给 API Server。

对系统资源占用情况的监控通过开源工具 [gopsutil](https://github.com/shirou/gopsutil) 实现，因为本地保存了每个 NCApp 对象对应的非容器化应用在系统中运行的进程ID，所以可以通过进程ID获取系统资源的占用情况。

[gopsutil](https://github.com/shirou/gopsutil) 是一个跨平台的、能够获取运行进程和系统资源占用的相关信息，可以借助它实现许多功能：

- 系统监控
- 性能分析
- 限制进程资源
- 对运行中的进程进行生命周期管理

将系统资源占用的数据汇总给 API Server，实际上就是更新 NCApp 对象保存在 etcd 中的相关字段的值，是对 NCApp 对象的局部更新，因此通过 HTTP Patch 请求实现。

在 Golang 的`net/http`包中，没有提供对 HTTP Patch 直接调用的方法，需要自己构造 HTTP Patch 请求，请求头至少需要包含以下三个字段：

```go
    req.Header.Set("Accept", "application/json")
    req.Header.Set("Content-Type", "application/merge-patch+json")
    req.Header.Set("User-Agent", "kubectl/v1.10.0 (linux/amd64) kubernetes/fc32d2f")
```

## 架构图

![](https://raw.githubusercontent.com/wsszh/k8s-nckubelet/master/pic/architecture.png)

NC-Kubelet 运行在每个需要部署非容器化应用的节点上，整体的工作流程如下：

- 通过`crd.yaml`文件，自定义一种名为 NCApp 的新资源类型
- 创建一个类型为 NCApp 的资源对象
- Kubernetes API Server 接受创建对象的请求，产生一个新的 RESTful API 端点
- CRD Scheduler 会根据调度策略将应用对象调度到最优的工作节点上
- NC-Kubelet 定时从 API Server 获取调度到其所在节点上的非容器化应用清单，并与节点正在运行的应用进行对比，进行部署和删除等操作
- NC-Kubelet 定时获取运行中的应用进程的资源利用等状态信息，并上报给 API Server








































