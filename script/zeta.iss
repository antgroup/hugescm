;; abc windows install package
;; Please Inno Setup >= 6.2.0
#if Ver < EncodeVer(6,1,0,0)
  #error This script requires Inno Setup 6 or later
#endif

#ifndef AppVersion
  #define AppVersion GetVersionNumbersString(AddBackslash(SourcePath) + "..\build\bin\zeta.exe")
#endif

#ifndef AppVerName
#define AppVerName "Zeta"
#endif

#ifndef ArchitecturesAllowed
  #define ArchitecturesAllowed "x64"
#endif

#ifndef ArchitecturesInstallIn64BitMode
  #define ArchitecturesInstallIn64BitMode "x64"
#endif

#ifndef InstallTarget
  #define InstallTarget "user"
#endif

#ifndef AppUserId
  #define AppUserId "Zeta"
#endif

#if "" == ArchitecturesInstallIn64BitMode
  #define BaseNameSuffix "ia32"
#elif "x64compatible" == ArchitecturesInstallIn64BitMode
  #define BaseNameSuffix "x64"
#else
  #define BaseNameSuffix ArchitecturesInstallIn64BitMode
#endif

[Setup]
AppId=B900C2E0-BA92-44A5-8445-F2F488927B49
AppName=Zeta
AppVersion={#AppVersion}
AppPublisher=HugeSCM contributors
AppPublisherURL=https://zeta.example.io
AppSupportURL=https://zeta.example.io
LicenseFile=..\LICENSE
WizardStyle=modern
DefaultGroupName=Zeta
Compression=lzma2
SolidCompression=yes
OutputDir=..\out
ChangesEnvironment=true
; "ArchitecturesAllowed=x64" specifies that Setup cannot run on
; anything but x64.
; "ArchitecturesInstallIn64BitMode=x64" requests that the install be
; done in "64-bit mode" on x64, meaning it should use the native
; 64-bit Program Files directory and the 64-bit view of the registry.
ArchitecturesAllowed={#ArchitecturesAllowed}
ArchitecturesInstallIn64BitMode={#ArchitecturesInstallIn64BitMode}
; version info
VersionInfoCompany=HugeSCM contributors
VersionInfoVersion={#AppVersion}
VersionInfoCopyright=Copyright © 2024. HugeSCM contributors

#if "user" == InstallTarget
AppVerName=Zeta (User)
VersionInfoDescription=Zeta User Installer
DefaultDirName={userpf}\Zeta
PrivilegesRequired=lowest
OutputBaseFilename=zeta-user-{#AppVersion}-{#BaseNameSuffix}
VersionInfoOriginalFileName=ZetaUserSetup-{#BaseNameSuffix}.exe
#else
AppVerName=Zeta
VersionInfoDescription=Zeta System Installer
DefaultDirName={commonpf}\Zeta
OutputBaseFilename=zeta-{#AppVersion}-{#BaseNameSuffix}
VersionInfoOriginalFileName=ZetaSetup-{#BaseNameSuffix}.exe
UsedUserAreasWarning=no
#endif

UninstallDisplayIcon={app}\bin\zeta.exe

[Files]
Source: "..\build\bin\zeta.exe"; DestDir: "{app}\bin"; DestName: "zeta.exe"
Source: "..\build\bin\zeta-mc.exe"; DestDir: "{app}\bin"; DestName: "zeta-mc.exe"
Source: "..\build\share\zeta\LEGAL.md"; DestDir: "{app}\share"; DestName: "LEGAL.md"


[Tasks]
Name: "addtopath"; Description: "Add to PATH (requires shell restart)"; GroupDescription: "Other:"

[Registry]
#define SoftwareClassesRootKey "HKLM"

; Environment
#define EnvironmentRootKey "HKLM"
#define EnvironmentKey "System\CurrentControlSet\Control\Session Manager\Environment"
#define Uninstall64RootKey "HKLM64"
#define Uninstall32RootKey "HKLM32"

Root: {#EnvironmentRootKey}; Subkey: "{#EnvironmentKey}"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}\bin"; Tasks: addtopath; Check: NeedsAddPath(ExpandConstant('{app}\bin'))

Root: HKLM; Subkey: Software\Zeta; ValueType: string; ValueName: CurrentVersion; ValueData: {#AppVersion}; Flags: uninsdeletevalue uninsdeletekeyifempty; Check: IsAdminInstallMode
Root: HKLM; Subkey: Software\Zeta; ValueType: string; ValueName: InstallPath; ValueData: {app}; Flags: uninsdeletevalue uninsdeletekeyifempty; Check: IsAdminInstallMode
Root: HKCU; Subkey: Software\Zeta; ValueType: string; ValueName: CurrentVersion; ValueData: {#AppVersion}; Flags: uninsdeletevalue uninsdeletekeyifempty; Check: not IsAdminInstallMode
Root: HKCU; Subkey: Software\Zeta; ValueType: string; ValueName: InstallPath; ValueData: {app}; Flags: uninsdeletevalue uninsdeletekeyifempty; Check: not IsAdminInstallMode


[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue({#EnvironmentRootKey}, '{#EnvironmentKey}', 'Path', OrigPath)
  then begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;


// https://stackoverflow.com/a/23838239/261019
procedure Explode(var Dest: TArrayOfString; Text: String; Separator: String);
var
  i, p: Integer;
begin
  i := 0;
  repeat
    SetArrayLength(Dest, i+1);
    p := Pos(Separator,Text);
    if p > 0 then begin
      Dest[i] := Copy(Text, 1, p-1);
      Text := Copy(Text, p + Length(Separator), Length(Text));
      i := i + 1;
    end else begin
      Dest[i] := Text;
      Text := '';
    end;
  until Length(Text)=0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path: string;
  ThisAppPath: string;
  Parts: TArrayOfString;
  NewPath: string;
  i: Integer;
begin
  if not CurUninstallStep = usUninstall then begin
    exit;
  end;
  if not RegQueryStringValue({#EnvironmentRootKey}, '{#EnvironmentKey}', 'Path', Path)
  then begin
    exit;
  end;
  NewPath := '';
  ThisAppPath := ExpandConstant('{app}\bin')
  Explode(Parts, Path, ';');
  for i:=0 to GetArrayLength(Parts)-1 do begin
    if CompareText(Parts[i], ThisAppPath) <> 0 then begin
      NewPath := NewPath + Parts[i];
      if i < GetArrayLength(Parts) - 1 then begin
        NewPath := NewPath + ';';
      end;
    end;
  end;
  RegWriteExpandStringValue({#EnvironmentRootKey}, '{#EnvironmentKey}', 'Path', NewPath);
end;