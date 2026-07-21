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
set "SDSLV_TEST_HOST=%OUT_DIR%\sdslv_test_host.exe"
set "VSDEVCMD_BAT=C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat"
set "CC=cl"
set "PROMETHEUS_MX5_CONTROL_DEFINE="
if "%PROMETHEUS_DVT2_MX5_VULKAN10_CONTROL%"=="1" set "PROMETHEUS_MX5_CONTROL_DEFINE=/DPROMETHEUS_DVT2_MX5_VULKAN10_CONTROL"
set "PROMETHEUS_M5B_EXPERIMENT_DEFINE="
if "%PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT%"=="1" set "PROMETHEUS_M5B_EXPERIMENT_DEFINE=/DPROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT"
set "PROMETHEUS_M5B_GEMINI_EXACT_DEFINE="
if "%PROMETHEUS_DVT2_M5B_GEMINI_EXACT_EXPERIMENT%"=="1" set "PROMETHEUS_M5B_GEMINI_EXACT_DEFINE=/DPROMETHEUS_DVT2_M5B_GEMINI_EXACT_EXPERIMENT"
set "PROMETHEUS_M5B_GEMINI_INPLACE_DEFINE="
if "%PROMETHEUS_DVT2_M5B_GEMINI_INPLACE_EXPERIMENT%"=="1" set "PROMETHEUS_M5B_GEMINI_INPLACE_DEFINE=/DPROMETHEUS_DVT2_M5B_GEMINI_INPLACE_EXPERIMENT"
set /a PROMETHEUS_M5B_EXPERIMENT_COUNT=0
if "%PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT%"=="1" set /a PROMETHEUS_M5B_EXPERIMENT_COUNT+=1
if "%PROMETHEUS_DVT2_M5B_GEMINI_EXACT_EXPERIMENT%"=="1" set /a PROMETHEUS_M5B_EXPERIMENT_COUNT+=1
if "%PROMETHEUS_DVT2_M5B_GEMINI_INPLACE_EXPERIMENT%"=="1" set /a PROMETHEUS_M5B_EXPERIMENT_COUNT+=1
if %PROMETHEUS_M5B_EXPERIMENT_COUNT% GTR 1 (
  echo error: choose one isolated M5b experimental attention route.
  exit /b 1
)
if defined PROMETHEUS_NATIVE_CC set "CC=%PROMETHEUS_NATIVE_CC%"
set "PUSHD_ACTIVE=0"

if not defined PROMETHEUS_NATIVE_CC where cl >nul 2>nul
if not defined PROMETHEUS_NATIVE_CC if not defined INCLUDE if exist "%VSDEVCMD_BAT%" (
  call "%VSDEVCMD_BAT%" -arch=x64 -host_arch=x64 >nul
)
if not defined PROMETHEUS_NATIVE_CC if errorlevel 1 (
  if exist "%VSDEVCMD_BAT%" (
    call "%VSDEVCMD_BAT%" -arch=x64 -host_arch=x64 >nul
  )
)

