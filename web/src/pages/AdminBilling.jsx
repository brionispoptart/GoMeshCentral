import { useEffect, useState } from "react";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AlertCircle, Download, FileText, Loader2, Mail } from "lucide-react";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";

const statusColors = {
  draft: "bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400",
  sent: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400",
  paid: "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400",
  overdue: "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400",
};

export function AdminBilling({ token }) {
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [generatingPDF, setGeneratingPDF] = useState({});
  const [sendingEmail, setSendingEmail] = useState({});

  useEffect(() => {
    fetchInvoices();
  }, [token]);

  const fetchInvoices = async () => {
    try {
      setLoading(true);
      const res = await fetch(`${API_BASE}/api/invoices`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!res.ok) throw new Error("Failed to load invoices");

      const data = await res.json();
      setInvoices(data || []);
      setError("");
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const generateAndDownloadPDF = async (invoiceId, invoiceNumber) => {
    try {
      setGeneratingPDF((prev) => ({ ...prev, [invoiceId]: true }));

      // First, generate the PDF
      const generateRes = await fetch(`${API_BASE}/api/invoices/${invoiceId}/generate-pdf`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!generateRes.ok) {
        throw new Error(`Failed to generate PDF: ${generateRes.statusText}`);
      }

      const result = await generateRes.json();

      // Then download it
      const downloadRes = await fetch(`${API_BASE}/api/invoices/${invoiceId}/download-pdf`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!downloadRes.ok) {
        throw new Error(`Failed to download PDF: ${downloadRes.statusText}`);
      }

      const blob = await downloadRes.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = result.filename || `invoice-${invoiceNumber}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(`Error downloading PDF: ${err.message}`);
    } finally {
      setGeneratingPDF((prev) => ({ ...prev, [invoiceId]: false }));
    }
  };

  const sendInvoiceEmail = async (invoiceId, invoiceNumber) => {
    try {
      setSendingEmail((prev) => ({ ...prev, [invoiceId]: true }));

      const res = await fetch(`${API_BASE}/api/invoices/${invoiceId}/send-email`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      });

      if (!res.ok) {
        throw new Error(`Failed to send email: ${res.statusText}`);
      }

      const result = await res.json();
      setError("");
      // Show success message briefly
      alert(result.message || "Invoice emailed successfully!");
    } catch (err) {
      setError(`Error sending email: ${err.message}`);
    } finally {
      setSendingEmail((prev) => ({ ...prev, [invoiceId]: false }));
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Billing</h1>
          <p className="text-gray-600 dark:text-gray-400">Create invoices and collect payment from customers.</p>
        </div>
        <div className="text-center py-12 text-gray-600 dark:text-gray-400">Loading invoices...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Billing</h1>
        <p className="text-gray-600 dark:text-gray-400">Create invoices and collect payment from customers.</p>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-4 bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 rounded-lg">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
        {invoices.length === 0 ? (
          <div className="text-center py-12 text-gray-600 dark:text-gray-400">
            <FileText className="w-12 h-12 mx-auto mb-3 opacity-40" />
            <p>No invoices created yet.</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="bg-gray-50 dark:bg-slate-800">
                <TableHead>Invoice #</TableHead>
                <TableHead>Client</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Issue Date</TableHead>
                <TableHead>Due Date</TableHead>
                <TableHead className="text-right">Amount</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {invoices.map((invoice) => (
                <TableRow key={invoice.id} className="hover:bg-gray-50 dark:hover:bg-slate-800/50">
                  <TableCell className="font-medium">{invoice.invoiceNumber}</TableCell>
                  <TableCell className="text-sm text-gray-600 dark:text-gray-400">{invoice.clientId}</TableCell>
                  <TableCell>
                    <Badge className={statusColors[invoice.status] || statusColors.draft}>
                      {invoice.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{new Date(invoice.issueDate).toLocaleDateString()}</TableCell>
                  <TableCell>{new Date(invoice.dueDate).toLocaleDateString()}</TableCell>
                  <TableCell className="text-right font-semibold">${invoice.total.toFixed(2)}</TableCell>
                  <TableCell className="text-right space-x-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => generateAndDownloadPDF(invoice.id, invoice.invoiceNumber)}
                      disabled={generatingPDF[invoice.id]}
                      className="gap-2"
                    >
                      {generatingPDF[invoice.id] ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          Generating...
                        </>
                      ) : (
                        <>
                          <Download className="w-4 h-4" />
                          PDF
                        </>
                      )}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => sendInvoiceEmail(invoice.id, invoice.invoiceNumber)}
                      disabled={sendingEmail[invoice.id]}
                      className="gap-2"
                    >
                      {sendingEmail[invoice.id] ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          Sending...
                        </>
                      ) : (
                        <>
                          <Mail className="w-4 h-4" />
                          Email
                        </>
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>
  );
}
