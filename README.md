# Mini Task Runner — Controller Runtime Version 

## Overview

Mini Task Runner extends Kubernetes by introducing:

* **Task** → defines reusable execution steps
* **TaskRun** → triggers execution
* **Controller** → ensures execution via Pods

This version rebuilds the controller using **controller-runtime**, replacing manual controller wiring from the earlier implementation.

---

## Key Components

* **Task (CRD)**: A reusable template describing steps (image + script)

* **TaskRun (CRD)**: Represents an execution instance of a Task

* **Controller (controller-runtime based)**: Watches TaskRuns and Pods, ensures execution

* **Kubectl Plugin (`kubectl-task`)**: CLI tool to trigger TaskRuns easily

---

## Execution Flow

The system operates through a clear lifecycle from definition to execution:

1. **Define**: The user creates a `Task` resource describing the steps.


2. **Trigger**: The user runs `kubectl task start <task-name>`, which creates a `TaskRun` object.


3. **Reconcile**: The controller detects the new `TaskRun` and creates a Pod to execute the defined scripts.


4. **Monitor**: The controller updates the `TaskRun` status based on the Pod's lifecycle.

---

## Limitations of Previous Implementation (client-go)

* **High Boilerplate**: Extensive setup for informers, workqueues, and worker loops

* **Manual Event Handling**: Explicit wiring and mapping of resource events

* **Workqueue Overhead**: Retry logic and rate limiting handled manually

* **Complex Resource Mapping**: Additional logic required to relate Pods with TaskRuns

* **Reduced Maintainability**: Infrastructure-heavy code made extension and debugging harder

---

## Architecture (controller-runtime)

Migrating to **controller-runtime** simplifies the controller design by abstracting low-level components into a few core building blocks:

* **Manager**
* **Controller Builder**
* **Reconciler**
* **Cached Client**
* **Automatic Workqueue**

---

### Manager

The **Manager** is the entry point of the controller-runtime application.

It acts as the central runtime responsible for:

* Starting and running controllers
* Maintaining shared caches (via informers)
* Providing Kubernetes clients
* Managing controller lifecycle

```go
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    Scheme: scheme,
})
```

Once started, the manager initializes all registered controllers and keeps them running continuously.

---

### Controller Builder

Controllers are registered using the **Controller Builder Pattern**, which declaratively defines what resources to watch:

```go
ctrl.NewControllerManagedBy(mgr).
    For(&miniv1.TaskRun{}).
    Owns(&corev1.Pod{}).
    Complete(&TaskRunReconciler{})
```

* `For(&TaskRun{})` → Primary resource being reconciled
* `Owns(&Pod{})` → Watch Pods created by the controller

This ensures:

* Any change in a `TaskRun` triggers reconciliation
* Any update to owned Pods automatically re-enqueues the corresponding TaskRun

---

### Reconciler

The **Reconciler** is the core of the controller logic.

Instead of managing worker loops manually, controller-runtime invokes:

```go
Reconcile(ctx, req)
```

on every relevant event.

Responsibilities:

* Fetch the current state of the `TaskRun`
* Compare actual vs desired state
* Take actions to converge the system

In this project:

* If TaskRun is new → create Pod
* If Pod is running → update status
* If Pod completes → mark success or failure

The reconciler is designed to be **idempotent**, meaning repeated executions produce consistent results.

---

### Cached Client

controller-runtime provides a **cached Kubernetes client**.

* Reads are served from a **local cache (informer-backed)**
* Reduces API server load
* Improves performance

Example:

```go
var tr miniv1.TaskRun
r.Get(ctx, req.NamespacedName, &tr)
```

The framework handles cache synchronization internally.

---

### Automatic Workqueue

Unlike client-go, the workqueue is **fully managed by controller-runtime**.

It automatically:

* Enqueues requests on:

  * TaskRun creation/update
  * Pod updates
* Handles:

  * Retry logic
  * Rate limiting
  * Worker execution

This eliminates the need to manually implement queues and significantly reduces boilerplate.

---

## Ownership Model

```go
ctrl.SetControllerReference(tr, pod, r.Scheme)
```

Ensures:

* Pods are owned by TaskRun
* Garbage collection
* Automatic reconciliation on Pod updates

---

## client-go vs controller-runtime

| Aspect             | client-go (fork1) | controller-runtime (fork2) |
| ------------------ | ----------------- | -------------------------- |
| Setup              | Manual            | Minimal                    |
| Informers          | Explicit          | Abstracted                 |
| Workqueue          | Manually managed  | Built-in                   |
| Event handling     | Custom wiring     | Declarative                |
| Boilerplate        | High              | Low                        |
| Ownership handling | Manual            | Built-in                   |
| Retry logic        | Manual            | Automatic                  |

---

## Benefits Observed

### 1. Reduced Boilerplate

* No manual informer setup
* No worker loop management

---

### 2. Built-in Retry Mechanism

* Automatic requeue on errors
* Exponential backoff handled internally

---

### 3. Cleaner Design

* Focus shifts to reconciliation logic
* Less infrastructure code

---

### 4. Easier Ownership Handling

* Native support for parent-child relationships
* Automatic garbage collection

---

### 5. Better Maintainability

* Easier to extend
* Aligns with production-grade controllers

---

## Prerequisites

* Go **1.25+**
* Kubernetes cluster
* `kubectl` configured
* Permissions to apply CRDs, RBAC, Deployment

---

## Installation & Setup

### Clone Repository

```bash
git clone https://github.com/ankrsinha/mini-task
cd mini-task
```

---

### Install CRDs

```bash
kubectl apply -f config/crd/bases/
kubectl get crds
```

---

## Code Generation

```bash
bash hack/update-codegen.sh
```

---

### Install Kubectl Plugin

```bash
go build -o kubectl-task cmd/kubectl-task/main.go
mv kubectl-task /usr/local/bin/
```

---

## Deploying In-Cluster

### Namespace & RBAC

```bash
kubectl create namespace mini-task-system
kubectl apply -f config/controller/rbac.yaml
```

---

### Build & Push Image (GHCR)

```bash
docker build -t ghcr.io/<your-username>/mini-task-runner:fork2 .
docker push ghcr.io/<your-username>/mini-task-runner:fork2
```

---

### Deploy Controller

```bash
kubectl apply -f config/controller/deployment.yaml
```

---

## Usage

### Create Task

```bash
kubectl apply -f artifacts/task-hello.yaml
```

---

### Start TaskRun

```bash
kubectl task start task-hello
```

or

```bash
kubectl apply -f artifacts/run-hello.yaml
```

---

### Monitor Execution

```bash
kubectl get taskruns -w
kubectl get pods -w
```

---

### Inspect Logs

```bash
kubectl logs <pod-name> -c <step-name>
```

---

## Final Outcome

| Metric               | Result   |
| -------------------- | -------- |
| Code Complexity      | Reduced  |
| Boilerplate          | Minimal  |
| Performance          | Improved |
| Maintainability      | High     |
| Production Readiness | Strong   |

---

## 🏁 Conclusion

Migrating from **client-go → controller-runtime** shifts development from:

> 🔧 Infrastructure-heavy → Logic-focused

This version aligns closely with **real-world Kubernetes operators**, making it more scalable and production-ready.

---
