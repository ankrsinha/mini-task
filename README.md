# Mini Task Runner

This project is a simplified, Tekton-like task execution system built using **Kubernetes Custom Resource Definitions (CRDs)** and **Controllers**. It serves as a practical guide for building automated systems that manage containerized workloads through custom resources.

## Key Components

* **Task (CRD)**: A reusable template describing script steps, including names, images, and scripts.


* **TaskRun (CRD)**: An execution instance created when a user starts a specific task.


* **Controller**: A background process that watches for `TaskRun` resources, creates corresponding Pods, and tracks execution status.


* **Kubectl Plugin**: A custom CLI tool (`kubectl-task`) used to trigger runs manually.



---

## Execution Flow

The system operates through a clear lifecycle from definition to execution:

1. **Define**: The user creates a `Task` resource describing the steps.


2. **Trigger**: The user runs `kubectl task start <task-name>`, which creates a `TaskRun` object.


3. **Reconcile**: The controller detects the new `TaskRun` and creates a Pod to execute the defined scripts.


4. **Monitor**: The controller updates the `TaskRun` status based on the Pod's lifecycle.

---


---

## Development Phases

### Phase 1: Custom Resource Definition & Code Gen

* **CRD Creation**: Define `Task` (steps, images, scripts) and `TaskRun` (task references and status).


* **Code Generation**: Use `k8s.io/code-generator` to create clients, listers, and informers.


* **Installation**: Apply CRDs to the cluster using `kubectl apply -f config/crd/bases/`.



### Phase 2: Basic Controller Logic

* **Reconciliation Loop**: Implement a loop that lists `TaskRuns`, creates Pods if they don't exist, and updates status upon completion.


* **Pod Management**: The controller translates `Task` steps into a Kubernetes Pod with a `Never` restart policy.

* **Architecture for Polling based controller**:
  ![Polling.png](https://i.postimg.cc/8CXtTHNP/Polling.png)

### Phase 3: Event-Driven Optimization

* **Informers**: Replace manual polling with a shared informer factory to watch for `TaskRun` and `Pod` events.


* **Workqueue**: Introduce a rate-limited workqueue to handle `TaskRun` keys efficiently.

* **Architecture for Informer based controller**:
![Workflow](https://user-images.githubusercontent.com/4377940/89552246-7cb48a00-d83e-11ea-8c3f-02d7c3400c2c.png)



### Phase 4: CLI Integration

* **Plugin Development**: Build a `kubectl-task` binary that allows users to initiate executions via the command line.



---

## TaskRun Status Phases

The controller tracks the following phases within the `TaskRun` status:

* **Pending**: The initial state before processing.


* **Running**: The execution Pod has started.


* **Succeeded**: The Pod completed its task successfully.


* **Failed**: The execution Pod failed during the task.

---

## Installation

### 1. Clone Repository

```bash
git clone https://github.com/ankrsinha/mini-task
cd mini-task
```

---

### 2. Install CRDs

```bash
kubectl apply -f config/crd/bases/
```

Verify:

```bash
kubectl get crds
```

---

### 3. Generate Clients

```bash
bash hack/update-codegen.sh
```

---

### 4. Run Controller

```bash
go run controller/basic/main.go
```
or 
```bash
go run controller/informer/main.go
```

---

### 5. Install Kubectl Plugin

Move binary to PATH:

```bash
mv kubectl-task /usr/local/bin/
```

---

## Usage

### Create Task

```bash
kubectl apply -f task.yaml
```

### Start Task

```bash
kubectl task start hello
```

### Watch Execution

```bash
kubectl get taskruns -w
```

### Inspect Pod

```bash
kubectl get pods
kubectl logs <pod>
```

### Inspect Status

```bash
kubectl get taskrun <name> -o yaml
```

