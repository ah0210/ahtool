@echo off
rem ============================================================
rem  ah系统优化工具箱（ahtool）- Windows build script
rem
rem  Usage:
rem    build.cmd            -> bin\ahtool_v1.0.0-dev.exe
rem    build.cmd release    -> bin\ahtool_v1.0.0.exe
rem
rem  Requirements: Go 1.24+ on PATH, or a local toolchain at
rem  _toolchain\go\bin\go.exe. Modules are fetched from GOPROXY
rem  (set GOPROXY=https://goproxy.cn,direct in mainland China).
rem
rem  This script only builds. It does NOT publish, upload, or
rem  rewrite any version manifest.
rem ============================================================
setlocal
cd /d "%~dp0"

set "ROOT=%~dp0"
set "APPVER=1.0.0"
set "GOEXE=go"
if exist "%ROOT%_toolchain\go\bin\go.exe" set "GOEXE=%ROOT%_toolchain\go\bin\go.exe"

set "MODE=dev"
if /i "%~1"=="release" set "MODE=release"
if /i "%~2"=="release" set "MODE=release"

if not exist "%ROOT%bin" mkdir "%ROOT%bin"

set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"

rem Keep a single source of truth for the version: only APPVER above.
for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyy-MM-dd"') do set "BUILDDATE=%%i"

if /i "%MODE%"=="release" (
  set "VERSTR=%APPVER%"
  set "EXE=%ROOT%bin\ahtool_v%APPVER%.exe"
) else (
  set "VERSTR=%APPVER%-dev"
  set "EXE=%ROOT%bin\ahtool_%APPVER%-dev.exe"
)

set "LDFLAGS=-s -w -H windowsgui -X main.appVersion=%VERSTR% -X main.buildDate=%BUILDDATE%"

echo.
echo   Building ahtool [%MODE%]
echo   version = %VERSTR%   date = %BUILDDATE%
echo.

"%GOEXE%" build -buildvcs=false -trimpath -tags desktop,production -ldflags "%LDFLAGS%" -o "%EXE%" .
if errorlevel 1 (
  echo.
  echo   BUILD FAILED - see errors above.
  exit /b 1
)

echo.
echo   OK: %EXE%
echo.
exit /b 0
