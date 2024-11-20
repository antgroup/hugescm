#!/usr/bin/env pwsh

Write-Host -ForegroundColor Green "HugeSCM: compiling ..."
$SOURCE_DIR = Split-Path -Path $PSScriptRoot

$InnoCli = Get-Command -ErrorAction SilentlyContinue -CommandType Application "iscc.exe"
if ($null -ne $InnoCli) {
    $InnoSetup = $InnoCli.Path
}
else {
    $InnoSetup = Join-Path ${env:PROGRAMFILES(X86)} -ChildPath 'Inno Setup 6\iscc.exe'
}

$VersionInput = Join-Path $SOURCE_DIR -ChildPath "VERSION"

try {
    $VERSION = Get-Content $VersionInput
    $VERSION = $VERSION.Trim()
}
catch {
    $VERSION = "0.0.1"
}

$HugescmIss = Join-Path $PSScriptRoot -ChildPath "zeta.iss"

$ps = Start-Process -FilePath "go" -WorkingDirectory "install github.com/balibuild/bali/v3/cmd/bali@latest" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}

Write-Host -ForegroundColor Green "HugeSCM: create zip package ..."

$ps = Start-Process -FilePath "bali" -WorkingDirectory $SOURCE_DIR -ArgumentList "--target=windows --arch=amd64 --pack=zip" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}

Write-Host -ForegroundColor Green "HugeSCM: build amd64 install package ..."
$ps = Start-Process -FilePath $InnoSetup -WorkingDirectory $SOURCE_DIR -ArgumentList "`"/dAppVersion=${VERSION}`" `"/dArchitecturesAllowed=x64compatible`" `"/dArchitecturesInstallIn64BitMode=x64compatible`" `"/dInstallTarget=admin`" `"$HugescmIss`"" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}

Write-Host -ForegroundColor Green "HugeSCM: build amd64[user] install package ..."
$ps = Start-Process -FilePath $InnoSetup -WorkingDirectory $SOURCE_DIR -ArgumentList "`"/dAppVersion=${VERSION}`" `"/dArchitecturesAllowed=x64compatible`" `"/dArchitecturesInstallIn64BitMode=x64compatible`" `"/dInstallTarget=user`" `"$HugescmIss`"" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}

$ps = Start-Process -FilePath "bali" -WorkingDirectory $SOURCE_DIR -ArgumentList "--target=windows --arch=arm64 --pack=zip" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}

Write-Host -ForegroundColor Green "HugeSCM: build arm64 install package ..."
$ps = Start-Process -FilePath $InnoSetup -WorkingDirectory $SOURCE_DIR -ArgumentList "`"/dAppVersion=${VERSION}`" `"/dArchitecturesAllowed=arm64`" `"/dArchitecturesInstallIn64BitMode=arm64`" `"/dInstallTarget=admin`" `"$HugescmIss`"" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}

Write-Host -ForegroundColor Green "HugeSCM: build arm64[user] install package ..."
$ps = Start-Process -FilePath $InnoSetup -WorkingDirectory $SOURCE_DIR -ArgumentList "`"/dAppVersion=${VERSION}`" `"/dArchitecturesAllowed=arm64`" `"/dArchitecturesInstallIn64BitMode=arm64`" `"/dInstallTarget=user`" `"$HugescmIss`"" -PassThru -Wait -NoNewWindow
if ($ps.ExitCode -ne 0) {
    Exit $ps.ExitCode
}


Write-Host -ForegroundColor Green "HugeSCM: compile success"