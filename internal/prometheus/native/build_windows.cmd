@echo off
setlocal EnableExtensions

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..\..\..") do set "ROOT_DIR=%%~fI"
set "NATIVE_DIR=%ROOT_DIR%\internal\prometheus\native"
set "OUT_DIR=%ROOT_DIR%\out\prometheus\native"
set "OBJ_DIR=%OUT_DIR%\obj"
set "REACTOR_DIR=%ROOT_DIR%\internal\prometheus\reactor"
set "REACTOR_DLL=%OUT_DIR%\prometheus_reactor.dll"
set "REACTOR_LIB=%OUT_DIR%\prometheus_reactor.lib"
set "REACTOR_PDB=%OUT_DIR%\prometheus_reactor.pdb"
set "MARIONETTE_EXE=%OUT_DIR%\marionette_tests.exe"
set "MARIONETTE_PDB=%OUT_DIR%\marionette_tests.pdb"
set "MARIONETTE_SLOW_EXE=%OUT_DIR%\marionette_slow_tests.exe"
set "MARIONETTE_SLOW_PDB=%OUT_DIR%\marionette_slow_tests.pdb"
set "MARIONETTE_BENCH_EXE=%OUT_DIR%\marionette_benchmarks.exe"
set "MARIONETTE_BENCH_PDB=%OUT_DIR%\marionette_benchmarks.pdb"
set "VSDEVCMD_BAT=C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat"

where cl >nul 2>nul
if errorlevel 1 (
  if exist "%VSDEVCMD_BAT%" (
    call "%VSDEVCMD_BAT%" -arch=x64 -host_arch=x64 >nul
  )
)

where cl >nul 2>nul
if errorlevel 1 (
  echo error: cl was not found on PATH and Visual Studio developer tools could not be initialized.
  echo error: run this helper from a Visual Studio developer shell or install Visual Studio 2026 Community Build Tools.
  exit /b 1
)

if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"
if not exist "%OBJ_DIR%" mkdir "%OBJ_DIR%"
if not exist "%REACTOR_DIR%" mkdir "%REACTOR_DIR%"

set "VULKAN_INCLUDE="
set "VULKAN_LIBPATH="
if defined VULKAN_SDK (
  if exist "%VULKAN_SDK%\Include\vulkan\vulkan.h" set "VULKAN_INCLUDE=/I%VULKAN_SDK%\Include"
  if exist "%VULKAN_SDK%\Lib\vulkan-1.lib" set "VULKAN_LIBPATH=/LIBPATH:%VULKAN_SDK%\Lib"
)
set "ROOT_DIR_FORWARD=%ROOT_DIR:\=/%"

go run ./tools/prometheus_native_manifest -check
if errorlevel 1 goto :fail
call "%NATIVE_DIR%\native_sources_windows.cmd"

pushd "%ROOT_DIR%"

cl /nologo /TC /std:c11 /O2 /W4 /c %VULKAN_INCLUDE% /Fo"%OBJ_DIR%\\" %PROMETHEUS_COMMON_C_SRCS%
if errorlevel 1 goto :fail

link /nologo /DLL %PROMETHEUS_COMMON_OBJS% /OUT:"%REACTOR_DLL%" /IMPLIB:"%REACTOR_LIB%" /PDB:"%REACTOR_PDB%" %VULKAN_LIBPATH% vulkan-1.lib
if errorlevel 1 goto :fail

copy /Y "%REACTOR_DLL%" "%REACTOR_DIR%\prometheus_reactor.dll" >nul
if errorlevel 1 goto :fail

call :build_marionette "%MARIONETTE_EXE%" "%MARIONETTE_PDB%" %PROMETHEUS_MARIONETTE_MAIN% "/DMARIONETTE_EXCLUDE_SLOW_TESTS /DMARIONETTE_EXCLUDE_BENCHMARK_TESTS" ""
if errorlevel 1 goto :fail

call :build_marionette "%MARIONETTE_SLOW_EXE%" "%MARIONETTE_SLOW_PDB%" %PROMETHEUS_MARIONETTE_SLOW_MAIN% "/DMARIONETTE_EXCLUDE_BENCHMARK_TESTS" "%PROMETHEUS_MARIONETTE_SLOW_ONLY_SRCS%"
if errorlevel 1 goto :fail

call :build_marionette "%MARIONETTE_BENCH_EXE%" "%MARIONETTE_BENCH_PDB%" %PROMETHEUS_MARIONETTE_BENCH_MAIN% "" ""
if errorlevel 1 goto :fail

popd

echo Built reactor library: %REACTOR_DLL%
echo Copied for bridge discovery: %REACTOR_DIR%\prometheus_reactor.dll
echo Built Marionette tests: %MARIONETTE_EXE%
echo Built Marionette slow tests: %MARIONETTE_SLOW_EXE%
echo Built Marionette benchmarks: %MARIONETTE_BENCH_EXE%
exit /b 0

:build_marionette
set "OUTPUT_EXE=%~1"
set "OUTPUT_PDB=%~2"
set "MAIN_CPP=%~3"
set "EXTRA_DEFINES=%~4"
set "EXTRA_SRCS=%~5"
cl /nologo /TP /std:c++latest /EHsc /O2 /W4 %VULKAN_INCLUDE% /DMARIONETTE_TEST_REPO_ROOT="\"%ROOT_DIR_FORWARD%\"" %EXTRA_DEFINES% ^
  %PROMETHEUS_MARIONETTE_CPP_SRCS% ^
  %EXTRA_SRCS% ^
  "%MAIN_CPP%" ^
  /link %PROMETHEUS_COMMON_OBJS% /OUT:"%OUTPUT_EXE%" /PDB:"%OUTPUT_PDB%" %VULKAN_LIBPATH% vulkan-1.lib
exit /b %ERRORLEVEL%

:fail
set "RC=%ERRORLEVEL%"
popd
exit /b %RC%
