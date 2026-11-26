; Inno Setup 6 Per-User Installer Script for sshm
; Copyright © 2026 Oktanetflow
; https://github.com/oktaaa17/sshm

#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif

#define MyAppName "sshm"
#define MyAppPublisher "Oktanetflow"
#define MyAppCopyright "Copyright © 2026 Oktanetflow"
#define MyAppURL "https://github.com/oktaaa17/sshm"
#define MyAppExeName "sshm.exe"

[Setup]
AppId={{8D4F6A2E-5A3B-4E91-9E12-7B4F2C8A1D09}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppUpdatesURL={#MyAppURL}/releases
AppCopyright={#MyAppCopyright}
VersionInfoVersion={#MyAppVersion}.0
VersionInfoCompany={#MyAppPublisher}
VersionInfoCopyright={#MyAppCopyright}
VersionInfoDescription=sshm — Minimalist CLI/TUI SSH Client for Windows
VersionInfoProductName={#MyAppName}
VersionInfoProductVersion={#MyAppVersion}
DefaultDirName={localappdata}\Programs\{#MyAppPublisher}\{#MyAppName}
DefaultGroupName={#MyAppPublisher}\{#MyAppName}
DisableProgramGroupPage=yes
; Per-user installation: zero UAC admin elevation required!
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\..\dist
OutputBaseFilename=sshm-setup-{#MyAppVersion}-amd64
SetupIconFile=..\..\cmd\sshm\assets\sshm.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName} ({#MyAppPublisher})
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
ChangesEnvironment=yes
LicenseFile=..\..\LICENSE

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "addtopath"; Description: "Add sshm to your Windows User PATH (recommended)"; GroupDescription: "Terminal integration:"
Name: "startmenu"; Description: "Create Start Menu shortcut to launch sshm TUI"; GroupDescription: "Shortcuts:"

[Files]
Source: "..\..\dist\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\..\SECURITY.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\..\docs\USAGE.md"; DestDir: "{app}\docs"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\{#MyAppPublisher}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\{#MyAppExeName}"; Tasks: startmenu
Name: "{userprograms}\{#MyAppPublisher}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"; Tasks: startmenu

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch sshm interactive TUI"; Flags: nowait postinstall skipifsilent

[Code]
const
  EnvKey = 'Environment';

function PathContainsDir(PathVal, Dir: string): Boolean;
var
  UpperPath, UpperDir: string;
begin
  UpperPath := ';' + UpperCase(PathVal) + ';';
  UpperDir := ';' + UpperCase(RemoveBackslashUnlessRoot(Dir)) + ';';
  Result := Pos(UpperDir, UpperPath) > 0;
end;

procedure AddToUserPath(Dir: string);
var
  OrigPath, NewPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', OrigPath) then
    OrigPath := '';
  if PathContainsDir(OrigPath, Dir) then
    Exit;
  if (Length(OrigPath) > 0) and (OrigPath[Length(OrigPath)] <> ';') then
    NewPath := OrigPath + ';' + Dir
  else
    NewPath := OrigPath + Dir;
  RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', NewPath);
end;

procedure RemoveFromUserPath(Dir: string);
var
  OrigPath, UpperPath, UpperDir: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', OrigPath) then
    Exit;
  UpperPath := ';' + UpperCase(OrigPath) + ';';
  UpperDir := ';' + UpperCase(RemoveBackslashUnlessRoot(Dir)) + ';';
  P := Pos(UpperDir, UpperPath);
  if P = 0 then
    Exit;
  Delete(OrigPath, P, Length(Dir) + 1);
  if (Length(OrigPath) > 0) and (OrigPath[1] = ';') then
    Delete(OrigPath, 1, 1);
  if (Length(OrigPath) > 0) and (OrigPath[Length(OrigPath)] = ';') then
    Delete(OrigPath, Length(OrigPath), 1);
  RegWriteExpandStringValue(HKEY_CURRENT_USER, EnvKey, 'Path', OrigPath);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if (CurStep = ssPostInstall) and WizardIsTaskSelected('addtopath') then
    AddToUserPath(ExpandConstant('{app}'));
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RemoveFromUserPath(ExpandConstant('{app}'));
end;
