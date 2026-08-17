@echo off
setlocal

cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0rebuild-all.ps1" %*

set "EXIT_CODE=%ERRORLEVEL%"
echo.
if not "%EXIT_CODE%"=="0" (
    echo Build failed with exit code %EXIT_CODE%.
) else (
    echo Build finished successfully.
)
pause
exit /b %EXIT_CODE%
