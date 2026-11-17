@echo off
rem Removes the whole lab from the docker-desktop cluster, including the MySQL
rem data and every metric Prometheus collected. Images stay in Docker.
setlocal
set CTX=docker-desktop
set NS=temporal-lab

helm --kube-context %CTX% -n %NS% uninstall temporal
kubectl --context %CTX% delete namespace %NS%
