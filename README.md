# kv-operator

**kv-operator** is a Kubernetes Operator built using [Kubebuilder](https://book.kubebuilder.io/) that automates the deployment and management of a distributed, Raft-based Key-Value store cluster.

The operator provides a Custom Resource Definition (CRD) called `KVCluster`, which allows users to easily spin up a fully configured stateful database cluster with a specific number of nodes.

## Features

- **Automated Deployment**: Automatically provisions a `StatefulSet` with the requested number of replicas (nodes).
- **Persistent Storage**: Automatically creates Persistent Volume Claims (PVC) for data storage (`/app/raft-data`).
- **Network Configuration**: Configures a Headless Service to enable stable network identities and Raft peer discovery.
- **Raft Bootstrapping**: Automatically injects standard configuration parameters (`NODE_ID`, `SEED_NODE_ID`, `SEED_NODE_ADDR`) to let the cluster seamlessly discover and join the Raft group.

---

## 🚀 Getting Started

### Prerequisites

- Go version v1.22+ or v1.24+
- Docker version 17.03+
- `kubectl` version v1.11.3+
- A running Kubernetes cluster (e.g., [Kind](https://kind.sigs.k8s.io/) or [Minikube](https://minikube.sigs.k8s.io/) for local development)

### Deploying on a cluster

1. **Install the Custom Resource Definitions (CRDs) into the cluster:**

   ```sh
   make install
   ```

2. **Run the controller locally (for development):**

   ```sh
   make run
   ```
   *Note: This will run the controller process outside of the K8s cluster right on your machine, communicating with the cluster via your `kubeconfig`.*

3. **Deploy the Operator to the cluster:**

   If you want to run the Operator as a Pod inside the cluster:
   ```sh
   make docker-build docker-push IMG=<some-registry>/kv-operator:tag
   make deploy IMG=<some-registry>/kv-operator:tag
   ```

### 📦 Creating a `KVCluster`

Once the operator is running (either via `make run` or deployed), you can create an instance of a Key-Value cluster by applying a `KVCluster` custom resource.

Create a file named `my-cluster.yaml`:

```yaml
apiVersion: storage.mydatabase.io/v1alpha1
kind: KVCluster
metadata:
  name: sample-kv-cluster
spec:
  size: 3 # Number of nodes in the Raft cluster
```

Apply it to the cluster:

```sh
kubectl apply -f my-cluster.yaml
```

The operator will immediately intercept this request and create:
- A Headless Service named `sample-kv-cluster-service`.
- A StatefulSet named `sample-kv-cluster` with `3` replicas.
- PVCs providing `1Gi` (default) disks mounted at `/app/raft-data` for each node.

You can verify the created pods:

```sh
kubectl get pods -l app=sample-kv-cluster
```

### 🧹 Uninstalling

**Delete your `KVCluster` instances:**

```sh
kubectl delete kvcluster sample-kv-cluster
```

**Uninstall CRDs:**

```sh
make uninstall
```

**Undeploy the Operator:**

```sh
make undeploy
```

---

## Under the Hood

When a `KVCluster` is created, the Controller manages a `Service` and a `StatefulSet`.

The operator automatically passes the environment variables needed for your Raft storage nodes to join the cluster. For a cluster named `sample-kv-cluster`, it automatically targets `sample-kv-cluster-0` as the initial seed node. 

- **Seed Node ID:** `sample-kv-cluster-0` 
- **Seed Node Address:** `sample-kv-cluster-0.sample-kv-cluster-service.<namespace>.svc.cluster.local:50051`

This ensures that newer replicas successfully join the existing Raft consensus group!

---

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
