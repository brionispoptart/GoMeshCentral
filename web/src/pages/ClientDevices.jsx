import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Search, RefreshCw, Monitor } from "lucide-react";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";

export function ClientDevices({ token }) {
  const [devices, setDevices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");

  useEffect(() => {
    loadDevices();
  }, [token]);

  const loadDevices = async () => {
    try {
      setLoading(true);
      const res = await fetch(`${API_BASE}/api/client/devices`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (res.ok) {
        const data = await res.json();
        setDevices(Array.isArray(data) ? data : []);
      }
    } catch (error) {
      console.error("Failed to load devices:", error);
    } finally {
      setLoading(false);
    }
  };

  const filteredDevices = devices.filter((d) =>
    d.name?.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getStatusBadge = (status) => {
    if (status === "online") {
      return <Badge className="bg-green-100 text-green-800 border border-green-300">Online</Badge>;
    }
    return <Badge className="bg-red-100 text-red-800 border border-red-300">Offline</Badge>;
  };

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">My Devices</h1>
          <p className="text-gray-600 mt-2 text-sm">Monitor and manage your connected devices.</p>
        </div>
        <Button
          onClick={loadDevices}
          variant="outline"
          size="sm"
          className="border-gray-300 hover:bg-gray-50"
        >
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Search */}
      <div className="max-w-md">
        <div className="relative">
          <Search className="absolute left-3 top-2.5 w-4 h-4 text-gray-400" />
          <Input
            placeholder="Search devices..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-10 border-gray-300"
          />
        </div>
      </div>

      {/* Devices Table */}
      {loading ? (
        <Card className="border border-gray-200 shadow-sm">
          <CardContent className="pt-12 text-center text-gray-500">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto mb-3"></div>
            <p className="text-sm">Loading devices...</p>
          </CardContent>
        </Card>
      ) : filteredDevices.length === 0 ? (
        <Card className="border border-gray-200 shadow-sm">
          <CardContent className="pt-12 text-center text-gray-500">
            <Monitor className="w-12 h-12 mx-auto mb-3 opacity-40" />
            <p className="font-medium">No devices found</p>
            <p className="text-sm mt-1">Contact support if you expect to see devices here.</p>
          </CardContent>
        </Card>
      ) : (
        <Card className="border border-gray-200 shadow-sm">
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="border-b border-gray-200 hover:bg-transparent">
                  <TableHead className="text-gray-700 font-semibold text-xs uppercase tracking-wide">
                    Device Name
                  </TableHead>
                  <TableHead className="text-gray-700 font-semibold text-xs uppercase tracking-wide">
                    Status
                  </TableHead>
                  <TableHead className="text-gray-700 font-semibold text-xs uppercase tracking-wide">
                    Type
                  </TableHead>
                  <TableHead className="text-gray-700 font-semibold text-xs uppercase tracking-wide">
                    Last Seen
                  </TableHead>
                  <TableHead className="text-gray-700 font-semibold text-xs uppercase tracking-wide">
                    Alerts
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredDevices.map((device) => (
                  <TableRow key={device.id || device.name} className="border-b border-gray-100 hover:bg-gray-50">
                    <TableCell className="font-medium text-gray-900 py-3">{device.name}</TableCell>
                    <TableCell className="py-3">
                      <div className="flex items-center gap-2">
                        <span
                          className={`inline-block w-2 h-2 rounded-full ${
                            device.status === "online" ? "bg-green-500" : "bg-red-500"
                          }`}
                        />
                        {getStatusBadge(device.status)}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-gray-600 py-3">
                      {device.type || "Computer"}
                    </TableCell>
                    <TableCell className="text-sm text-gray-600 py-3">
                      {device.lastSeen ? new Date(device.lastSeen).toLocaleDateString() : "—"}
                    </TableCell>
                    <TableCell className="py-3">
                      {device.alertCount > 0 ? (
                        <Badge className="bg-red-100 text-red-800 border border-red-300">
                          {device.alertCount}
                        </Badge>
                      ) : (
                        <span className="text-sm text-gray-400">—</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </Card>
      )}
    </div>
  );
}
