@echo off
if "%1%"=="deps" (
    call :deps
    goto :eof
)
if "%1%"=="" (
    set cmd=ci
) else (
    set cmd=%1%
)
call :.dapper
.dapper.exe -f Dockerfile-windows.dapper %cmd%
goto :eof

:.dapper
if not exist .dapper.exe (
    bitsadmin /rawreturn /transfer dappwer-download https://releases.rancher.com/dapper/latest/dapper-Windows-x86_64.exe %~dp0\.dapper.exe
    .dapper.exe -v
)
goto :eof
