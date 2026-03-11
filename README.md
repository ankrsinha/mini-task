# Mini Task Runner

A simplified, Tekton-like task execution system built with **Kubernetes Custom Resource Definitions (CRDs)** and a **controller-runtime** based controller. It runs multi-step containerized scripts by creating Pods from `Task` and `TaskRun` resources.

## Key Components

* **Task (CRD)**: A reusable template describing script steps, including names, images, and scripts.
* **TaskRun (CRD)**: An execution instance created when a user starts a specific task.
* **Controller**: A background process that watches for `TaskRun` resources, creates corresponding Pods, and tracks execution status.
* **Kubectl Plugin**: A custom CLI tool (`kubectl-task`) used to trigger runs manually.


## Execution flow

1. **Define** — Create a `Task` with one or more steps (image + script).
2. **Trigger** — Run `kubectl task start <task-name>` (or create a `TaskRun` manually).
3. **Reconcile** — The controller sees the `TaskRun`, resolves the `Task`, and creates a Pod whose containers run the steps.
4. **Monitor** — The controller updates the `TaskRun` status from the Pod’s phase (Pending → Running → Succeeded/Failed).

## TaskRun phases

*(empty)* : New `TaskRun`, not yet processed.
**Pending**: Controller has created the Pod; it may not be running yet.
**Running**: Pod is running.
**Succeeded**: All containers in the Pod completed successfully.
**Failed**: Pod failed or the referenced Task was not found.

## Prerequisites

- Go 1.25+
- A Kubernetes cluster and `kubectl` configured (e.g. `KUBECONFIG` or default kubeconfig).
- For cluster deployment: ability to apply CRDs, RBAC, and a Deployment.

## Installation

### 1. Clone and enter the repo

```bash
git clone https://github.com/ankrsinha/mini-task
cd mini-task
```

### 2. Install CRDs

```bash
kubectl apply -f config/crd/bases/
```

Check that CRDs exist:

```bash
kubectl get crds
```

### 3. Run the controller locally

From the repo root:

```bash
go run cmd/controller/main.go
```

The controller uses your kubeconfig and needs permission to manage `Task`/`TaskRun` and Pods in the target namespace(s). For a quick local test, run with a cluster admin context or use the same RBAC as the deployed controller (see below).

### 4. Build and install the kubectl plugin

```bash
go build -o kubectl-task cmd/kubectl-task/main.go
mv kubectl-task /usr/local/bin/   # or another directory in your PATH
```

Ensure the binary is named `kubectl-task` so that `kubectl task` works.

## Deploying the controller in-cluster

1. **Namespace and RBAC**

```bash
kubectl create namespace mini-task-system
kubectl apply -f config/controller/rbac.yaml
```

2. **Build and push image** (example; adjust registry/tag)

```bash
docker build -t <your-registry>/mini-task-controller:latest .
docker push <your-registry>/mini-task-controller:latest
```

3. **Deploy**

Edit `config/controller/deployment.yaml` and set the controller image to your tag, then:

```bash
kubectl apply -f config/controller/deployment.yaml
```

## Usage

### Create a Task

Example from the repo:

```bash
kubectl apply -f artifacts/task-hello.yaml
```

Or define your own Task (API group `minitask.myorg.dev/v1`, kind `Task`):

```yaml
apiVersion: minitask.myorg.dev/v1
kind: Task
metadata:
  name: my-task
  namespace: default
spec:
  steps:
    - name: step1
      image: bash:latest
      script: |
        echo "Hello from Step 1"
    - name: step2
      image: bash:latest
      script: |
        echo "Hello from Step 2"
```

### Start a run

Using the plugin (creates a TaskRun with generated name in `default` namespace):

```bash
kubectl task start task-hello
```

Or create a TaskRun manually:

```bash
kubectl apply -f artifacts/run-hello.yaml
```

### Watch runs and pods

```bash
kubectl get taskruns -w
kubectl get pods -w
```

### Inspect status and logs

```bash
kubectl get taskrun <name> -o yaml
kubectl get pods
kubectl logs <pod-name> -c <step-name>
```


## Code generation

If you change types under `pkg/apis/minitask/`, regenerate deepcopy and client code:

```bash
bash hack/update-codegen.sh
```

