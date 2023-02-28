# Astertower Tutorial

This document introduces how to install astertower and use astertower to manage resources in a kubernetes cluster with given yaml files.

Our samples provide an [astro.yaml](https://github.com/kasterism/astertower/samples/astro.yaml) as a demo.

Suppose you start as a beginner, and don't have 'golang', 'docker', 'kubectl', or 'kind', we will start with the installaion of golang, kubectl, docker and kind.

## Install tools
### 1. Install golang
Official document for installation of golang is [here](https://tip.golang.org/doc/install). Below is one way to install golang v1.19.6: 
```bash
wget https://go.dev/dl/go1.19.6.linux-amd64.tar.gz
sudo tar -xzf go1.19.6.linux-amd64.tar.gz -C /usr/local
sudo vim ~/.profile
    export PATH=$PATH:/usr/local/go/bin
source ~/.profile
go version
```
We strongly suggest turning on GO111MODULE and setting GOPROXY by:
```bash
go env -w GO111MODULE=on
go env -w GOPROXY=https://goproxy.io
# $ go env -w GOPROXY=https://goproxy.cn,direct ##
```

### 2. Install kubectl
The Kubernetes command-line tool, kubectl, allows you to run commands against Kubernetes clusters. You can use kubectl to deploy applications, inspect and manage cluster resources, and view logs. For more information including a complete list of kubectl operations, see the [kubectl reference documentation](https://kubernetes.io/docs/reference/kubectl/).

Official document for installation of kubectl is [here](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/). As a kubectl client of newer version can communicate with control planes of older versions, using the latest compatible version of kubectl is a good idea to avoid hidden bugs. For the astro-demo, the version should be kubectl v1.24+.
```bash
curl -LO "https://dl.k8s.io/release/$(curl -L -s \
    https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
kubectl version --client
```

### 3. Install docker
Docker is an open platform for developing, shipping, and running applications. Docker enables you to separate your applications from your infrastructure so you can deliver software quickly. 

Official document for installation of docker is [here](https://docs.docker.com/desktop/install/linux-install/). However, some HIGHER versions of docker may be incompatible with kubectl. To get more infomation and choose a suited version of docker, you can refer to [the CHANGELOG](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.26.md) to see the highest version of docker it supports. To use kubectl v1.26, docker 20.10.18+ is incompatible. 
```bash
sudo apt-get update
sudo apt-get install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | sudo apt-key add -
sudo add-apt-repository \
    "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
    sudo apt-get update
sudo apt-cache madison docker-ce # to get available versions info
# eg. ubuntu 22.10, docker-ce 20.10.21
sudo apt-get install docker-ce=5:20.10.21~3-0~ubuntu-kinetic \
    docker-ce-cli=5:20.10.21~3-0~ubuntu-kinetic containerd.io
docker --version
```

### 4. Install kind
kind is a tool for running local Kubernetes clusters using Docker container “nodes”. kind was primarily designed for testing Kubernetes itself, but may be used for local development or CI.

Official document for quick start of kubectl is [here](https://kind.sigs.k8s.io/docs/user/quick-start/#creating-a-cluster).
```bash
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.17.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

## Using astertower
### 1. use kind to create k8s cluster
Create file kindconfig.yaml in your working space:
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: kindest/node:v1.24.6@sha256:97e8d00bc37a7598a0b32d1fabd155a96355c49fa0d4d4790aab0f161bf31be1
    # Since the controller may run externally, you need kind to map out the access ports
    extraPortMappings:
    - containerPort: 30000
      hostPort: 30000
  - role: worker
    image: kindest/node:v1.24.6@sha256:97e8d00bc37a7598a0b32d1fabd155a96355c49fa0d4d4790aab0f161bf31be1
  - role: worker
    image: kindest/node:v1.24.6@sha256:97e8d00bc37a7598a0b32d1fabd155a96355c49fa0d4d4790aab0f161bf31be1
```
```bash
kind create cluster --name openyurt --config kindconfig.yaml
kubectl get nodes
```
```bash
OUTPUT:
NAME                 STATUS   ROLES           AGE    VERSION
kind-control-plane   Ready    control-plane   178m   v1.24.6
kind-worker          Ready    <none>          178m   v1.24.6
kind-worker2         Ready    <none>          178m   v1.24.6
```

### 2. git clone 
```bash
git clone https://github.com/kasterism/astertower.git
cd astertower
```

### 3. install crd
Use ```make install``` to install crd(Custom Resource Definition). 
```bash
make install
```
```bash
OUTPUT:
kubectl apply -f crds
customresourcedefinition.apiextensions.k8s.io/astros.astertower.kasterism.io created
```

### 4. start controller
Use make run to start astertower controller. Dependences will be downloaded automatically. 
```bash
make run
```
```bash
OUTPUT:
go fmt ./...
go: downloading k8s.io/apimachinery v0.26.0
go: downloading k8s.io/client-go v0.26.0
...
go vet ./...
go: downloading k8s.io/apimachinery v0.26.0
go: downloading github.com/kasterism/astermule v0.1.0-rc
...
go run ./main.go
I0224 05:35:07.357337   25134 astro_controller.go:164] "Starting controller" controller="astro"
I0224 05:35:07.357433   25134 shared_informer.go:273] Waiting for caches to sync for astro
I0224 05:35:07.458311   25134 shared_informer.go:280] Caches are synced for astro
```

### 5. apply yaml
Split terminal and apply yaml file.
```bash
kubectl apply -f crds/samples/astro.yaml 
```
```bash
OUTPUT:
astro.astertower.kasterism.io/astro-demo created
```
use ```kubectl get astro```, ```kubectl get deploy```, ```kubectl get svc```, ```kubectl get pod``` to get info.
```bash
kubectl get astro
```
```bash
OUTPUT:
NAME         PHASE         NODENUMBER   READYNODENUMBER
astro-demo   Initialized   4            2
```
```bash
kubectl get deploy
```
```bash
OUTPUT:
NAME   READY   UP-TO-DATE   AVAILABLE   AGE
a      1/1     1            1           50s
b      1/1     1            1           50s
c      1/1     1            1           50s
d      1/1     1            1           50s
```
```bash
kubectl get svc
```
```bash
OUTPUT:
NAME                   TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
a                      ClusterIP   10.96.232.77    <none>        8000/TCP         69s
astro-demo-astermule   NodePort    10.96.255.72    <none>        8080:30000/TCP   34s
b                      ClusterIP   10.96.224.244   <none>        8001/TCP         69s
c                      ClusterIP   10.96.158.167   <none>        8002/TCP         69s
d                      ClusterIP   10.96.155.24    <none>        8003/TCP         69s
kubernetes             ClusterIP   10.96.0.1       <none>        443/TCP          81m
```
```bash
kubectl get pod
```
```bash
OUTPUT:
NAME                   READY   STATUS    RESTARTS   AGE
a-58d79c8c84-fms94     1/1     Running   0          2m19s
astro-demo-astermule   1/1     Running   0          104s
b-9f9dc8f5d-gcrfz      1/1     Running   0          2m19s
c-5fbf5d95f6-gvpnk     1/1     Running   0          2m19s
d-57dd5654cd-zm9kr     1/1     Running   0          2m19s
```
```bash
kubectl get astro
```
```bash
OUTPUT:
NAME         PHASE   NODENUMBER   READYNODENUMBER
astro-demo   Success 4            4
```
```bash
kubectl get astro astro-demo -o yaml
```
```bash
OUTPUT:
...
result:
  data:
    {"age":"22","city":"Shanghai","country":"China","gender":"male","name":"myname"}
  status:
    health:true
...
```

### 6. delete yaml
```bash
kubectl delete -f crds/samples/astro.yaml
```
```bash
OUTPUT:
astro.astertower.kasterism.io "astro-demo" deleted
```

### 7. close controller
Return to the oringin terminal and press 'Ctrl'+'C' to stop controller. 

### 8. uninstall crd
Use ```make uninstall``` to uninstall crd. 
```bash
make uninstall
```
```bash
OUTPUT:
kubectl delete -f crds
customresourcedefinition.apiextensions.k8s.io "astros.astertower.kasterism.io" deleted
```
