# Temporal lab

A local test setup for the analysis outlined in [analysis-sample-outline.md](../analysis-sample-outline.md). It runs Temporal 1.32.2 on the Docker Desktop Kubernetes cluster, with an example workflow, a load generator, and helpers for injecting faults and reading shard ownership.

Everything lives in the `temporal-lab` namespace of the `docker-desktop` context. The scripts name that context explicitly, so they cannot act on another cluster. The `kubectl` commands below use whatever context is current, so check that it is `docker-desktop` before running them.

## What is here

| Path | What it is |
|---|---|
| `up.cmd` | Deploys MySQL, Temporal, Prometheus and the pause helper. Safe to run again. |
| `down.cmd` | Removes the whole lab, including its data. |
| `build.cmd <tag>` | Builds the example worker and load generator into the image `changelab:<tag>`. |
| `tidy.cmd` | Refreshes `app\go.mod` and `app\go.sum` after a dependency change. |
| `charts\temporal-1.8.0.tgz` | The official Temporal Helm chart, saved here so every install uses the same chart. |
| `k8s\10-mysql.yaml` | One MySQL 8.4 instance, named `lab-db`. |
| `k8s\20-temporal-values.yaml` | Helm values: MySQL persistence, 16 history shards, three History pods. |
| `k8s\30-prometheus.yaml` | Prometheus, scraping the Temporal server and the example workers every 5 seconds. |
| `k8s\40-app.yaml` | The example workers (two pods) and the load generator (off until you start it). |
| `k8s\50-freezer.yaml` | The helper that pauses and resumes a pod's processes. |
| `app\approval\` | The example workflow and its activities. This is the code the experiments change. |
| `app\cmd\worker`, `app\cmd\loadgen` | The worker program and the load generator. |

## Requirements

- **Docker Desktop** with Kubernetes enabled and at least 4 GB of memory (Settings, Resources). With the load generator running, the cluster and the lab together use about 2.5 GB. The Hyper-V default of 2 GB is not enough.
- **kubectl and Helm** on the PATH. Helm 4.3.1 was installed with winget; open a new terminal so that `helm` is found.
- **Go is not needed.** The image build and `tidy.cmd` run Go inside Docker. `go.mod` asks for Go 1.26, so a local `go build` with an older Go would download a newer toolchain.

## Start

Run these from this folder. They work in both cmd and PowerShell.

```bat
.\up.cmd
.\build.cmd v1
kubectl apply -f k8s\40-app.yaml
kubectl -n temporal-lab scale deploy/loadgen --replicas=1
```

`up.cmd` takes a few minutes the first time, because it downloads the images. Once they are cached it takes a little over a minute, most of it spent setting up the database schema.

## Watch it

Open the Temporal web UI at http://localhost:8080 after starting:

```bat
kubectl -n temporal-lab port-forward svc/temporal-web 8080:8080
```

Open Prometheus at http://localhost:9090 after starting:

```bat
kubectl -n temporal-lab port-forward svc/prometheus 9090:9090
```

Follow the load generator. It prints one CSV line per call (`time,op,workflow_id,latency_ms,error`), and `error` is empty when the call succeeded:

```bat
kubectl -n temporal-lab logs -f deploy/loadgen
```

Follow both workers. Each activity logs its attempt number, so an activity that ran twice shows up as `attempt 2`:

```bat
kubectl -n temporal-lab logs -f -l app=change-worker --prefix
```

Count workflows by status with the Temporal CLI:

```bat
kubectl -n temporal-lab exec deploy/temporal-admintools -- temporal workflow count --query "ExecutionStatus='Running'"
```

## Tools for the experiments

Print the range ID of every history shard. The range ID goes up each time a History pod acquires a shard. It also goes up when the current owner re-acquires the shard after a storage error, or uses up its block of task IDs, so a higher number does not always mean a new owner. Use the History pods' logs to see who owns a shard:

```bat
kubectl -n temporal-lab exec lab-db-0 -- /lab/shards.sh
```

List the History pods:

```bat
kubectl -n temporal-lab get pods -l app.kubernetes.io/component=history
```

Pause one pod's processes for 30 seconds, then resume them. The helper prints the time of each step and the process state, which reads `T (stopped)` while paused:

```bat
kubectl -n temporal-lab exec deploy/freezer -- /lab/freeze.sh <pod-name> 30
```

Delete a pod gracefully, or add `--grace-period=0 --force` to kill it at once:

```bat
kubectl -n temporal-lab delete pod <pod-name>
```

Restart the workers without changing their code:

```bat
kubectl -n temporal-lab rollout restart deploy/change-worker
```

Roll out changed workflow code. Edit `app\approval\workflow.go`, build it under a new tag, and point the Deployment at that tag:

```bat
.\build.cmd v2
kubectl -n temporal-lab set image deploy/change-worker worker=changelab:v2
```

Go back to the previous version:

```bat
kubectl -n temporal-lab rollout undo deploy/change-worker
```

The load generator's rate and timings are environment variables in `k8s\40-app.yaml`. Change them there and apply the file again.

## Stop and reset

Stop the load and leave everything else running:

```bat
kubectl -n temporal-lab scale deploy/loadgen --replicas=0
```

Remove the lab completely. This deletes the database and every metric Prometheus collected:

```bat
.\down.cmd
```

To start again with an empty database, run `down.cmd` and then `up.cmd`. That is also the only way to change the number of history shards.

## How this differs from the outline

- **One node.** The outline assumed a three-node kind cluster. This cluster has one node, so there is no node drain to test, and all pods share one machine's CPU.
- **Pausing a pod.** The outline planned to send SIGSTOP from the kind node. The pause helper does the same thing from a privileged pod that shares the node's process namespace.
- **Liveness probe.** The chart's liveness probe only opens a TCP connection, which still succeeds while a process is paused. A 4-second pause did not restart the pod; longer pauses have not been tried.

## Things worth knowing

- **Use a new image tag for every build.** Kubernetes reuses an image it already has under the same tag, so rebuilding `v1` changes nothing in the cluster.
- **Prometheus keeps its data in the pod.** Metrics are lost when that pod is replaced. Export what you need before tearing down.
- **`kubectl logs` keeps only recent output.** For a long run, write the load generator's output to a file as it goes: `kubectl -n temporal-lab logs -f deploy/loadgen > run.csv`.
- **The database is called `lab-db`, not `mysql`, on purpose.** Kubernetes gives every pod environment variables for each Service in its namespace. A Service named `mysql` injects `MYSQL_PORT=tcp://<ip>:3306`, and Temporal's schema tool reads `MYSQL_PORT` as a port number and crashes.
- **The password in `k8s\10-mysql.yaml` is for this lab only.**
- **The pause helper is privileged.** That is acceptable on a local test cluster and nowhere else.

## Versions

| Component | Version |
|---|---|
| Kubernetes (Docker Desktop, kind-based) | 1.36.1 |
| Helm | 4.3.1 |
| Temporal Helm chart | 1.8.0 |
| Temporal server and admin tools | 1.32.2 |
| Temporal web UI | 2.56.1 |
| MySQL | 8.4 |
| Prometheus | 3.5.0 |
| Temporal Go SDK | 1.51.0 |
| Go (build image) | 1.27 |
