import { useState } from "react";
import { Copy, Check, Plus, Loader2, AlertCircle, Zap, Server, Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";

function apiUrl(path) {
  return `${API_BASE}${path}`;
}

export function AdminDownloads() {
  const [selectedPlatform, setSelectedPlatform] = useState(null);
  const [token, setToken] = useState(null);
  const [command, setCommand] = useState(null);
  const [copied, setCopied] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const generateToken = async (platform) => {
    setLoading(true);
    setError(null);
    try {
      // Retrieve token from session storage
      const sessionKey = "gomeshcentral-session-v1";
      const sessionData = JSON.parse(localStorage.getItem(sessionKey) || "{}");
      const authToken = sessionData.token;
      
      if (!authToken) {
        setError("Not authenticated. Please log in.");
        setLoading(false);
        return;
      }

      const response = await fetch(apiUrl("/api/enrollment-tokens"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${authToken}`,
        },
        body: JSON.stringify({ name: `Agent-${platform}-${Date.now()}` }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || "Failed to generate enrollment token");
      }

      const data = await response.json();
      const enrollmentToken = data.token || data.enrollment_token;
      
      if (!enrollmentToken) {
        throw new Error("No token returned from server");
      }

      setToken(enrollmentToken);

      // Generate the installation command
      const serverAddr = window.location.hostname === "localhost" ? "localhost:8080" : window.location.host;

      if (platform === "windows") {
        const cmd = `powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri 'http://${serverAddr}/api/download/install.ps1' -OutFile 'install.ps1' -UseBasicParsing; & .\\install.ps1 -Server '${serverAddr}' -EnrollToken '${enrollmentToken}'"`;
        setCommand(cmd);
      } else {
        const cmd = `curl -sSL http://${serverAddr}/api/download/install.sh | sudo bash -s -- -server ${serverAddr} -enroll-token ${enrollmentToken}`;
        setCommand(cmd);
      }

      setSelectedPlatform(platform);
    } catch (err) {
      console.error("Error:", err);
      setError(err.message || "Failed to create agent");
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text, id) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text).then(() => {
        setCopied(id);
        setTimeout(() => setCopied(null), 2000);
      }).catch(() => {
        fallbackCopyToClipboard(text, id);
      });
    } else {
      fallbackCopyToClipboard(text, id);
    }
  };

  const fallbackCopyToClipboard = (text, id) => {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    try {
      document.execCommand("copy");
      setCopied(id);
      setTimeout(() => setCopied(null), 2000);
    } catch (err) {
      console.error("Failed to copy:", err);
    }
    document.body.removeChild(textarea);
  };

  const downloadAgent = async () => {
    try {
      setLoading(true);
      setError(null);
      
      let filename, url;
      
      if (selectedPlatform === "windows") {
        // Download MSI installer
        url = apiUrl("/api/download/agent/windows-msi");
        filename = "GoMeshCentralAgent.msi";
        const response = await fetch(url);
        
        if (!response.ok) {
          throw new Error(`Download failed: ${response.statusText}`);
        }
        
        // Download the file
        const blob = await response.blob();
        const downloadUrl = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = downloadUrl;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(downloadUrl);
      } else {
        // Linux - just download directly
        url = apiUrl("/api/download/agent/linux-amd64");
        filename = "gomesh-agent";
        
        const response = await fetch(url);
        if (!response.ok) {
          throw new Error(`Download failed: ${response.statusText}`);
        }
        
        const blob = await response.blob();
        const downloadUrl = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = downloadUrl;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(downloadUrl);
      }
    } catch (err) {
      console.error("Download error:", err);
      setError(err.message || "Failed to download agent binary");
    } finally {
      setLoading(false);
    }
  };

  if (!selectedPlatform) {
    return (
      <div className="space-y-8 pb-8">
        <div>
          <h1 className="text-4xl font-bold tracking-tight">Deploy Agent</h1>
          <p className="text-lg text-muted-foreground mt-3">
            Install an agent on your endpoints to begin monitoring and management
          </p>
        </div>

        <div className="grid gap-6 md:grid-cols-2">
          {/* Windows Card */}
          <div className="group relative">
            <Card className="flex flex-col h-full border-2 hover:border-blue-400 hover:shadow-lg transition-all duration-300 cursor-pointer overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-br from-blue-50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
              <CardHeader className="relative pb-4">
                <div className="flex items-start justify-between mb-2">
                  <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center group-hover:bg-blue-200 transition-colors">
                    <Server className="w-6 h-6 text-blue-600" />
                  </div>
                  <Badge variant="outline" className="bg-blue-50 text-blue-700 border-blue-200">
                    Windows
                  </Badge>
                </div>
                <CardTitle className="text-2xl">Windows</CardTitle>
                <CardDescription className="text-base">Deploy to Windows endpoints</CardDescription>
              </CardHeader>
              <CardContent className="flex-grow flex flex-col justify-between relative">
                <p className="text-sm text-muted-foreground mb-6">
                  Automatically enroll Windows servers and workstations with one command
                </p>
                <Button
                  size="lg"
                  onClick={() => generateToken("windows")}
                  disabled={loading}
                  className="w-full bg-blue-600 hover:bg-blue-700 text-white py-6 text-lg font-semibold"
                >
                  {loading ? (
                    <>
                      <Loader2 className="w-5 h-5 mr-2 animate-spin" />
                      Generating...
                    </>
                  ) : (
                    <>
                      <Plus className="w-5 h-5 mr-2" />
                      Create Agent
                    </>
                  )}
                </Button>
              </CardContent>
            </Card>
          </div>

          {/* Linux Card */}
          <div className="group relative">
            <Card className="flex flex-col h-full border-2 hover:border-orange-400 hover:shadow-lg transition-all duration-300 cursor-pointer overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-br from-orange-50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
              <CardHeader className="relative pb-4">
                <div className="flex items-start justify-between mb-2">
                  <div className="w-12 h-12 bg-orange-100 rounded-lg flex items-center justify-center group-hover:bg-orange-200 transition-colors">
                    <Zap className="w-6 h-6 text-orange-600" />
                  </div>
                  <Badge variant="outline" className="bg-orange-50 text-orange-700 border-orange-200">
                    Linux
                  </Badge>
                </div>
                <CardTitle className="text-2xl">Linux</CardTitle>
                <CardDescription className="text-base">Deploy to Linux endpoints</CardDescription>
              </CardHeader>
              <CardContent className="flex-grow flex flex-col justify-between relative">
                <p className="text-sm text-muted-foreground mb-6">
                  Deploy to Ubuntu, CentOS, Debian, and other Linux distributions
                </p>
                <Button
                  size="lg"
                  onClick={() => generateToken("linux")}
                  disabled={loading}
                  className="w-full bg-orange-600 hover:bg-orange-700 text-white py-6 text-lg font-semibold"
                >
                  {loading ? (
                    <>
                      <Loader2 className="w-5 h-5 mr-2 animate-spin" />
                      Generating...
                    </>
                  ) : (
                    <>
                      <Plus className="w-5 h-5 mr-2" />
                      Create Agent
                    </>
                  )}
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8 pb-8">
      <div className="flex items-center gap-4">
        <Button 
          variant="ghost" 
          onClick={() => setSelectedPlatform(null)}
          className="text-muted-foreground hover:text-foreground"
        >
          ← Back
        </Button>
        <div>
          <h1 className="text-4xl font-bold tracking-tight">
            {selectedPlatform === "windows" ? "Windows" : "Linux"} Agent
          </h1>
          <p className="text-lg text-muted-foreground mt-2">
            Copy the command below and run it on your {selectedPlatform === "windows" ? "Windows" : "Linux"} system
          </p>
        </div>
      </div>

      {error && (
        <Card className="border-red-200 bg-red-50 border-l-4 border-l-red-500">
          <CardContent className="pt-6 flex gap-3">
            <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
            <div>
              <p className="font-semibold text-red-900">Error</p>
              <p className="text-sm text-red-800">{error}</p>
            </div>
          </CardContent>
        </Card>
      )}

      <Card className="border-2 border-gray-200">
        <CardHeader className="bg-gradient-to-r from-gray-50 to-gray-100 pb-4">
          <CardTitle className="text-2xl">
            {selectedPlatform === "windows" ? "Installation Options" : "Installation Command"}
          </CardTitle>
          <CardDescription className="text-base">
            {selectedPlatform === "windows" 
              ? "Choose your preferred installation method" 
              : "This command downloads and installs the agent with automatic enrollment"}
          </CardDescription>
        </CardHeader>
        <CardContent className="pt-8 space-y-6">
          {selectedPlatform === "windows" && (
            <div className="space-y-3">
              <p className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Option 1: MSI Installer (Recommended)
              </p>
              <p className="text-sm text-muted-foreground mb-4">
                Download the MSI installer for professional deployment. It handles installation, service setup, and automatic enrollment all in one package.
              </p>
              <div className="flex gap-3">
                <Button
                  size="lg"
                  onClick={downloadAgent}
                  disabled={loading}
                  className="flex-1 bg-green-600 hover:bg-green-700 text-white py-6 text-base font-semibold"
                >
                  {loading ? (
                    <>
                      <Loader2 className="w-5 h-5 mr-2 animate-spin" />
                      Downloading...
                    </>
                  ) : (
                    <>
                      <Download className="w-5 h-5 mr-2" />
                      Download GoMeshCentralAgent.msi
                    </>
                  )}
                </Button>
                <input
                  type="hidden"
                  id="enrollToken"
                  value={token}
                />
              </div>
              <p className="text-xs text-muted-foreground mt-3 p-3 bg-blue-50 rounded border border-blue-200">
                <strong>Enrollment Token:</strong> {token}
              </p>
              <p className="text-xs text-muted-foreground">
                After downloading, double-click <code className="bg-gray-100 px-2 py-1 rounded">GoMeshCentralAgent.msi</code> to install and automatically enroll the agent.
              </p>
            </div>
          )}

          <div className={selectedPlatform === "windows" ? "pt-6 border-t" : ""}>
            <p className="text-sm font-semibold uppercase tracking-wide text-muted-foreground mb-3">
              {selectedPlatform === "windows" ? "Option 2: PowerShell Command" : "Terminal / Bash"}
            </p>
            <div className="space-y-3">
              <div className="flex gap-2 items-start">
                <div className="flex-1 bg-gray-900 text-gray-100 p-4 rounded-lg font-mono text-sm overflow-x-auto whitespace-pre-wrap break-words leading-relaxed">
                  {command}
                </div>
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => copyToClipboard(command, "command")}
                  title="Copy command"
                  className="flex-shrink-0 mt-1 h-12 w-12 p-0"
                >
                  {copied === "command" ? (
                    <Check className="w-5 h-5 text-green-600" />
                  ) : (
                    <Copy className="w-5 h-5" />
                  )}
                </Button>
              </div>
              {copied === "command" && (
                <p className="text-sm text-green-600 font-medium">✓ Command copied to clipboard</p>
              )}
            </div>
          </div>

          {token && (
            <div className="space-y-3 pt-4 border-t">
              <p className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Enrollment Token
              </p>
              <div className="flex gap-2 items-start">
                <div className="flex-1 bg-gray-100 p-4 rounded-lg font-mono text-sm overflow-x-auto break-all border border-gray-200">
                  {token}
                </div>
                <Button
                  variant="outline"
                  size="lg"
                  onClick={() => copyToClipboard(token, "token")}
                  title="Copy token"
                  className="flex-shrink-0 mt-1 h-12 w-12 p-0"
                >
                  {copied === "token" ? (
                    <Check className="w-5 h-5 text-green-600" />
                  ) : (
                    <Copy className="w-5 h-5" />
                  )}
                </Button>
              </div>
              {copied === "token" && (
                <p className="text-sm text-green-600 font-medium">✓ Token copied to clipboard</p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="border-l-4 border-l-blue-500 bg-blue-50">
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Zap className="w-5 h-5 text-blue-600" />
            What happens next?
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 text-sm">
          <p className="text-gray-700">
            After {selectedPlatform === "windows" 
              ? "running either installation method" 
              : "running this command"} on your {selectedPlatform === "windows" ? "Windows" : "Linux"} system:
          </p>
          <ol className="space-y-3 ml-2">
            <li className="flex gap-3">
              <span className="font-bold text-blue-600 flex-shrink-0">1</span>
              <span>{selectedPlatform === "windows" 
                ? "The agent binary starts and connects to the server" 
                : "The installer script downloads the agent binary"}</span>
            </li>
            <li className="flex gap-3">
              <span className="font-bold text-blue-600 flex-shrink-0">2</span>
              <span>The enrollment token automatically authenticates with the server</span>
            </li>
            <li className="flex gap-3">
              <span className="font-bold text-blue-600 flex-shrink-0">3</span>
              <span>The agent connects to GoMeshCentral and begins system monitoring</span>
            </li>
            <li className="flex gap-3">
              <span className="font-bold text-blue-600 flex-shrink-0">4</span>
              <span>The system appears in your Devices list within seconds</span>
            </li>
          </ol>
        </CardContent>
      </Card>
    </div>
  );
}
