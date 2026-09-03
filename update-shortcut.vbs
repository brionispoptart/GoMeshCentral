Set shell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

linkPath = shell.SpecialFolders("AllUsersStartup") & "\GoMeshCentral Agent Tray.lnk"
exePath = "C:\Program Files\GoMeshCentral\agent.exe"
iconPath = "C:\Program Files\GoMeshCentral\agent.ico"

' Delete existing shortcut if it exists
If fso.FileExists(linkPath) Then
    fso.DeleteFile linkPath
End If

' Create new shortcut
Set shortcut = shell.CreateShortcut(linkPath)
shortcut.TargetPath = exePath
shortcut.Arguments = "-tray-ui-only -tray-icon """ & iconPath & """"
shortcut.IconLocation = exePath & ",0"
shortcut.WorkingDirectory = "C:\Program Files\GoMeshCentral"
shortcut.WindowStyle = 7
shortcut.Save()

WScript.Echo "Shortcut created at " & linkPath
