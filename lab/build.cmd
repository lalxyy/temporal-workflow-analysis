@echo off
rem Builds the worker and the load generator into the image changelab:<tag>.
rem Use a new tag for every code change, for example: build.cmd v2
setlocal
cd /d "%~dp0"
if "%~1"=="" (
  echo usage: build.cmd ^<tag^>     for example: build.cmd v2
  exit /b 2
)

docker build -t changelab:%~1 --build-arg BUILD_ID=%~1 app || exit /b 1
echo.
echo Built changelab:%~1
