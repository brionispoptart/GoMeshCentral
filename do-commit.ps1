cd "c:\Users\Brion Lund\Documents\GoMeshCentral"
git add .
git commit -m "Remove MSI installer, switch to PowerShell-only installation, add agent downloader to Devices tab"
git log --oneline -n 5 > last-commits.txt
"Commit completed - see last-commits.txt for details"
