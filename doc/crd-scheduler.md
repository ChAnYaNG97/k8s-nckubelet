# CRD Scheduler

## Kubernetes Scheduler

Kubernetes Scheduler 会从 Kubernetes API server 那里获取还没有分配节点的那些 Pod，根据调度策略选择一个合适的节点给 Pod。

## Configure Multiple Schedulers

如果在 yaml 文件中，没有指定任何 scheduler name，那么 Kubernetes 会默认用自带的 kube-scheduler 对 Pod 进行调度。如果在 yaml 文件中指定了 `spec.schedulerName`，那么 Kubernetes 会用名称与 `spec.schedulerName` 的值对应的 scheduler 对 Pod 进行调度。

以下是三种指定 scheduler 的情况：

- 不在 Pod spec 中指定 scheduler name，默认用 Kubernetes 自带的 scheduler：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: no-annotation
  labels:
    name: multischeduler-example
spec:
  containers:
  - name: pod-with-no-annotation-container
    image: k8s.gcr.io/pause:2.0
```

- 在 Pod spec 中将 `spec.schedulerName` 的值设为 `default-scheduler`，这是 Kubernetes 自带的 scheduler 的名称，所以还是用 Kubernetes 自带的 scheduler 进行调度：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: annotation-default-scheduler
  labels:
    name: multischeduler-example
spec:
  schedulerName: default-scheduler
  containers:
  - name: pod-with-default-annotation-container
    image: k8s.gcr.io/pause:2.0
```

- 在 Pod spec 中将 `spec.schedulerName` 的值设为 `my-scheduler`，这是我们自定义的任意 scheduler 的名称，Kubernetes 会找到对应名称的 scheduler 进行调度，如果找不到对应的 scheduler，那么 Pod 就保持 Pending 状态：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: annotation-second-scheduler
  labels:
    name: multischeduler-example
spec:
  schedulerName: my-scheduler
  containers:
  - name: pod-with-second-annotation-container
    image: k8s.gcr.io/pause:2.0
```

## CRD Scheduler 实现

### CRD

自定义资源是对 Kubernetes API 的扩展，当创建一个新的自定义资源定义（CRD）时，Kubernetes API Server 通过创建一个新的RESTful资源路径进行应答，Kubernetes 中的每种资源都是对应该资源类型的 API 对象的一个集合。比如，Kubernetes 内置的 pods 资源就是 Pod 对象的集合。

### Custom Scheduler

自定义的 scheduler 可以用任何语言编写，[官方](https://kubernetes.io/blog/2017/03/advanced-scheduling-in-kubernetes/)给了一个用 Bash 脚本写的 scheduler。在原生的 Kubernetes 中，Pod 是 scheduler 的调度单元，而在我们的场景中，因为需要部署非容器化的应用，所以用 CRD 抽象了一个新的资源类型，需要将这个新资源类型的对象作为调度单元，这个调度流程实际上是根据自定义 scheduler 的调度算法，选定一个 Node，然后将 Node 名称更新到 CRD 对象的状态信息中（更新 nodeName 字段）。

### Update API Objects in Place Using kubectl patch

因为 Pod 包含了 binding 这个 subresource，所以在 scheduler 调度 Pod 的过程中，直接通过 binding 对应的 API（`http://$SERVER/api/v1/namespaces/default/pods/$PODNAME/binding/`），以 POST 方式就可以将 scheduler 选择的 Node 与 Pod 绑定，具体代码可在[这里](https://github.com/wsszh/k8s-nckubelet/blob/master/crd-scheduler/pod-scheduler.sh)查看。

但是 CRD 没有包含 binding 这个 subresource，我们需要借助 kubectl patch 将 Node 与 CRD 对象绑定。kubectl patch 分为 strategic merge patch 和 JSON merge patch 两种，只有 Kubernetes 原生的资源类型和 Aggregated APIs 支持 strategic merge patch，CRD 目前还不支持，所以我们通过 JSON merge patch 更新 CRD 对象绑定的 Node 信息。

假设我们自定义了一个叫做 myapp 的资源类型，并创建了一个名为 test-app1 的对象，类型为 myapp，通过 JSON merge patch 命令，就可以将 test-app1 对象 spec 中 nodeName 字段的值更新为 node1，这样就可以实现 CRD 对象与 Node 的绑定，相关信息都存在 etcd 中，我们实现的 nckubelet 通过 API Server 可以找到与其所在 Node 绑定的所有 CRD 对象。

执行命令前，先看一下 test-app1 对象当前的状态信息：

```
Name:         test-app1
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  app.example.com/v1alpha1
Kind:         MyApp
Metadata:
  Cluster Name:
  Creation Timestamp:  2018-11-29T08:29:23Z
  Generation:          1
  Resource Version:    6205423
  Self Link:           /apis/app.example.com/v1alpha1/namespaces/default/myapps/test-app1
  UID:                 dfdb6daa-f3b0-11e8-93ff-000c2966da2a
Spec:
  Input:      Hello World!
Events:       <none>
```

```bash
kubectl patch myapp test-app1 --type merge --patch $'spec:\n nodeName: node1'
```

执行命令后，test-app1 对象 spec 中会增加一个 `Node Name` 字段，执行 `kubectl describe myapp test-app1` 命令，得到 test-app1 对象当前的状态信息：

```
Name:         test-app1
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  app.example.com/v1alpha1
Kind:         MyApp
Metadata:
  Cluster Name:
  Creation Timestamp:  2018-11-29T08:29:23Z
  Generation:          1
  Resource Version:    6205423
  Self Link:           /apis/app.example.com/v1alpha1/namespaces/default/myapps/test-app1
  UID:                 dfdb6daa-f3b0-11e8-93ff-000c2966da2a
Spec:
  Input:      Hello World!
  Node Name:  node1
Events:       <none>
```

如果执行 JSON merge patch 时，字段的值为空，对应的 CRD 对象的 spec 中就会删除对应的字段：

```
kubectl patch myapp test-app1 --type merge --patch $'spec:\n nodeName:'
```

执行这条命令后，可以看到 `Node Name` 字段不见了。

**注意：** `kubectl --server $SERVER`参数用来指定 Kubernetes API server 的 IP 和端口，需要先在 master 上执行 `kubectl proxy &`，默认用 8001 端口作为 Kubernetes API server 的代理，然后在`crd-scheduler.sh`中设置`SERVER=IP:port`，IP 为 master 的地址，端口默认为 8001，可以通过`kubectl proxy [--port=PORT]`修改端口。

`crd-scheduler.sh`的源码在[这里](https://github.com/wsszh/k8s-nckubelet/blob/master/crd-scheduler/crd-scheduler.sh)查看。