if not defined PROMETHEUS_NATIVE_CC where cl >nul 2>nul
if not defined PROMETHEUS_NATIVE_CC if errorlevel 1 (
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
if errorlevel 1 call :fail_now "native manifest parity" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%
call "%NATIVE_DIR%\native_sources_windows.cmd"
if errorlevel 1 call :fail_now "native source manifest" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

pushd "%ROOT_DIR%"
if errorlevel 1 call :fail_now "enter repository root" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%
set "PUSHD_ACTIVE=1"

call "%CC%" /nologo /TC /std:c11 /O2 /W4 /DPROMETHEUS_REACTOR_BUILD_DLL %PROMETHEUS_MX5_CONTROL_DEFINE% %PROMETHEUS_M5B_EXPERIMENT_DEFINE% %PROMETHEUS_M5B_GEMINI_EXACT_DEFINE% %PROMETHEUS_M5B_GEMINI_INPLACE_DEFINE% /c %VULKAN_INCLUDE% /Fo"%OBJ_DIR%\\" %PROMETHEUS_COMMON_C_SRCS%
if errorlevel 1 call :fail_now "compile common native sources" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

link /nologo /DLL %PROMETHEUS_COMMON_OBJS% /OUT:"%REACTOR_DLL%" /IMPLIB:"%REACTOR_LIB%" /PDB:"%REACTOR_PDB%" %VULKAN_LIBPATH% vulkan-1.lib
if errorlevel 1 call :fail_now "link reactor library" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

call "%CC%" /nologo /TC /std:c11 /O2 /W4 %VULKAN_INCLUDE% "%PROMETHEUS_SDSLV_TEST_HOST%" /link /OUT:"%SDSLV_TEST_HOST%" %VULKAN_LIBPATH% vulkan-1.lib
if errorlevel 1 call :fail_now "build SDSL-V test host" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

copy /Y "%REACTOR_DLL%" "%REACTOR_DIR%\prometheus_reactor.dll" >nul
if errorlevel 1 call :fail_now "copy reactor bridge" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

call :build_marionette "%MARIONETTE_EXE%" "%MARIONETTE_PDB%" %PROMETHEUS_MARIONETTE_MAIN% "/DMARIONETTE_EXCLUDE_SLOW_TESTS /DMARIONETTE_EXCLUDE_BENCHMARK_TESTS %PROMETHEUS_MX5_CONTROL_DEFINE% %PROMETHEUS_M5B_EXPERIMENT_DEFINE% %PROMETHEUS_M5B_GEMINI_EXACT_DEFINE% %PROMETHEUS_M5B_GEMINI_INPLACE_DEFINE%" ""
if errorlevel 1 call :fail_now "build Marionette tests" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

if not "%PROMETHEUS_MARIONETTE_SLOW_MAIN%"=="" (
  call :build_marionette "%MARIONETTE_SLOW_EXE%" "%MARIONETTE_SLOW_PDB%" %PROMETHEUS_MARIONETTE_SLOW_MAIN% "/DMARIONETTE_EXCLUDE_BENCHMARK_TESTS" "%PROMETHEUS_MARIONETTE_SLOW_ONLY_SRCS%"
  if errorlevel 1 call :fail_now "build Marionette slow tests" %ERRORLEVEL%
  if errorlevel 1 exit /b %ERRORLEVEL%
)

call :build_marionette "%MARIONETTE_BENCH_EXE%" "%MARIONETTE_BENCH_PDB%" %PROMETHEUS_MARIONETTE_BENCH_MAIN% "" ""
if errorlevel 1 call :fail_now "build Marionette benchmarks" %ERRORLEVEL%
if errorlevel 1 exit /b %ERRORLEVEL%

for %%I in ("%REACTOR_DLL%" "%MARIONETTE_EXE%" "%MARIONETTE_BENCH_EXE%" "%SDSLV_TEST_HOST%") do if not exist "%%~fI" (
  call :fail_now "verify required output %%~fI" 1
  exit /b 1
)

popd
set "PUSHD_ACTIVE=0"

echo Built reactor library: %REACTOR_DLL%
echo Copied for bridge discovery: %REACTOR_DIR%\prometheus_reactor.dll
echo Built Marionette tests: %MARIONETTE_EXE%
echo Built Marionette benchmarks: %MARIONETTE_BENCH_EXE%
echo Built SDSL-V test host: %SDSLV_TEST_HOST%
exit /b 0

:build_marionette
set "OUTPUT_EXE=%~1"
set "OUTPUT_PDB=%~2"
set "MAIN_CPP=%~3"
set "EXTRA_DEFINES=%~4"
set "EXTRA_SRCS=%~5"
call "%CC%" /nologo /TP /std:c++latest /EHsc /O2 /W4 %VULKAN_INCLUDE% /DMARIONETTE_TEST_REPO_ROOT="\"%ROOT_DIR_FORWARD%\"" %EXTRA_DEFINES% ^
  %PROMETHEUS_MARIONETTE_CPP_SRCS% ^
  %EXTRA_SRCS% ^
  "%MAIN_CPP%" ^
  /link %PROMETHEUS_COMMON_OBJS% /OUT:"%OUTPUT_EXE%" /PDB:"%OUTPUT_PDB%" %VULKAN_LIBPATH% vulkan-1.lib
exit /b %ERRORLEVEL%

:fail_now
set "FAILED_STAGE=%~1"
set "RC=%~2"
echo error: Windows native build failed during stage: %FAILED_STAGE% ^(exit code %RC%^).
if "%PUSHD_ACTIVE%"=="1" popd
exit /b %RC%
