@echo off
setlocal EnableExtensions

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%..\..\..") do set "ROOT_DIR=%%~fI"
set "NATIVE_DIR=%ROOT_DIR%\internal\prometheus\native"
set "OUT_DIR=%ROOT_DIR%\out\prometheus\native"
set "REACTOR_DIR=%ROOT_DIR%\internal\prometheus\reactor"
set "REACTOR_DLL=%OUT_DIR%\prometheus_reactor.dll"
set "REACTOR_LIB=%OUT_DIR%\prometheus_reactor.lib"
set "REACTOR_PDB=%OUT_DIR%\prometheus_reactor.pdb"
set "MARIONETTE_EXE=%OUT_DIR%\marionette_tests.exe"
set "MARIONETTE_PDB=%OUT_DIR%\marionette_tests.pdb"

where cl >nul 2>nul
if errorlevel 1 (
  echo error: cl was not found on PATH. Run this helper from a Visual Studio developer shell.
  exit /b 1
)

if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"
if not exist "%REACTOR_DIR%" mkdir "%REACTOR_DIR%"

set "VULKAN_INCLUDE="
set "VULKAN_LIBPATH="
if defined VULKAN_SDK (
  if exist "%VULKAN_SDK%\Include\vulkan\vulkan.h" set "VULKAN_INCLUDE=/I%VULKAN_SDK%\Include"
  if exist "%VULKAN_SDK%\Lib\vulkan-1.lib" set "VULKAN_LIBPATH=/LIBPATH:%VULKAN_SDK%\Lib"
)

pushd "%ROOT_DIR%"

cl /nologo /std:c11 /O2 /W4 /LD %VULKAN_INCLUDE% /DPROMETHEUS_REACTOR_BUILD_DLL ^
  "%NATIVE_DIR%\reactor_api.c" ^
  "%NATIVE_DIR%\reactor_vulkan.c" ^
  /link /OUT:"%REACTOR_DLL%" /IMPLIB:"%REACTOR_LIB%" /PDB:"%REACTOR_PDB%" %VULKAN_LIBPATH% vulkan-1.lib
if errorlevel 1 goto :fail

copy /Y "%REACTOR_DLL%" "%REACTOR_DIR%\prometheus_reactor.dll" >nul
if errorlevel 1 goto :fail

cl /nologo /std:c++latest /EHsc /O2 /W4 %VULKAN_INCLUDE% /DMARIONETTE_TEST_REPO_ROOT="\".\"" ^
  "%NATIVE_DIR%\reactor_api.c" ^
  "%NATIVE_DIR%\reactor_vulkan.c" ^
  "%NATIVE_DIR%\Marionette\test_main.cpp" ^
  "%NATIVE_DIR%\Marionette\test_harness.cpp" ^
  "%NATIVE_DIR%\Marionette\test_doom.cpp" ^
  "%NATIVE_DIR%\Marionette\smoke_tests.cpp" ^
  "%NATIVE_DIR%\Marionette\test_harness_phase1_tests.cpp" ^
  "%NATIVE_DIR%\Marionette\test_harness_doom_tests.cpp" ^
  "%NATIVE_DIR%\Marionette\reactor_stub_tests.cpp" ^
  /link /OUT:"%MARIONETTE_EXE%" /PDB:"%MARIONETTE_PDB%" %VULKAN_LIBPATH% vulkan-1.lib
if errorlevel 1 goto :fail

popd

echo Built reactor library: %REACTOR_DLL%
echo Copied for bridge discovery: %REACTOR_DIR%\prometheus_reactor.dll
echo Built Marionette tests: %MARIONETTE_EXE%
exit /b 0

:fail
set "RC=%ERRORLEVEL%"
popd
exit /b %RC%
