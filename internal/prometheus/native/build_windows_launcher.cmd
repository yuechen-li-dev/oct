@echo off
setlocal EnableExtensions DisableDelayedExpansion

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..\..\..") do set "ROOT_DIR=%%~fI"
set "ARTIFACT_DIR=%ROOT_DIR%\out\test-artifacts"
set "LOG_FILE=%ARTIFACT_DIR%\prometheus_native_windows_build.log"
set "BUILD_SCRIPT=%SCRIPT_DIR%build_windows.cmd"
set "VSWHERE=%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe"
set "VSDEVCMD="

if not exist "%ARTIFACT_DIR%" mkdir "%ARTIFACT_DIR%" >nul 2>nul
>"%LOG_FILE%" echo Prometheus native Windows build launcher
>>"%LOG_FILE%" echo Started (UTC): %DATE% %TIME%
>>"%LOG_FILE%" echo Repository: %ROOT_DIR%

if exist "%VSWHERE%" (
  for /f "usebackq delims=" %%I in (`"%VSWHERE%" -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath`) do (
    if not defined VSDEVCMD if exist "%%I\Common7\Tools\VsDevCmd.bat" set "VSDEVCMD=%%I\Common7\Tools\VsDevCmd.bat"
  )
)

if not defined VSDEVCMD (
  set "VSDEVCMD=C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat"
  if not exist "%VSDEVCMD%" set "VSDEVCMD="
)

if not defined VSDEVCMD (
  >>"%LOG_FILE%" echo error: Visual Studio with x64 C++ tools was not found by vswhere or the supported fallback path.
  >>"%LOG_FILE%" echo error: Expected fallback: C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat
  type "%LOG_FILE%"
  exit /b 1
)

>>"%LOG_FILE%" echo VsDevCmd: %VSDEVCMD%
for /f %%I in ('powershell -NoProfile -Command "[DateTime]::UtcNow.Ticks"') do set "START_TICKS=%%I"

rem VsDevCmd may leave its redirected standard handles open briefly.  Keep its
rem banner on the invoking console; the actual build stdout/stderr below is
rem captured atomically in the deterministic artifact.
call "%VSDEVCMD%" -arch=x64 -host_arch=x64
if errorlevel 1 (
  set "RC=%ERRORLEVEL%"
  >>"%LOG_FILE%" echo error: VsDevCmd failed with exit code %RC%.
  goto :report
)

where cl >>"%LOG_FILE%" 2>&1
if errorlevel 1 (
  set "RC=%ERRORLEVEL%"
  >>"%LOG_FILE%" echo error: VsDevCmd completed but cl.exe is not available on PATH.
  goto :report
)

>>"%LOG_FILE%" echo Running build body: %BUILD_SCRIPT%
rem The only build authority remains build_windows.cmd. `call` preserves its
rem exit code while keeping this launcher available for reporting.
call "%BUILD_SCRIPT%" >>"%LOG_FILE%" 2>&1
set "RC=%ERRORLEVEL%"
>>"%LOG_FILE%" echo Build body returned: %RC%

:report
for /f %%I in ('powershell -NoProfile -Command "([TimeSpan]::FromTicks([DateTime]::UtcNow.Ticks - [Int64]$env:START_TICKS)).TotalSeconds.ToString('F3',[Globalization.CultureInfo]::InvariantCulture)"') do set "ELAPSED_SECONDS=%%I"
>>"%LOG_FILE%" echo Finished (UTC): %DATE% %TIME%
>>"%LOG_FILE%" echo Elapsed seconds: %ELAPSED_SECONDS%
>>"%LOG_FILE%" echo Build exit code: %RC%
if "%RC%"=="0" (
  >>"%LOG_FILE%" echo Built outputs:
  for %%I in ("%ROOT_DIR%\out\prometheus\native\prometheus_reactor.dll" "%ROOT_DIR%\out\prometheus\native\marionette_tests.exe" "%ROOT_DIR%\out\prometheus\native\marionette_slow_tests.exe" "%ROOT_DIR%\out\prometheus\native\marionette_benchmarks.exe") do (
    if exist "%%~fI" >>"%LOG_FILE%" echo   %%~fI
  )
)
type "%LOG_FILE%"
exit /b %RC%
