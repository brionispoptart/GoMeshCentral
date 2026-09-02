import { useState, useEffect } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AlertCircle, CheckCircle, XCircle } from "lucide-react";

export function PendingApprovals() {
  const [tickets, setTickets] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [processing, setProcessing] = useState({});

  useEffect(() => {
    fetchPendingApprovals();
    // Refresh every 30 seconds
    const interval = setInterval(fetchPendingApprovals, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchPendingApprovals = async () => {
    try {
      const raw = localStorage.getItem("gomeshcentral-session-v1");
      if (!raw) {
        setError("Not authenticated");
        setLoading(false);
        return;
      }
      const { token } = JSON.parse(raw);
      const res = await fetch("/api/admin/pending-approvals", {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (res.status === 401) {
        setError("Session expired. Please log in again.");
        return;
      }

      if (!res.ok) throw new Error("Failed to fetch pending approvals");

      const data = await res.json();
      setTickets(data || []);
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
      const raw = localStorage.getItem("gomeshcentral-session-v1");
      const { token } = JSON.parse(raw);
      const res = await fetch(`/api/tickets/${ticketId}`, {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ status: "open" }),
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
      const raw = localStorage.getItem("gomeshcentral-session-v1");
      const { token } = JSON.parse(raw);
      const res = await fetch(`/api/tickets/${ticketId}`, {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ status: "rejected" }),
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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">Ticket Approvals</h2>
          <p className="text-gray-600">
            Review and approve pending client support tickets
          </p>
        </div>
        {tickets.length > 0 && (
          <Badge className="bg-yellow-100 text-yellow-700 text-lg px-3 py-1">
            {tickets.length} pending
          </Badge>
        )}
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex gap-3">
          <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
          <div className="text-red-700">{error}</div>
        </div>
      )}

      {tickets.length === 0 ? (
        <Card className="p-8 text-center">
          <CheckCircle className="w-12 h-12 text-green-500 mx-auto mb-3" />
          <p className="text-gray-600">
            All caught up! No tickets pending approval.
          </p>
        </Card>
      ) : (
        <div className="space-y-3">
          {tickets.map((ticket) => (
            <Card key={ticket.id} className="p-4">
              <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <div className="md:col-span-2">
                  <h3 className="font-semibold text-gray-900">
                    {ticket.subject}
                  </h3>
                  <p className="text-sm text-gray-600 line-clamp-2">
                    {ticket.description}
                  </p>
                  <div className="mt-2 text-xs text-gray-500">
                    Submitted by: <span className="font-medium">{ticket.createdBy}</span>
                  </div>
                </div>

                <div>
                  <Badge className={getPriorityColor(ticket.priority)}>
                    {ticket.priority || "medium"}
                  </Badge>
                  <div className="text-xs text-gray-600 mt-2">
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
  );
}
