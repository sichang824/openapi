@echo off
setlocal
REM Run from repo root (parent of scripts\) so paths match Makefile expectations.
cd /d "%~dp0.."
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-windows.ps1" %*
set EXITCODE=%ERRORLEVEL%
echo.
if %EXITCODE% neq 0 (
  echo Install failed with exit code %EXITCODE%.
)
pause
endlocal
exit /b %EXITCODE%
