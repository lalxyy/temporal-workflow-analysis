@echo off
rem Rewrites app\go.mod and app\go.sum. Run it after adding or removing a Go
rem dependency. It runs "go mod tidy" inside Docker, so Go is not needed here.
setlocal
cd /d "%~dp0"

docker build --target modfiles --output type=local,dest=app app
