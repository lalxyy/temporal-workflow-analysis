@echo off
rem Brings up MySQL, Temporal, Prometheus and the freeze helper on the
rem docker-desktop cluster. Safe to run again: every step updates in place.
setlocal
cd /d "%~dp0"
set CTX=docker-desktop
set NS=temporal-lab

echo == Namespace and MySQL
kubectl --context %CTX% apply -f k8s\00-namespace.yaml || goto fail
kubectl --context %CTX% apply -f k8s\10-mysql.yaml || goto fail
kubectl --context %CTX% -n %NS% rollout status statefulset/lab-db --timeout=300s || goto fail

echo == Temporal (database schema setup runs first and can take a few minutes)
helm --kube-context %CTX% -n %NS% upgrade --install temporal charts\temporal-1.8.0.tgz -f k8s\20-temporal-values.yaml --wait --timeout 15m || goto fail

echo == Prometheus and the freeze helper
kubectl --context %CTX% apply -f k8s\30-prometheus.yaml || goto fail
kubectl --context %CTX% apply -f k8s\50-freezer.yaml || goto fail
kubectl --context %CTX% -n %NS% rollout status deploy/prometheus --timeout=180s || goto fail
kubectl --context %CTX% -n %NS% rollout status deploy/freezer --timeout=180s || goto fail

echo.
echo Temporal is up. Next, build and deploy the example workers:
echo   build.cmd v1
echo   kubectl --context %CTX% apply -f k8s\40-app.yaml
exit /b 0

:fail
echo.
echo FAILED. See the output above.
exit /b 1
