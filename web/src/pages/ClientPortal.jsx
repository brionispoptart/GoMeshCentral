import { useEffect, useState } from "react";
import { useNavigate, Routes, Route } from "react-router-dom";
import { Card, CardContent } from "@/components/ui/card";
import { AlertCircle } from "lucide-react";
import { ClientPortalLayout } from "@/components/ClientPortalLayout";
import { ClientDashboard } from "./ClientDashboard";
import { ClientDevices } from "./ClientDevices";
import { ClientTickets } from "./ClientTickets";
import { ClientInvoices } from "./ClientInvoices";
import { ClientSettings } from "./ClientSettings";
import { POCPortal } from "./POCPortal";
import { Button } from "@/components/ui/button";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";

// Wrapper component for portal routes
function ClientPortalWrapper({ children, client, token, onLogout }) {
  return (
    <ClientPortalLayout
      clientName={client?.name}
      role={client?.role}
      onLogout={onLogout}
    >
      {children}
    </ClientPortalLayout>
  );
}

export function ClientPortal() {
  const navigate = useNavigate();
  const [client, setClient] = useState(null);
  const [role, setRole] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const token = localStorage.getItem("clientToken");

  useEffect(() => {
    if (!token) {
      navigate("/client");
      return;
    }

    fetchClientData();
  }, [token, navigate]);

  const fetchClientData = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/client/me`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!res.ok) {
        if (res.status === 401) {
          localStorage.removeItem("clientToken");
          localStorage.removeItem("clientId");
          navigate("/client");
          return;
        }
        throw new Error("Failed to load client data");
      }

      const data = await res.json();
      setClient(data);
      setRole(data.role);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem("clientToken");
    localStorage.removeItem("clientId");
    navigate("/client");
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-teal-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading your account...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-50 p-4 flex items-center justify-center">
        <Card className="max-w-md">
          <CardContent className="pt-6">
            <div className="flex items-center gap-2 text-red-600 mb-4">
              <AlertCircle className="w-5 h-5" />
              <span>Error: {error}</span>
            </div>
            <Button onClick={handleLogout} className="w-full">
              Return to Login
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Show POC portal for POCs
  if (role === "poc") {
    return (
      <div className="min-h-screen bg-gray-50">
        {/* Header */}
        <div className="bg-white border-b border-gray-200 sticky top-0 z-40">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">Approval Center</h1>
              <p className="text-sm text-gray-600">Review and approve client tickets</p>
            </div>
            <Button variant="outline" size="sm" onClick={handleLogout}>
              Sign Out
            </Button>
          </div>
        </div>

        {/* Main Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <POCPortal />
        </div>
      </div>
    );
  }

  // Regular client portal with sidebar layout
  return (
    <Routes>
      <Route
        path="/"
        element={
          <ClientPortalWrapper client={client} token={token} onLogout={handleLogout}>
            <ClientDashboard token={token} />
          </ClientPortalWrapper>
        }
      />
      <Route
        path="/devices"
        element={
          <ClientPortalWrapper client={client} token={token} onLogout={handleLogout}>
            <ClientDevices token={token} />
          </ClientPortalWrapper>
        }
      />
      <Route
        path="/tickets"
        element={
          <ClientPortalWrapper client={client} token={token} onLogout={handleLogout}>
            <ClientTickets token={token} />
          </ClientPortalWrapper>
        }
      />
      <Route
        path="/invoices"
        element={
          <ClientPortalWrapper client={client} token={token} onLogout={handleLogout}>
            <ClientInvoices token={token} />
          </ClientPortalWrapper>
        }
      />
      <Route
        path="/settings"
        element={
          <ClientPortalWrapper client={client} token={token} onLogout={handleLogout}>
            <ClientSettings token={token} clientName={client?.name} onLogout={handleLogout} />
          </ClientPortalWrapper>
        }
      />
    </Routes>
  );
}
