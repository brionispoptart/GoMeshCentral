import { useState, useEffect } from "react";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Card } from "../components/ui/card";

export default function AdminBranding({ token, orgId }) {
  const [branding, setBranding] = useState({
    companyName: "",
    phoneNumber: "",
    website: "",
    email: "",
    logo: "",
    icon: "",
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    loadBranding();
  }, []);

  const loadBranding = async () => {
    try {
      const response = await fetch("/api/admin/branding", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (response.ok) {
        const data = await response.json();
        setBranding({
          companyName: data.companyName || "",
          phoneNumber: data.phoneNumber || "",
          website: data.website || "",
          email: data.email || "",
          logo: data.logo || "",
          icon: data.icon || "",
        });
      }
    } catch (err) {
      setMessage("Failed to load branding settings");
    } finally {
      setLoading(false);
    }
  };

  const handleChange = (field, value) => {
    setBranding((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleImageUpload = async (field, file) => {
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e) => {
      const base64 = e.target.result;
      setBranding((prev) => ({
        ...prev,
        [field]: base64,
      }));
      setMessage(`${field === "logo" ? "Logo" : "Icon"} uploaded`);
    };
    reader.readAsDataURL(file);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const response = await fetch("/api/admin/branding", {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(branding),
      });

      if (response.ok) {
        setMessage("Branding saved successfully!");
        setTimeout(() => setMessage(""), 3000);
      } else {
        const error = await response.text();
        setMessage(`Error: ${error}`);
      }
    } catch (err) {
      setMessage("Failed to save branding: " + err.message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="p-8">Loading branding settings...</div>;
  }

  return (
    <div className="max-w-2xl mx-auto p-8">
      <div className="mb-8">
        <h2 className="text-3xl font-bold mb-2">Company Branding</h2>
        <p className="text-gray-600">
          Customize your company's name and contact information that appears
          throughout the system.
        </p>
      </div>

      {message && (
        <div className="mb-6 p-4 bg-blue-50 border border-blue-200 rounded-lg text-sm text-blue-800">
          {message}
        </div>
      )}

      <Card>
        <div className="p-6 space-y-6">
          {/* Company Name */}
          <div>
            <label className="block text-sm font-medium mb-2">
              Company Name
            </label>
            <Input
              placeholder="Enter your company name"
              value={branding.companyName}
              onChange={(e) => handleChange("companyName", e.target.value)}
            />
            <p className="text-xs text-gray-500 mt-1">
              Displayed in invoices, emails, and throughout the app
            </p>
          </div>

          {/* Phone Number */}
          <div>
            <label className="block text-sm font-medium mb-2">
              Phone Number
            </label>
            <Input
              placeholder="e.g., +1 (555) 123-4567"
              value={branding.phoneNumber}
              onChange={(e) => handleChange("phoneNumber", e.target.value)}
            />
            <p className="text-xs text-gray-500 mt-1">
              Appears in invoices and customer-facing materials
            </p>
          </div>

          {/* Website */}
          <div>
            <label className="block text-sm font-medium mb-2">Website</label>
            <Input
              placeholder="e.g., https://www.yourcompany.com"
              value={branding.website}
              onChange={(e) => handleChange("website", e.target.value)}
            />
            <p className="text-xs text-gray-500 mt-1">
              Your company's website URL
            </p>
          </div>

          {/* Email */}
          <div>
            <label className="block text-sm font-medium mb-2">Email</label>
            <Input
              type="email"
              placeholder="e.g., support@yourcompany.com"
              value={branding.email}
              onChange={(e) => handleChange("email", e.target.value)}
            />
            <p className="text-xs text-gray-500 mt-1">
              Support or contact email address
            </p>
          </div>

          {/* Logo Upload */}
          <div>
            <label className="block text-sm font-medium mb-2">
              Company Logo
            </label>
            <div className="border-2 border-dashed border-gray-300 rounded-lg p-4">
              {branding.logo && (
                <div className="mb-4">
                  <img
                    src={branding.logo}
                    alt="Company Logo"
                    className="h-24 w-auto"
                  />
                </div>
              )}
              <input
                type="file"
                accept="image/*"
                onChange={(e) =>
                  handleImageUpload("logo", e.target.files?.[0])
                }
                className="block w-full text-sm text-gray-600 file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
              />
              <p className="text-xs text-gray-500 mt-2">
                PNG or JPG, max 5MB. Used in invoices and app header.
              </p>
            </div>
          </div>

          {/* Icon Upload */}
          <div>
            <label className="block text-sm font-medium mb-2">
              App Icon
            </label>
            <div className="border-2 border-dashed border-gray-300 rounded-lg p-4">
              {branding.icon && (
                <div className="mb-4">
                  <img
                    src={branding.icon}
                    alt="App Icon"
                    className="h-12 w-12 rounded-lg"
                  />
                </div>
              )}
              <input
                type="file"
                accept="image/*"
                onChange={(e) => handleImageUpload("icon", e.target.files?.[0])}
                className="block w-full text-sm text-gray-600 file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
              />
              <p className="text-xs text-gray-500 mt-2">
                PNG or JPG, max 5MB. Square icon displayed in the app header.
              </p>
            </div>
          </div>

          {/* Save Button */}
          <div className="flex gap-2 pt-4">
            <Button
              onClick={handleSave}
              disabled={saving}
              className="bg-blue-600 text-white hover:bg-blue-700"
            >
              {saving ? "Saving..." : "Save Branding"}
            </Button>
            <Button
              onClick={loadBranding}
              disabled={saving}
              variant="outline"
            >
              Cancel
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
