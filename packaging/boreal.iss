#ifndef Version
  #define Version "0.1.0-dev"
#endif

#ifndef Arch
  #define Arch "amd64"
#endif

#ifndef SourceDir
  #define SourceDir "..\dist\boreal-" + Arch
#endif

#if Arch == "arm64"
  #define Machines "arm64"
  #define SetupName "boreal-" + Version + "-arm64-setup"
#else
  #define Machines "x64compatible"
  #define SetupName "boreal-" + Version + "-setup"
#endif

[Setup]
AppId={{8E6C3F2A-9C3B-4C55-9B7E-2D1F0B6A1C11}
AppName=Boreal
AppVersion={#Version}
AppPublisher=Northlight
AppPublisherURL=https://github.com/northlighthq
AppSupportURL=https://github.com/northlighthq/boreal/issues
AppUpdatesURL=https://github.com/northlighthq/boreal/releases
DefaultDirName={localappdata}\Programs\Boreal
DisableProgramGroupPage=yes
DisableDirPage=yes
PrivilegesRequired=lowest
OutputBaseFilename={#SetupName}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=boreal.ico
UninstallDisplayIcon={app}\boreal.exe
UninstallDisplayName=Boreal
ArchitecturesAllowed={#Machines}
ArchitecturesInstallIn64BitMode={#Machines}
MinVersion=10.0.17763
CloseApplications=force
RestartApplications=no

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\boreal.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion

[InstallDelete]
Type: files; Name: "{app}platty.exe"
Type: files; Name: "{autoprograms}Platty.lnk"
Type: files; Name: "{autodesktop}Platty.lnk"

[Icons]
Name: "{autoprograms}\Boreal"; Filename: "{app}\boreal.exe"
Name: "{autodesktop}\Boreal"; Filename: "{app}\boreal.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\boreal.exe"; Description: "{cm:LaunchProgram,Boreal}"; Flags: nowait postinstall skipifsilent
Filename: "{app}\boreal.exe"; Flags: nowait; Check: IsUpdate

[Code]
function IsUpdate: Boolean;
begin
  Result := Pos('/UPDATE', UpperCase(GetCmdTail)) > 0;
end;
