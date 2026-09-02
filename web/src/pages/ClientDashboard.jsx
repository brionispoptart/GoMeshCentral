import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { AlertCircle, CheckCircle2, Clock, AlertTriangle, Activity } from "lucide-react";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";

export function ClientDashboard({ token }) {
  const [stats, setStats] = useState({
    devices: { online: 0, offline: 0, total: 0 },
    tickets: { open: 0, pending: 0, closed: 0, total: 0 },
    alerts: { critical: 0, warning: 0, total: 0 },
    invoices: { unpaid: 0, paid: 0, total: 0 },
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadClientStats();
  }, [token]);

  const loadClientStats = async () => {
    try {
      // In a real implementation, you'd have a /api/client/stats endpoint
      // For now, we'll show the structure
      setStats({
        devices: { online: 12, offline: 3, total: 15 },
        tickets: { open: 5, pending: 2, closed: 45, total: 52 },
        alerts: { critical: 1, warning: 3, total: 4 },
        invoices: { unpaid: 2, paid: 18, total: 20 },
      });
    } catch (error) {
      console.error("Failed to load client stats:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-8">
      {/* Page Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-600 mt-2 text-sm">Welcome back! Here's an overview of your account.</p>
      </div>

      {/* Quick Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Devices Card */}
        <Card className="border border-gray-200 shadow-sm hover:shadow-md transition-shadow">
          <CardHeader className="pb-2.5">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-gray-600">Devices</CardTitle>
              <Activity className="w-4 h-4 text-gray-400" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-gray-900">{stats.devices.total}</div>
            <p className="text-xs text-gray-500 mt-2">
              <span className="text-green-600 font-medium">{stats.devices.online}</span> online •{" "}
              <span className="text-red-600">{stats.devices.offline}</span> offline
            </p>
          </CardContent>
        </Card>

        {/* Tickets Card */}
        <Card className="border border-gray-200 shadow-sm hover:shadow-md transition-shadow">
          <CardHeader className="pb-2.5">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-gray-600">Support Tickets</CardTitle>
              <Clock className="w-4 h-4 text-gray-400" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-gray-900">{stats.tickets.open}</div>
            <p className="text-xs text-gray-500 mt-2">
              <span className="text-blue-600 font-medium">{stats.tickets.open}</span> open •{" "}
              <span className="text-orange-600">{stats.tickets.pending}</span> pending
            </p>
          </CardContent>
        </Card>

        {/* Alerts Card */}
        <Card className="border border-gray-200 shadow-sm hover:shadow-md transition-shadow">
          <CardHeader className="pb-2.5">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-gray-600">Alerts</CardTitle>
              <AlertTriangle className="w-4 h-4 text-gray-400" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-gray-900">{stats.alerts.total}</div>
            <p className="text-xs text-gray-500 mt-2">
              {stats.alerts.critical > 0 && (
                <>
                  <span className="text-red-600 font-medium">{stats.alerts.critical}</span> critical •{" "}
                </>
              )}
              <span className="text-orange-600">{stats.alerts.warning}</span> warning
            </p>
          </CardContent>
        </Card>

        {/* Invoices Card */}
        <Card className="border border-gray-200 shadow-sm hover:shadow-md transition-shadow">
          <CardHeader className="pb-2.5">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium text-gray-600">Invoices</CardTitle>
              <AlertCircle className="w-4 h-4 text-gray-400" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold text-gray-900">{stats.invoices.unpaid}</div>
            <p className="text-xs text-gray-500 mt-2">
              <span className={stats.invoices.unpaid > 0 ? "text-orange-600 font-medium" : "text-green-600 font-medium"}>
                {stats.invoices.unpaid > 0 ? `${stats.invoices.unpaid} unpaid` : "All paid"}
              </span>
              {stats.invoices.unpaid > 0 && ` • ${stats.invoices.paid} paid`}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Detailed Status Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* System Health */}
        <Card className="border border-gray-200 shadow-sm">
          <CardHeader>
            <CardTitle className="text-base font-semibold text-gray-900">System Health</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {stats.devices.online > 0 && (
              <div className="flex items-center justify-between p-3 bg-green-50 border border-green-100 rounded-lg">
                <div className="flex items-center gap-3">
                  <CheckCircle2 className="w-5 h-5 text-green-600" />
                  <span className="text-sm font-medium text-green-900">Devices operational</span>
                </div>
                <span className="text-lg font-bold text-green-600">{stats.devices.online}</span>
              </div>
            )}
            {stats.devices.offline > 0 && (
              <div className="flex items-center justify-between p-3 bg-red-50 border border-red-100 rounded-lg">
                <div className="flex items-center gap-3">
                  <AlertCircle className="w-5 h-5 text-red-600" />
                  <span className="text-sm font-medium text-red-900">Offline devices</span>
                </div>
                <span className="text-lg font-bold text-red-600">{stats.devices.offline}</span>
              </div>
            )}
            {stats.alerts.critical === 0 && (
              <div className="flex items-center justify-between p-3 bg-blue-50 border border-blue-100 rounded-lg">
                <div className="flex items-center gap-3">
                  <CheckCircle2 className="w-5 h-5 text-blue-600" />
                  <span className="text-sm font-medium text-blue-900">No critical alerts</span>
                </div>
                <span className="text-lg font-bold text-blue-600">0</span>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Support Status */}
        <Card className="border border-gray-200 shadow-sm">
          <CardHeader>
            <CardTitle className="text-base font-semibold text-gray-900">Support Status</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {stats.tickets.open > 0 && (
              <div className="flex items-center justify-between p-3 bg-blue-50 border border-blue-100 rounded-lg">
                <div className="flex items-center gap-3">
                  <Clock className="w-5 h-5 text-blue-600" />
                  <span className="text-sm font-medium text-blue-900">Open tickets</span>
                </div>
                <span className="text-lg font-bold text-blue-600">{stats.tickets.open}</span>
              </div>
            )}
            {stats.tickets.pending > 0 && (
              <div className="flex items-center justify-between p-3 bg-orange-50 border border-orange-100 rounded-lg">
                <div className="flex items-center gap-3">
                  <AlertTriangle className="w-5 h-5 text-orange-600" />
                  <span className="text-sm font-medium text-orange-900">Pending approval</span>
                </div>
                <span className="text-lg font-bold text-orange-600">{stats.tickets.pending}</span>
              </div>
            )}
            {stats.tickets.open === 0 && stats.tickets.pending === 0 && (
              <div className="flex items-center justify-between p-3 bg-green-50 border border-green-100 rounded-lg">
                <div className="flex items-center gap-3">
                  <CheckCircle2 className="w-5 h-5 text-green-600" />
                  <span className="text-sm font-medium text-green-900">All tickets resolved</span>
                </div>
                <span className="text-lg font-bold text-green-600">0</span>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
