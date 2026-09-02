import { useState, useEffect } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AlertCircle, CheckCircle, XCircle } from "lucide-react";

export function POCPortal() {
  const [tickets, setTickets] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [processing, setProcessing] = useState({});
  const [pocName, setPOCName] = useState("");

  useEffect(() => {
    fetchPOCData();
  }, []);

  const fetchPOCData = async () => {
    try {
      // Get POC info
      const raw = localStorage.getItem("clientToken");
      const res = await fetch("/api/client/me", {
        headers: { Authorization: `Bearer ${raw}` },
      });

      if (res.status === 401) {
        localStorage.removeItem("clientToken");
        window.location.href = "/client";
        return;
      }

      if (res.ok) {
        const data = await res.json();
        setPOCName(data.name || "Point of Contact");
      }

      // Get pending approvals
      const ticketsRes = await fetch("/api/client/tickets", {
        headers: { Authorization: `Bearer ${raw}` },
      });

      if (!ticketsRes.ok) throw new Error("Failed to fetch pending tickets");

      const ticketsData = await ticketsRes.json();
      setTickets(ticketsData || []);
      setError("");
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleApprove = async (ticketId) => {
    setProcessing((prev) => ({ ...prev, [ticketId]: "approving" }));
    try {
      const token = localStorage.getItem("clientToken");
      const res = await fetch(`/api/client/tickets/${ticketId}/approve`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
      });

      if (!res.ok) throw new Error("Failed to approve ticket");

      // Remove from pending list
      setTickets((prev) => prev.filter((t) => t.id !== ticketId));
    } catch (err) {
      setError(err.message);
    } finally {
      setProcessing((prev) => ({ ...prev, [ticketId]: null }));
    }
  };

  const handleReject = async (ticketId) => {
    setProcessing((prev) => ({ ...prev, [ticketId]: "rejecting" }));
    try {
      const token = localStorage.getItem("clientToken");
      const res = await fetch(`/api/client/tickets/${ticketId}/reject`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
      });

      if (!res.ok) throw new Error("Failed to reject ticket");

      // Remove from pending list
      setTickets((prev) => prev.filter((t) => t.id !== ticketId));
    } catch (err) {
      setError(err.message);
    } finally {
      setProcessing((prev) => ({ ...prev, [ticketId]: null }));
    }
  };

  const getPriorityColor = (priority) => {
    const colors = {
      low: "bg-gray-100 text-gray-700",
      medium: "bg-blue-100 text-blue-700",
      high: "bg-orange-100 text-orange-700",
      critical: "bg-red-100 text-red-700",
    };
    return colors[priority] || colors.medium;
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-slate-800 dark:to-slate-900 rounded-lg p-6">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
          Welcome, {pocName}
        </h1>
        <p className="text-gray-600 dark:text-gray-400 mt-2">
          Manage and approve client support requests
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="p-4">
          <div className="text-sm text-gray-600 dark:text-gray-400">
            Pending Approvals
          </div>
          <div className="text-3xl font-bold text-yellow-600 mt-2">
            {tickets.filter((t) => t.status === "pending_approval").length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600 dark:text-gray-400">
            Approved Today
          </div>
          <div className="text-3xl font-bold text-green-600 mt-2">
            {tickets.filter((t) => t.status === "open" && t.approvedAt).length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600 dark:text-gray-400">
            Rejected
          </div>
          <div className="text-3xl font-bold text-red-600 mt-2">
            {tickets.filter((t) => t.status === "rejected").length}
          </div>
        </Card>
      </div>

      {/* Error Alert */}
      {error && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 flex gap-3">
          <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
          <div className="text-red-700 dark:text-red-400">{error}</div>
        </div>
      )}

      {/* Approval Queue */}
      <div>
        <h2 className="text-2xl font-bold mb-4">Approval Queue</h2>

        {tickets.filter((t) => t.status === "pending_approval").length === 0 ? (
          <Card className="p-8 text-center">
            <CheckCircle className="w-12 h-12 text-green-500 mx-auto mb-3" />
            <p className="text-gray-600 dark:text-gray-400">
              All caught up! No tickets pending approval.
            </p>
          </Card>
        ) : (
          <div className="space-y-3">
            {tickets
              .filter((t) => t.status === "pending_approval")
              .map((ticket) => (
                <Card key={ticket.id} className="p-4">
                  <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                    <div className="md:col-span-2">
                      <h3 className="font-semibold text-gray-900 dark:text-white">
                        {ticket.subject}
                      </h3>
                      <p className="text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                        {ticket.description}
                      </p>
                      <div className="mt-2 text-xs text-gray-500">
                        From: <span className="font-medium">{ticket.clientId}</span>
                      </div>
                    </div>

                    <div>
                      <Badge className={getPriorityColor(ticket.priority)}>
                        {ticket.priority || "medium"}
                      </Badge>
                      <div className="text-xs text-gray-600 dark:text-gray-400 mt-2">
                        {new Date(ticket.createdAt).toLocaleDateString()}
                      </div>
                    </div>

                    <div className="flex gap-2 items-center">
                      <Button
                        onClick={() => handleApprove(ticket.id)}
                        disabled={processing[ticket.id] === "approving"}
                        className="bg-green-600 hover:bg-green-700 text-white text-sm flex-1"
                      >
                        {processing[ticket.id] === "approving" ? (
                          <span className="flex items-center gap-2">
                            <span className="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin"></span>
                            Approving
                          </span>
                        ) : (
                          <span className="flex items-center gap-1">
                            <CheckCircle className="w-4 h-4" />
                            Approve
                          </span>
                        )}
                      </Button>
                      <Button
                        onClick={() => handleReject(ticket.id)}
                        disabled={processing[ticket.id] === "rejecting"}
                        className="bg-red-100 hover:bg-red-200 text-red-700 text-sm flex-1"
                      >
                        {processing[ticket.id] === "rejecting" ? (
                          <span className="flex items-center gap-2">
                            <span className="w-3 h-3 border-2 border-red-700 border-t-transparent rounded-full animate-spin"></span>
                            Rejecting
                          </span>
                        ) : (
                          <span className="flex items-center gap-1">
                            <XCircle className="w-4 h-4" />
                            Reject
                          </span>
                        )}
                      </Button>
                    </div>
                  </div>
                </Card>
              ))}
          </div>
        )}
      </div>
    </div>
  );
}
