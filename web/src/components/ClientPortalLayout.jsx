import { useState } from "react";
import { useNavigate, useLocation, NavLink } from "react-router-dom";
import {
  LayoutDashboard,
  Monitor,
  Ticket,
  FileText,
  Settings,
  LogOut,
  Menu,
  X,
  Bell,
  Search,
  ChevronDown,
  Home,
  Zap,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const CLIENT_NAV_ITEMS = [
  { path: "/client/portal", label: "Dashboard", icon: LayoutDashboard },
  { path: "/client/portal/devices", label: "Devices", icon: Monitor },
  { path: "/client/portal/tickets", label: "Support Tickets", icon: Ticket },
  { path: "/client/portal/invoices", label: "Invoices", icon: FileText },
  { path: "/client/portal/settings", label: "Settings", icon: Settings },
];

export function ClientPortalLayout({ children, clientName, role, onLogout, unreadCount = 0 }) {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const navigate = useNavigate();
  const location = useLocation();

  return (
    <div className="flex h-screen bg-white">
      {/* Sidebar */}
      <div
        className={cn(
          "flex flex-col bg-[#1F2937] text-gray-100 border-r border-gray-800 transition-all duration-300",
          sidebarOpen ? "w-64" : "w-20"
        )}
      >
        {/* Logo */}
        <div className="flex items-center justify-between px-4 py-6 border-b border-gray-700">
          <div className={cn("flex items-center gap-3", !sidebarOpen && "justify-center w-full")}>
            <div className="w-8 h-8 bg-gradient-to-br from-cyan-400 to-blue-500 rounded-md flex items-center justify-center flex-shrink-0">
              <Zap className="w-5 h-5 text-white" />
            </div>
            {sidebarOpen && <span className="font-bold text-base text-gray-100">GoMesh</span>}
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-2 py-4 space-y-1 overflow-y-auto">
          {CLIENT_NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const isActive = location.pathname === item.path;
            return (
              <NavLink
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-3 px-3 py-2.5 rounded text-sm transition-all duration-150",
                  isActive
                    ? "bg-gray-700 text-white"
                    : "text-gray-400 hover:text-gray-200 hover:bg-gray-800"
                )}
              >
                <Icon className="w-4 h-4 flex-shrink-0" />
                {sidebarOpen && <span className="font-medium">{item.label}</span>}
              </NavLink>
            );
          })}
        </nav>

        {/* Footer */}
        <div className="px-2 py-4 border-t border-gray-700 space-y-1">
          <button
            onClick={onLogout}
            className={cn(
              "flex items-center gap-3 w-full px-3 py-2.5 rounded text-sm transition-all duration-150",
              "text-gray-400 hover:text-gray-200 hover:bg-gray-800"
            )}
          >
            <LogOut className="w-4 h-4 flex-shrink-0" />
            {sidebarOpen && <span className="font-medium">Sign Out</span>}
          </button>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden bg-gray-50">
        {/* Header */}
        <header className="bg-white border-b border-gray-200 sticky top-0 z-40 shadow-sm">
          <div className="flex items-center justify-between h-16 px-8 gap-6">
            {/* Left side - Menu and Search */}
            <div className="flex items-center gap-4 flex-1">
              {!sidebarOpen && (
                <button
                  onClick={() => setSidebarOpen(true)}
                  className="text-gray-600 hover:text-gray-900 p-1.5 hover:bg-gray-100 rounded"
                >
                  <Menu className="w-5 h-5" />
                </button>
              )}
              <div className="max-w-md flex-1">
                <div className="relative">
                  <Search className="absolute left-3 top-2.5 w-4 h-4 text-gray-400" />
                  <Input
                    placeholder="Search..."
                    className="pl-10 bg-gray-50 border-gray-300 text-sm rounded-md focus:bg-white focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                  />
                </div>
              </div>
            </div>

            {/* Right side - Notifications and User */}
            <div className="flex items-center gap-6">
              {/* Notifications */}
              <button className="relative p-1.5 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors">
                <Bell className="w-5 h-5" />
                {unreadCount > 0 && (
                  <span className="absolute top-0 right-0 w-2 h-2 bg-red-500 rounded-full"></span>
                )}
              </button>

              {/* Divider */}
              <div className="h-6 w-px bg-gray-200"></div>

              {/* User Menu */}
              <div className="flex items-center gap-3">
                <div className="text-right hidden sm:block">
                  <p className="text-sm font-medium text-gray-900">{clientName}</p>
                  <p className="text-xs text-gray-500">{role}</p>
                </div>
                <div className="w-8 h-8 bg-gradient-to-br from-cyan-400 to-blue-500 rounded-full flex items-center justify-center text-white font-bold text-sm flex-shrink-0">
                  {clientName
                    ?.split(" ")
                    .map((n) => n[0])
                    .join("")
                    .toUpperCase() || "C"}
                </div>
              </div>
            </div>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-1 overflow-auto">
          <div className="p-8">{children}</div>
        </main>
      </div>
    </div>
  );
}
