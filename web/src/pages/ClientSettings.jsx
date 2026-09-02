import { useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AlertCircle, CheckCircle2 } from "lucide-react";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";

export function ClientSettings({ token, clientName, onLogout }) {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [formData, setFormData] = useState({
    name: clientName || "",
    email: "",
    phone: "",
    address: "",
  });

  useEffect(() => {
    loadClientInfo();
  }, [token]);

  const loadClientInfo = async () => {
    try {
      setLoading(true);
      const res = await fetch(`${API_BASE}/api/client/me`, {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (res.ok) {
        const data = await res.json();
        setFormData({
          name: data.name || "",
          email: data.email || "",
          phone: data.phone || "",
          address: data.address || "",
        });
      }
    } catch (error) {
      console.error("Failed to load client info:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (e) => {
    e.preventDefault();
    setSaving(true);
    setMessage("");

    try {
      const res = await fetch(`${API_BASE}/api/client/update`, {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(formData),
      });

      if (res.ok) {
        setMessage("success");
        setTimeout(() => setMessage(""), 5000);
      } else {
        setMessage("error");
      }
    } catch (error) {
      console.error("Failed to save settings:", error);
      setMessage("error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Page Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Settings</h1>
        <p className="text-gray-600 mt-2 text-sm">Manage your account and preferences.</p>
      </div>

      {/* Message */}
      {message && (
        <Card
          className={`border-l-4 shadow-sm ${
            message === "success"
              ? "bg-green-50 border-green-300"
              : "bg-red-50 border-red-300"
          }`}
        >
          <CardContent className="pt-6 flex items-center gap-3">
            {message === "success" ? (
              <>
                <CheckCircle2 className="w-5 h-5 text-green-600 flex-shrink-0" />
                <span className="text-sm font-medium text-green-800">Settings saved successfully!</span>
              </>
            ) : (
              <>
                <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0" />
                <span className="text-sm font-medium text-red-800">Failed to save settings. Please try again.</span>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {/* Profile Information */}
      <Card className="border border-gray-200 shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold text-gray-900">Profile Information</CardTitle>
          <CardDescription className="text-sm">Update your account details</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSave} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Company Name
                </label>
                <Input
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder="Your company name"
                  className="border-gray-300"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Email Address
                </label>
                <Input
                  type="email"
                  value={formData.email}
                  onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                  placeholder="contact@example.com"
                  className="border-gray-300"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Phone Number
                </label>
                <Input
                  value={formData.phone}
                  onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                  placeholder="+1 (555) 000-0000"
                  className="border-gray-300"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Address
                </label>
                <Input
                  value={formData.address}
                  onChange={(e) => setFormData({ ...formData, address: e.target.value })}
                  placeholder="Street address"
                  className="border-gray-300"
                />
              </div>
            </div>

            <div className="flex justify-end pt-2">
              <Button
                type="submit"
                disabled={saving}
                className="bg-blue-600 hover:bg-blue-700 text-white"
              >
                {saving ? "Saving..." : "Save Changes"}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Password */}
      <Card className="border border-gray-200 shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold text-gray-900">Password</CardTitle>
          <CardDescription className="text-sm">Change your password regularly for security</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Current Password
              </label>
              <Input type="password" placeholder="••••••••" className="border-gray-300" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                New Password
              </label>
              <Input type="password" placeholder="••••••••" className="border-gray-300" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Confirm New Password
              </label>
              <Input type="password" placeholder="••••••••" className="border-gray-300" />
            </div>

            <div className="flex justify-end pt-2">
              <Button type="submit" variant="outline" className="border-gray-300">
                Update Password
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className="border border-red-200 bg-red-50 shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-semibold text-red-900">Danger Zone</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium text-red-900">Sign Out</p>
              <p className="text-sm text-red-700 mt-1">
                This will log you out. You'll need to sign in again.
              </p>
            </div>
            <Button
              onClick={onLogout}
              className="bg-red-600 hover:bg-red-700 text-white"
            >
              Sign Out
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
