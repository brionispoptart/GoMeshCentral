import { useEffect, useMemo, useRef, useState } from "react";
import { BrowserRouter, NavLink, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { ClientLogin } from "./pages/ClientLogin";
import { ClientPortal } from "./pages/ClientPortal";
import { PendingApprovals } from "./pages/PendingApprovals";
import AdminCustomFields from "./pages/AdminCustomFields";
import { AdminDownloads } from "./pages/AdminDownloads";
import {
  CheckCircle2,
  Clock3,
  Monitor,
  RefreshCw,
  LayoutDashboard,
  Server,
  Ticket,
  Building2,
  BarChart3,
  Zap,
  Bell,
  TerminalSquare,
  Settings,
  Search,
  LogOut,
  Bot,
  Menu,
  X,
  Home,
  Users,
  FileText,
  Calendar,
  AlertTriangle,
  HardDrive,
  Eye,
  Cpu,
  Code,
  Network,
  Folder,
  Database,
  ChevronDown,
} from "lucide-react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "";
const AI_PROVIDER_URLS = {
  openrouter: "https://openrouter.ai/api/v1",
  hermes: "http://localhost:11434/v1",
};

// Grouped and ordered to mirror Atera's left-nav layout: Dashboard, Devices,
// Tickets, Customers (with Contracts/Billing nested under it), Reports,
// Automation, Alerts, Remote Access, then Admin at the bottom.
const FEATURE_SECTIONS = [
  {
    title: "Main",
    items: [
      { path: "/overview", label: "Dashboard", status: "implemented", description: "Program dashboard and delivery tracking." },
      { path: "/tickets", label: "Tickets", status: "implemented", description: "Helpdesk ticket queue and lifecycle." },
      { path: "/clients", label: "Customers", status: "implemented", description: "Organize and sort customers/companies." },
      { path: "/devices", label: "Devices", status: "implemented", description: "Live devices, heartbeat state, and quick commands." },
      { path: "/alerts", label: "Alerts", status: "implemented", description: "Threshold and policy-based alerting workflows." },
      { path: "/reports", label: "Reports", status: "implemented", description: "Operational endpoint inventory and health reporting." },
      { path: "/billing", label: "Billing", status: "implemented", description: "Invoices and line-item billing per client." },
    ],
  },
];

// Admin menu items (shown in collapsed submenu)
const ADMIN_ITEMS = [
  { path: "/users", label: "Users & Roles", status: "implemented", description: "Account and role management." },
  { path: "/enrollment", label: "Enrollment", status: "implemented", description: "Agent enrollment token lifecycle." },
  { path: "/downloads", label: "Agent Downloads", status: "implemented", description: "Download agent binaries for Windows and Linux." },
  { path: "/events", label: "Audit Log", status: "implemented", description: "Timeline of server, user, and endpoint events." },
  { path: "/work-queue", label: "Work Queue", status: "implemented", description: "Live execution tracker from repository planning docs." },
  { path: "/device-groups", label: "Device Groups", status: "implemented", description: "Organize endpoints by site, role, or environment." },
  { path: "/custom-fields", label: "Custom Fields", status: "implemented", description: "Define metadata fields for device tracking." },
  { path: "/approvals", label: "Pending Approvals", status: "implemented", description: "Review and approve client-submitted tickets." },
  { path: "/contracts", label: "Contracts", status: "implemented", description: "Service agreements and billing terms per client." },
  { path: "/scripts", label: "Scripts", status: "implemented", description: "Command/script library and staged execution." },
  { path: "/policies", label: "Policies", status: "planned", description: "Endpoint policy templates and assignment." },
  { path: "/terminal", label: "Terminal", status: "implemented", description: "Interactive shell relay sessions for enrolled Linux agents." },
  { path: "/files", label: "File Transfer", status: "implemented", description: "Upload, download, and file operations." },
  { path: "/desktop", label: "Remote Desktop", status: "planned", description: "Screen streaming and remote control." },
  { path: "/relay", label: "Relay / Routing", status: "planned", description: "Mesh relay and transport topology." },
  { path: "/amt", label: "Intel AMT", status: "planned", description: "Out-of-band management integration." },
  { path: "/branding", label: "Branding", status: "implemented", description: "Customize company name, logo, and contact information." },
  { path: "/settings", label: "Settings", status: "implemented", description: "Global server and tenant settings." },
];

// App Center items
const APP_CENTER_ITEMS = [
  { path: "/assistant", label: "AI Center", status: "implemented", description: "Inspect system context and approve AI-proposed actions." },
  { path: "/integrations", label: "Integrations", status: "planned", description: "External systems and webhooks." },
];

// Icon shown next to each nav section header in the sidebar.
const SECTION_ICONS = {
  Main: LayoutDashboard,
};

function apiUrl(path) {
  return `${API_BASE}${path}`;
}

function wsBase() {
  if (API_BASE) {
    const u = new URL(API_BASE, window.location.origin);
    u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
    return u.origin;
  }
  return `${window.location.protocol === "https:" ? "wss" : "ws"}://${window.location.host}`;
}

function roleVariant(role) {
  if (role === "admin") return "default";
  if (role === "operator") return "secondary";
  return "outline";
}

function statusBadgeVariant(status) {
  return status === "implemented" ? "default" : "outline";
}

function statusText(status) {
  return status === "implemented" ? "Implemented" : "Planned";
}

function getDeviceHealth(device) {
  if (!device) return { label: "Unknown", variant: "outline" };

  if (device.connected) {
    return { label: "Connected", variant: "default" };
  }

  const lastHeartbeat = device.lastHeartbeat ? new Date(device.lastHeartbeat).getTime() : null;
  if (lastHeartbeat && Date.now() - lastHeartbeat < 10 * 60 * 1000) {
    return { label: "Stale", variant: "secondary" };
  }

  return { label: "Offline", variant: "outline" };
}

function flattenFeatures() {
  return [
    ...FEATURE_SECTIONS.flatMap((section) => section.items),
    ...APP_CENTER_ITEMS,
    ...ADMIN_ITEMS,
  ];
}

function getNavigationIcon(path) {
  const iconClass = "h-5 w-5";
  const iconMap = {
    "/overview": <Home className={iconClass} />,
    "/work-queue": <Cpu className={iconClass} />,
    "/devices": <HardDrive className={iconClass} />,
    "/device-groups": <Database className={iconClass} />,
    "/tickets": <Ticket className={iconClass} />,
    "/approvals": <Eye className={iconClass} />,
    "/clients": <Building2 className={iconClass} />,
    "/contracts": <FileText className={iconClass} />,
    "/billing": <BarChart3 className={iconClass} />,
    "/reports": <BarChart3 className={iconClass} />,
    "/assistant": <Bot className={iconClass} />,
    "/scripts": <Code className={iconClass} />,
    "/policies": <Zap className={iconClass} />,
    "/alerts": <AlertTriangle className={iconClass} />,
    "/terminal": <TerminalSquare className={iconClass} />,
    "/files": <Folder className={iconClass} />,
    "/desktop": <Monitor className={iconClass} />,
    "/users": <Users className={iconClass} />,
    "/enrollment": <Network className={iconClass} />,
    "/events": <Clock3 className={iconClass} />,
    "/integrations": <Zap className={iconClass} />,
    "/relay": <Network className={iconClass} />,
    "/amt": <Server className={iconClass} />,
    "/settings": <Settings className={iconClass} />,
  };
  return iconMap[path] || <LayoutDashboard className={iconClass} />;
}

// Reads the active route inside the Router tree to show a page title/description
// in the top bar, mirroring a typical SaaS dashboard header.
function CurrentPageTitle({ allFeatures }) {
  const location = useLocation();
  const current = allFeatures.find((f) => f.path === location.pathname);
  return (
    <div className="min-w-0">
      <p className="truncate text-base font-semibold">{current?.label || "Dashboard"}</p>
      {current?.description && (
        <p className="hidden truncate text-xs text-muted-foreground sm:block">{current.description}</p>
      )}
    </div>
  );
}

function PlaceholderFeature({ title, description }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-sm text-muted-foreground">
        <p>This page is intentionally scaffolded as a milestone placeholder.</p>
        <div className="rounded-md border border-dashed p-4">
          <p className="font-medium text-foreground">Implementation checklist</p>
          <ul className="mt-2 list-disc space-y-1 pl-5">
            <li>Backend API contract and authz rules</li>
            <li>Agent protocol support</li>
            <li>UI workflows and validation</li>
            <li>Audit/event logging</li>
            <li>End-to-end test coverage</li>
          </ul>
        </div>
      </CardContent>
    </Card>
  );
}

function OverviewPage({ session, devices, users }) {
  const allFeatures = flattenFeatures();
  const implementedCount = allFeatures.filter((f) => f.status === "implemented").length;
  const plannedCount = allFeatures.length - implementedCount;

  const connectedCount = devices.filter((d) => d.connected).length;
  const offlineCount = devices.filter((d) => !d.connected).length;
  const staleCount = devices.filter((d) => !d.connected && d.lastHeartbeat && Date.now() - new Date(d.lastHeartbeat).getTime() < 10 * 60 * 1000).length;

  return (
    <div className="space-y-6">
      <Card className="bg-white border-gray-200 shadow-sm">
        <CardHeader className="pb-4">
          <CardTitle className="text-2xl font-bold text-gray-900">Program Overview</CardTitle>
          <CardDescription className="text-gray-600 mt-1">
            Signed in as <span className="font-medium">{session.username}</span> ({session.role})
          </CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Connected Devices</p>
            <p className="mt-2 text-3xl font-bold text-gray-900">{connectedCount}</p>
            <p className="mt-1 text-xs text-gray-500">Out of {devices.length} total</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Known Devices</p>
            <p className="mt-2 text-3xl font-bold text-gray-900">{devices.length}</p>
            <p className="mt-1 text-xs text-gray-500">Registered endpoints</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Users</p>
            <p className="mt-2 text-3xl font-bold text-gray-900">{users.length || "-"}</p>
            <p className="mt-1 text-xs text-gray-500">Active accounts</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Roadmap Progress</p>
            <p className="mt-2 text-3xl font-bold text-gray-900">{implementedCount}/{allFeatures.length}</p>
            <p className="mt-1 text-xs text-gray-500">Features complete</p>
          </div>
        </CardContent>
      </Card>

      <Card className="bg-white border-gray-200 shadow-sm">
        <CardHeader className="pb-4">
          <CardTitle className="text-xl font-bold text-gray-900">System Status</CardTitle>
          <CardDescription className="text-gray-600 mt-1">Live endpoint health at a glance.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Healthy</p>
            <div className="mt-3 flex items-center justify-between">
              <p className="text-3xl font-bold text-green-600">{connectedCount}</p>
              <Badge className="bg-green-100 text-green-800 border-0 px-3 py-1">Connected</Badge>
            </div>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Stale</p>
            <div className="mt-3 flex items-center justify-between">
              <p className="text-3xl font-bold text-yellow-600">{staleCount}</p>
              <Badge className="bg-yellow-100 text-yellow-800 border-0 px-3 py-1">Watch</Badge>
            </div>
          </div>
          <div className="rounded-lg border border-gray-200 bg-white p-4 hover:shadow-md transition-shadow">
            <p className="text-xs font-medium text-gray-600 uppercase tracking-wide">Offline</p>
            <div className="mt-3 flex items-center justify-between">
              <p className="text-3xl font-bold text-red-600">{offlineCount - staleCount}</p>
              <Badge className="bg-red-100 text-red-800 border-0 px-3 py-1">Offline</Badge>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card className="bg-white border-gray-200 shadow-sm">
        <CardHeader className="pb-4">
          <CardTitle className="text-xl font-bold text-gray-900">Feature Tracker</CardTitle>
          <CardDescription className="text-gray-600 mt-1">
            Implemented: <span className="font-semibold text-green-600">{implementedCount}</span> · Planned: <span className="font-semibold text-blue-600">{plannedCount}</span>
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {FEATURE_SECTIONS.map((section) => (
            <div key={section.title} className="rounded-lg border border-gray-200 bg-gray-50 p-4">
              <p className="mb-3 text-sm font-semibold text-gray-900">{section.title}</p>
              <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
                {section.items.map((item) => (
                  <div key={item.path} className="flex items-center justify-between rounded-md bg-white border border-gray-200 px-3 py-2.5 text-sm hover:bg-gray-50 transition-colors">
                    <span className="text-gray-700">{item.label}</span>
                    <Badge variant={statusBadgeVariant(item.status)} className={statusBadgeVariant(item.status) === "default" ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-700"}>{statusText(item.status)}</Badge>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

function DevicesPage({ devices, clients, groups, canCommand, canManageUsers, canManagePSA, onSendPing, onDeleteDevice, onAssignDeviceClient, onAssignDeviceGroup, appStatus }) {
  const [clientFilter, setClientFilter] = useState("all");
  const [groupFilter, setGroupFilter] = useState("all");
  const [sortBy, setSortBy] = useState("name");

  const clientNameById = useMemo(() => {
    const map = new Map();
    for (const c of clients || []) map.set(c.id, c.name);
    return map;
  }, [clients]);

  const groupNameById = useMemo(() => {
    const map = new Map();
    for (const g of groups || []) map.set(g.id, g.name);
    return map;
  }, [groups]);

  const filteredSorted = useMemo(() => {
    let list = devices;
    if (clientFilter === "unassigned") {
      list = list.filter((d) => !d.clientId);
    } else if (clientFilter !== "all") {
      list = list.filter((d) => d.clientId === clientFilter);
    }
    if (groupFilter === "unassigned") {
      list = list.filter((d) => !d.groupId);
    } else if (groupFilter !== "all") {
      list = list.filter((d) => d.groupId === groupFilter);
    }
    const sorted = [...list];
    sorted.sort((a, b) => {
      if (sortBy === "client") {
        const an = clientNameById.get(a.clientId) || "";
        const bn = clientNameById.get(b.clientId) || "";
        return an.localeCompare(bn);
      }
      if (sortBy === "group") {
        const an = groupNameById.get(a.groupId) || "";
        const bn = groupNameById.get(b.groupId) || "";
        return an.localeCompare(bn);
      }
      if (sortBy === "connected") {
        return (b.connected ? 1 : 0) - (a.connected ? 1 : 0);
      }
      return (a.name || a.id || "").localeCompare(b.name || b.id || "");
    });
    return sorted;
  }, [devices, clientFilter, groupFilter, sortBy, clientNameById, groupNameById]);

  return (
    <Card className="bg-white border-gray-200 shadow-sm">
      <CardHeader className="pb-4">
        <CardTitle className="text-xl font-bold text-gray-900">Connected Devices</CardTitle>
        <CardDescription className="text-gray-600 mt-1">Endpoint inventory, client assignment, quick command dispatch, and delete lifecycle action.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-[200px_200px_200px_1fr]">
          <select
            className="flex h-10 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            value={clientFilter}
            onChange={(e) => setClientFilter(e.target.value)}
          >
            <option value="all">All Clients</option>
            <option value="unassigned">Unassigned</option>
            {(clients || []).map((c) => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <select
            className="flex h-10 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            value={groupFilter}
            onChange={(e) => setGroupFilter(e.target.value)}
          >
            <option value="all">All Groups</option>
            <option value="unassigned">Ungrouped</option>
            {(groups || []).map((g) => (
              <option key={g.id} value={g.id}>{g.name}</option>
            ))}
          </select>
          <select
            className="flex h-10 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value)}
          >
            <option value="name">Sort: Name</option>
            <option value="client">Sort: Client</option>
            <option value="group">Sort: Group</option>
            <option value="connected">Sort: Connected</option>
          </select>
        </div>
        <div className="overflow-x-auto rounded-lg border border-gray-200 shadow-sm">
          <Table>
            <TableHeader className="bg-gray-50 border-b border-gray-200">
              <TableRow className="hover:bg-gray-50">
                <TableHead className="font-semibold text-gray-700">ID</TableHead>
                <TableHead className="font-semibold text-gray-700">Name</TableHead>
                <TableHead className="font-semibold text-gray-700">Client</TableHead>
                <TableHead className="font-semibold text-gray-700">Group</TableHead>
                <TableHead className="font-semibold text-gray-700">Connected</TableHead>
                <TableHead className="font-semibold text-gray-700">Last Heartbeat</TableHead>
                <TableHead className="font-semibold text-gray-700">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredSorted.map((d) => (
                <TableRow key={d.id} className="hover:bg-gray-50 border-gray-200">
                  <TableCell className="text-gray-600">{d.id || "-"}</TableCell>
                  <TableCell className="font-medium text-gray-900">{d.name || "-"}</TableCell>
                  <TableCell>
                    {canManagePSA ? (
                      <select
                        className="flex h-8 w-full min-w-[140px] rounded-md border border-gray-300 bg-white px-2 py-1 text-xs hover:bg-gray-50"
                        value={d.clientId || ""}
                        onChange={(e) => onAssignDeviceClient(d.id, e.target.value)}
                      >
                        <option value="">Unassigned</option>
                        {(clients || []).map((c) => (
                          <option key={c.id} value={c.id}>{c.name}</option>
                        ))}
                      </select>
                    ) : (
                      <span className="text-gray-600">{clientNameById.get(d.clientId) || "-"}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {canCommand ? (
                      <select
                        className="flex h-8 w-full min-w-[120px] rounded-md border border-gray-300 bg-white px-2 py-1 text-xs hover:bg-gray-50"
                        value={d.groupId || ""}
                        onChange={(e) => onAssignDeviceGroup(d.id, e.target.value)}
                      >
                        <option value="">Ungrouped</option>
                        {(groups || []).map((g) => (
                          <option key={g.id} value={g.id}>{g.name}</option>
                        ))}
                      </select>
                    ) : (
                      <span className="text-gray-600">{groupNameById.get(d.groupId) || "-"}</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={getDeviceHealth(d).variant}>{getDeviceHealth(d).label}</Badge>
                  </TableCell>
                  <TableCell>{d.lastHeartbeat ? new Date(d.lastHeartbeat).toLocaleString() : "-"}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap items-center gap-2">
                      {canCommand ? <Button size="sm" variant="outline" onClick={() => onSendPing(d.id)}>Ping</Button> : null}
                      {canManageUsers ? (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => onDeleteDevice(d.id)}
                          disabled={d.connected}
                          title={d.connected ? "Connected devices cannot be deleted" : "Delete device and related records"}
                        >
                          Delete
                        </Button>
                      ) : null}
                      {!canCommand && !canManageUsers ? <span className="text-muted-foreground">-</span> : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
      </CardContent>
    </Card>
  );
}

function ClientsPage({ clients, canManagePSA, createClient, deleteClient, appStatus }) {
  const [form, setForm] = useState({ name: "", contactName: "", contactEmail: "", contactPhone: "", address: "", notes: "" });

  function submit() {
    if (!form.name.trim()) return;
    createClient(form);
    setForm({ name: "", contactName: "", contactEmail: "", contactPhone: "", address: "", notes: "" });
  }

  return (
    <div className="space-y-4">
      {canManagePSA && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">New Client</CardTitle>
            <CardDescription>Add a company/customer to organize devices, contracts, tickets, and invoices.</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-2 md:grid-cols-3">
            <Input placeholder="Client name" value={form.name} onChange={(e) => setForm((v) => ({ ...v, name: e.target.value }))} />
            <Input placeholder="Contact name" value={form.contactName} onChange={(e) => setForm((v) => ({ ...v, contactName: e.target.value }))} />
            <Input placeholder="Contact email" value={form.contactEmail} onChange={(e) => setForm((v) => ({ ...v, contactEmail: e.target.value }))} />
            <Input placeholder="Contact phone" value={form.contactPhone} onChange={(e) => setForm((v) => ({ ...v, contactPhone: e.target.value }))} />
            <Input placeholder="Address" value={form.address} onChange={(e) => setForm((v) => ({ ...v, address: e.target.value }))} />
            <Input placeholder="Notes" value={form.notes} onChange={(e) => setForm((v) => ({ ...v, notes: e.target.value }))} />
            <Button size="sm" onClick={submit}>Create Client</Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Clients</CardTitle>
          <CardDescription>All organizations tracked in this workspace.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Contact</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Phone</TableHead>
                  {canManagePSA && <TableHead>Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {clients.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell>{c.name}</TableCell>
                    <TableCell>{c.contactName || "-"}</TableCell>
                    <TableCell>{c.contactEmail || "-"}</TableCell>
                    <TableCell>{c.contactPhone || "-"}</TableCell>
                    {canManagePSA && (
                      <TableCell>
                        <Button size="sm" variant="outline" onClick={() => deleteClient(c.id)}>Delete</Button>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {clients.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-muted-foreground">No clients yet.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

function ContractsPage({ contracts, clients, canManagePSA, createContract, generateContractInvoice, appStatus }) {
  const [form, setForm] = useState({ clientId: "", name: "", contractType: "Managed Services", rateType: "monthly", rateAmount: "", billingCycle: "monthly", notes: "" });

  const clientNameById = useMemo(() => {
    const map = new Map();
    for (const c of clients || []) map.set(c.id, c.name);
    return map;
  }, [clients]);

  function submit() {
    if (!form.clientId || !form.name.trim()) return;
    createContract({ ...form, rateAmount: Number(form.rateAmount) || 0 });
    setForm({ clientId: "", name: "", contractType: "Managed Services", rateType: "monthly", rateAmount: "", billingCycle: "monthly", notes: "" });
  }

  return (
    <div className="space-y-4">
      {canManagePSA && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">New Contract</CardTitle>
            <CardDescription>Define a service agreement and billing terms for a client.</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-2 md:grid-cols-3">
            <select
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={form.clientId}
              onChange={(e) => setForm((v) => ({ ...v, clientId: e.target.value }))}
            >
              <option value="">Select client</option>
              {(clients || []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
            <Input placeholder="Contract name" value={form.name} onChange={(e) => setForm((v) => ({ ...v, name: e.target.value }))} />
            <Input placeholder="Contract type (e.g. Managed Services)" value={form.contractType} onChange={(e) => setForm((v) => ({ ...v, contractType: e.target.value }))} />
            <select
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={form.rateType}
              onChange={(e) => setForm((v) => ({ ...v, rateType: e.target.value }))}
            >
              <option value="monthly">Monthly Flat Rate</option>
              <option value="hourly">Hourly</option>
              <option value="fixed">Fixed Project</option>
              <option value="per_device">Per Device</option>
            </select>
            <Input placeholder="Rate amount" type="number" value={form.rateAmount} onChange={(e) => setForm((v) => ({ ...v, rateAmount: e.target.value }))} />
            <select
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={form.billingCycle}
              onChange={(e) => setForm((v) => ({ ...v, billingCycle: e.target.value }))}
            >
              <option value="monthly">Bill Monthly</option>
              <option value="quarterly">Bill Quarterly</option>
              <option value="annual">Bill Annually</option>
              <option value="one_time">One Time</option>
            </select>
            <Input placeholder="Notes" value={form.notes} onChange={(e) => setForm((v) => ({ ...v, notes: e.target.value }))} />
            <Button size="sm" onClick={submit}>Create Contract</Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Contracts</CardTitle>
          <CardDescription>Active and historical service agreements.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Client</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Rate</TableHead>
                  <TableHead>Billing Cycle</TableHead>
                  <TableHead>Last Invoiced</TableHead>
                  <TableHead>Status</TableHead>
                  {canManagePSA && <TableHead>Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {contracts.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell>{clientNameById.get(c.clientId) || c.clientId}</TableCell>
                    <TableCell>{c.name}</TableCell>
                    <TableCell>{c.contractType || "-"}</TableCell>
                    <TableCell>{c.rateType}: ${c.rateAmount}</TableCell>
                    <TableCell>{c.billingCycle}</TableCell>
                    <TableCell>{c.lastInvoicedAt ? new Date(c.lastInvoicedAt).toLocaleDateString() : "Never"}</TableCell>
                    <TableCell><Badge variant={c.status === "active" ? "default" : "outline"}>{c.status}</Badge></TableCell>
                    {canManagePSA && (
                      <TableCell>
                        <Button size="sm" variant="outline" onClick={() => generateContractInvoice(c.id)}>Generate Invoice Now</Button>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {contracts.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={7} className="text-muted-foreground">No contracts yet.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

const TICKET_STATUSES = ["open", "in_progress", "waiting", "resolved", "closed"];
const TICKET_PRIORITIES = ["low", "medium", "high", "urgent"];

function TicketsPage({ tickets, clients, devices, canManagePSA, createTicket, updateTicket, appStatus }) {
  const [form, setForm] = useState({ clientId: "", deviceId: "", subject: "", description: "", priority: "medium", assignee: "" });
  const [statusFilter, setStatusFilter] = useState("all");

  const clientNameById = useMemo(() => {
    const map = new Map();
    for (const c of clients || []) map.set(c.id, c.name);
    return map;
  }, [clients]);

  const filtered = useMemo(() => {
    if (statusFilter === "all") return tickets;
    return tickets.filter((t) => t.status === statusFilter);
  }, [tickets, statusFilter]);

  function submit() {
    if (!form.subject.trim()) return;
    createTicket(form);
    setForm({ clientId: "", deviceId: "", subject: "", description: "", priority: "medium", assignee: "" });
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">New Ticket</CardTitle>
          <CardDescription>Log a support request tied to a client and/or device.</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-2 md:grid-cols-3">
          <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.clientId} onChange={(e) => setForm((v) => ({ ...v, clientId: e.target.value }))}>
            <option value="">No client</option>
            {(clients || []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
          <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.deviceId} onChange={(e) => setForm((v) => ({ ...v, deviceId: e.target.value }))}>
            <option value="">No device</option>
            {(devices || []).map((d) => <option key={d.id} value={d.id}>{d.name || d.id}</option>)}
          </select>
          <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.priority} onChange={(e) => setForm((v) => ({ ...v, priority: e.target.value }))}>
            {TICKET_PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
          <Input placeholder="Subject" value={form.subject} onChange={(e) => setForm((v) => ({ ...v, subject: e.target.value }))} className="md:col-span-2" />
          <Input placeholder="Assignee" value={form.assignee} onChange={(e) => setForm((v) => ({ ...v, assignee: e.target.value }))} />
          <Input placeholder="Description" value={form.description} onChange={(e) => setForm((v) => ({ ...v, description: e.target.value }))} className="md:col-span-3" />
          <Button size="sm" onClick={submit}>Create Ticket</Button>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Tickets</CardTitle>
          <CardDescription>Helpdesk queue across all clients.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <select className="flex h-10 w-56 rounded-md border border-input bg-background px-3 py-2 text-sm" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="all">All statuses</option>
            {TICKET_STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Subject</TableHead>
                  <TableHead>Client</TableHead>
                  <TableHead>Priority</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Assignee</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell>{t.subject}</TableCell>
                    <TableCell>{clientNameById.get(t.clientId) || "-"}</TableCell>
                    <TableCell><Badge variant="outline">{t.priority}</Badge></TableCell>
                    <TableCell>
                      <select
                        className="flex h-8 rounded-md border border-input bg-background px-2 py-1 text-xs"
                        value={t.status}
                        onChange={(e) => updateTicket({ ...t, status: e.target.value })}
                      >
                        {TICKET_STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
                      </select>
                    </TableCell>
                    <TableCell>{t.assignee || "-"}</TableCell>
                    <TableCell>{t.createdAt ? new Date(t.createdAt).toLocaleString() : "-"}</TableCell>
                  </TableRow>
                ))}
                {filtered.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-muted-foreground">No tickets match this filter.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

function BillingPage({ token, invoices, clients, canManagePSA, createInvoice, updateInvoice, timeEntries, createTimeEntry, deleteTimeEntry, appStatus }) {
  const [form, setForm] = useState({ clientId: "", notes: "" });
  const [lineItems, setLineItems] = useState([{ description: "", quantity: 1, unitPrice: 0 }]);
  const [generatingPDF, setGeneratingPDF] = useState({});

  const clientNameById = useMemo(() => {
    const map = new Map();
    for (const c of clients || []) map.set(c.id, c.name);
    return map;
  }, [clients]);

  function updateLineItem(idx, key, value) {
    setLineItems((items) => items.map((it, i) => (i === idx ? { ...it, [key]: value } : it)));
  }

  function addLineItem() {
    setLineItems((items) => [...items, { description: "", quantity: 1, unitPrice: 0 }]);
  }

  function submit() {
    if (!form.clientId) return;
    createInvoice({
      clientId: form.clientId,
      notes: form.notes,
      lineItems: lineItems
        .filter((it) => it.description.trim())
        .map((it) => ({ description: it.description, quantity: Number(it.quantity) || 0, unitPrice: Number(it.unitPrice) || 0 })),
    });
    setForm({ clientId: "", notes: "" });
    setLineItems([{ description: "", quantity: 1, unitPrice: 0 }]);
  }

  const generateAndDownloadPDF = async (invoiceId, invoiceNumber) => {
    try {
      setGeneratingPDF((prev) => ({ ...prev, [invoiceId]: true }));

      // First, generate the PDF
      const generateRes = await fetch(`${API_BASE}/api/invoices/${invoiceId}/generate-pdf`, {
        method: "POST",
        headers: { "Authorization": `Bearer ${token}` },
      });

      if (!generateRes.ok) {
        throw new Error(`Failed to generate PDF: ${generateRes.statusText}`);
      }

      const result = await generateRes.json();

      // Then download it
      const downloadRes = await fetch(`${API_BASE}/api/invoices/${invoiceId}/download-pdf`, {
        headers: { "Authorization": `Bearer ${token}` },
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
      console.error("Error downloading PDF:", err);
    } finally {
      setGeneratingPDF((prev) => ({ ...prev, [invoiceId]: false }));
    }
  };

  return (
    <div className="space-y-4">
      {canManagePSA && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">New Invoice</CardTitle>
            <CardDescription>Bill a client with itemized line items; totals compute automatically.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
              <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.clientId} onChange={(e) => setForm((v) => ({ ...v, clientId: e.target.value }))}>
                <option value="">Select client</option>
                {(clients || []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
              </select>
              <Input placeholder="Notes" value={form.notes} onChange={(e) => setForm((v) => ({ ...v, notes: e.target.value }))} />
            </div>
            <div className="space-y-2">
              {lineItems.map((it, idx) => (
                <div key={idx} className="grid grid-cols-1 gap-2 md:grid-cols-[1fr_100px_120px]">
                  <Input placeholder="Description" value={it.description} onChange={(e) => updateLineItem(idx, "description", e.target.value)} />
                  <Input placeholder="Qty" type="number" value={it.quantity} onChange={(e) => updateLineItem(idx, "quantity", e.target.value)} />
                  <Input placeholder="Unit price" type="number" value={it.unitPrice} onChange={(e) => updateLineItem(idx, "unitPrice", e.target.value)} />
                </div>
              ))}
              <Button size="sm" variant="outline" onClick={addLineItem}>Add Line Item</Button>
            </div>
            <Button size="sm" onClick={submit}>Create Invoice</Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Invoices</CardTitle>
          <CardDescription>Billing history across all clients.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Invoice #</TableHead>
                  <TableHead>Client</TableHead>
                  <TableHead>Total</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invoices.map((inv) => (
                  <TableRow key={inv.id}>
                    <TableCell>{inv.invoiceNumber}</TableCell>
                    <TableCell>{clientNameById.get(inv.clientId) || inv.clientId}</TableCell>
                    <TableCell>${inv.total?.toFixed ? inv.total.toFixed(2) : inv.total}</TableCell>
                    <TableCell>
                      {canManagePSA ? (
                        <select
                          className="flex h-8 rounded-md border border-input bg-background px-2 py-1 text-xs"
                          value={inv.status}
                          onChange={(e) => updateInvoice({ ...inv, status: e.target.value })}
                        >
                          <option value="draft">draft</option>
                          <option value="sent">sent</option>
                          <option value="paid">paid</option>
                          <option value="overdue">overdue</option>
                          <option value="void">void</option>
                        </select>
                      ) : (
                        <Badge variant={inv.status === "paid" ? "default" : "outline"}>{inv.status}</Badge>
                      )}
                    </TableCell>
                    <TableCell>{inv.createdAt ? new Date(inv.createdAt).toLocaleString() : "-"}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => generateAndDownloadPDF(inv.id, inv.invoiceNumber)}
                        disabled={generatingPDF[inv.id]}
                        className="gap-2"
                      >
                        {generatingPDF[inv.id] ? (
                          <>
                            <RefreshCw className="w-4 h-4 animate-spin" />
                            Generating...
                          </>
                        ) : (
                          <>
                            <FileText className="w-4 h-4" />
                            PDF
                          </>
                        )}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {invoices.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-muted-foreground">No invoices yet.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
      <TimeEntriesCard clients={clients} timeEntries={timeEntries} canManagePSA={canManagePSA} createTimeEntry={createTimeEntry} deleteTimeEntry={deleteTimeEntry} />
    </div>
  );
}

function TimeEntriesCard({ clients, timeEntries, canManagePSA, createTimeEntry, deleteTimeEntry }) {
  const [form, setForm] = useState({ clientId: "", description: "", minutes: 30, billable: true });

  const clientNameById = useMemo(() => {
    const map = new Map();
    for (const c of clients || []) map.set(c.id, c.name);
    return map;
  }, [clients]);

  function submit() {
    if (!form.clientId || Number(form.minutes) <= 0) return;
    createTimeEntry({ ...form, minutes: Number(form.minutes) });
    setForm({ clientId: "", description: "", minutes: 30, billable: true });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Time Entries</CardTitle>
        <CardDescription>Log billable/non-billable time; billable entries roll into the next generated invoice for that client.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {canManagePSA && (
          <div className="grid grid-cols-1 gap-2 md:grid-cols-[1fr_2fr_100px_120px_auto] items-center">
            <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.clientId} onChange={(e) => setForm((v) => ({ ...v, clientId: e.target.value }))}>
              <option value="">Select client</option>
              {(clients || []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
            <Input placeholder="Description" value={form.description} onChange={(e) => setForm((v) => ({ ...v, description: e.target.value }))} />
            <Input placeholder="Minutes" type="number" value={form.minutes} onChange={(e) => setForm((v) => ({ ...v, minutes: e.target.value }))} />
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={form.billable} onChange={(e) => setForm((v) => ({ ...v, billable: e.target.checked }))} /> Billable
            </label>
            <Button size="sm" onClick={submit}>Log Time</Button>
          </div>
        )}
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Client</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Duration</TableHead>
                <TableHead>Billable</TableHead>
                <TableHead>Invoiced</TableHead>
                {canManagePSA && <TableHead>Actions</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {(timeEntries || []).map((t) => (
                <TableRow key={t.id}>
                  <TableCell>{clientNameById.get(t.clientId) || t.clientId}</TableCell>
                  <TableCell>{t.description || "-"}</TableCell>
                  <TableCell>{(t.minutes / 60).toFixed(2)}h</TableCell>
                  <TableCell><Badge variant={t.billable ? "default" : "outline"}>{t.billable ? "Billable" : "Non-billable"}</Badge></TableCell>
                  <TableCell>{t.invoiceId ? "Yes" : "No"}</TableCell>
                  {canManagePSA && (
                    <TableCell>
                      <Button size="sm" variant="outline" onClick={() => deleteTimeEntry(t.id)} disabled={!!t.invoiceId}>Delete</Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
              {(!timeEntries || timeEntries.length === 0) && (
                <TableRow>
                  <TableCell colSpan={6} className="text-muted-foreground">No time entries logged yet.</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}

function DeviceGroupsPage({ groups, devices, canCommand, createDeviceGroup, deleteDeviceGroup, appStatus }) {
  const [form, setForm] = useState({ name: "", notes: "" });

  const countByGroup = useMemo(() => {
    const map = new Map();
    for (const d of devices || []) {
      if (!d.groupId) continue;
      map.set(d.groupId, (map.get(d.groupId) || 0) + 1);
    }
    return map;
  }, [devices]);

  function submit() {
    if (!form.name.trim()) return;
    createDeviceGroup(form);
    setForm({ name: "", notes: "" });
  }

  return (
    <div className="space-y-4">
      {canCommand && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">New Device Group</CardTitle>
            <CardDescription>Group endpoints by site, role, or environment. Assign devices from the Devices page.</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-2 md:grid-cols-3">
            <Input placeholder="Group name" value={form.name} onChange={(e) => setForm((v) => ({ ...v, name: e.target.value }))} />
            <Input placeholder="Notes" value={form.notes} onChange={(e) => setForm((v) => ({ ...v, notes: e.target.value }))} />
            <Button size="sm" onClick={submit}>Create Group</Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Device Groups</CardTitle>
          <CardDescription>Groups configured in this workspace.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Notes</TableHead>
                  <TableHead>Devices</TableHead>
                  {canCommand && <TableHead>Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.map((g) => (
                  <TableRow key={g.id}>
                    <TableCell>{g.name}</TableCell>
                    <TableCell>{g.notes || "-"}</TableCell>
                    <TableCell>{countByGroup.get(g.id) || 0}</TableCell>
                    {canCommand && (
                      <TableCell>
                        <Button size="sm" variant="outline" onClick={() => deleteDeviceGroup(g.id)}>Delete</Button>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {groups.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-muted-foreground">No device groups yet.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

function ScriptsPage({ scripts, devices, canCommand, createScript, deleteScript, runScript, appStatus }) {
  const [form, setForm] = useState({ name: "", description: "", targetOs: "any", body: "" });
  const [runDeviceByScript, setRunDeviceByScript] = useState({});

  function submit() {
    if (!form.name.trim() || !form.body.trim()) return;
    createScript(form);
    setForm({ name: "", description: "", targetOs: "any", body: "" });
  }

  return (
    <div className="space-y-4">
      {canCommand && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">New Script</CardTitle>
            <CardDescription>Save a reusable command/script to run against any online device.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <div className="grid grid-cols-1 gap-2 md:grid-cols-3">
              <Input placeholder="Script name" value={form.name} onChange={(e) => setForm((v) => ({ ...v, name: e.target.value }))} />
              <Input placeholder="Description" value={form.description} onChange={(e) => setForm((v) => ({ ...v, description: e.target.value }))} />
              <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.targetOs} onChange={(e) => setForm((v) => ({ ...v, targetOs: e.target.value }))}>
                <option value="any">Any OS</option>
                <option value="windows">Windows (PowerShell)</option>
                <option value="linux">Linux (shell)</option>
              </select>
            </div>
            <textarea
              className="flex min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
              placeholder="Command or script body"
              value={form.body}
              onChange={(e) => setForm((v) => ({ ...v, body: e.target.value }))}
            />
            <Button size="sm" onClick={submit}>Save Script</Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Script Library</CardTitle>
          <CardDescription>Run a saved script against any connected device.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            {scripts.map((sc) => (
              <div key={sc.id} className="rounded-md border p-3 text-sm space-y-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="font-medium text-foreground">{sc.name} <Badge variant="outline">{sc.targetOs}</Badge></p>
                    <p className="text-xs text-muted-foreground">{sc.description || "No description"}</p>
                  </div>
                  {canCommand && (
                    <Button size="sm" variant="outline" onClick={() => deleteScript(sc.id)}>Delete</Button>
                  )}
                </div>
                <pre className="rounded bg-muted p-2 text-xs overflow-x-auto">{sc.body}</pre>
                {canCommand && (
                  <div className="flex flex-wrap items-center gap-2">
                    <select
                      className="flex h-8 min-w-[180px] rounded-md border border-input bg-background px-2 py-1 text-xs"
                      value={runDeviceByScript[sc.id] || ""}
                      onChange={(e) => setRunDeviceByScript((v) => ({ ...v, [sc.id]: e.target.value }))}
                    >
                      <option value="">Select device</option>
                      {(devices || []).map((d) => <option key={d.id} value={d.id}>{d.name || d.id}</option>)}
                    </select>
                    <Button
                      size="sm"
                      onClick={() => runDeviceByScript[sc.id] && runScript(sc.id, runDeviceByScript[sc.id])}
                      disabled={!runDeviceByScript[sc.id]}
                    >
                      Run Script
                    </Button>
                  </div>
                )}
              </div>
            ))}
            {scripts.length === 0 && <p className="text-sm text-muted-foreground">No scripts saved yet.</p>}
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

const ALERT_METRIC_TYPES = [
  { value: "cpu", label: "CPU Usage %" },
  { value: "memory", label: "Memory Usage %" },
  { value: "offline", label: "Device Offline" },
];

function AlertsPage({ alertRules, alerts, devices, clients, canCommand, createAlertRule, deleteAlertRule, acknowledgeAlert, resolveAlert, appStatus }) {
  const [form, setForm] = useState({ name: "", metricType: "cpu", comparator: "gt", thresholdValue: 90, severity: "warning", deviceId: "", clientId: "" });
  const [statusFilter, setStatusFilter] = useState("open");

  const deviceNameById = useMemo(() => {
    const map = new Map();
    for (const d of devices || []) map.set(d.id, d.name || d.id);
    return map;
  }, [devices]);

  const filteredAlerts = useMemo(() => {
    if (statusFilter === "all") return alerts;
    return alerts.filter((a) => a.status === statusFilter);
  }, [alerts, statusFilter]);

  function submit() {
    if (!form.name.trim()) return;
    createAlertRule({ ...form, thresholdValue: Number(form.thresholdValue) || 0 });
    setForm({ name: "", metricType: "cpu", comparator: "gt", thresholdValue: 90, severity: "warning", deviceId: "", clientId: "" });
  }

  return (
    <div className="space-y-4">
      {canCommand && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">New Alert Rule</CardTitle>
            <CardDescription>Trigger an alert when a device crosses a CPU/memory threshold or goes offline.</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-2 md:grid-cols-3">
            <Input placeholder="Rule name" value={form.name} onChange={(e) => setForm((v) => ({ ...v, name: e.target.value }))} />
            <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.metricType} onChange={(e) => setForm((v) => ({ ...v, metricType: e.target.value }))}>
              {ALERT_METRIC_TYPES.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
            </select>
            <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.severity} onChange={(e) => setForm((v) => ({ ...v, severity: e.target.value }))}>
              <option value="warning">Warning</option>
              <option value="critical">Critical</option>
            </select>
            {form.metricType !== "offline" && (
              <>
                <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.comparator} onChange={(e) => setForm((v) => ({ ...v, comparator: e.target.value }))}>
                  <option value="gt">Above</option>
                  <option value="lt">Below</option>
                </select>
                <Input placeholder="Threshold %" type="number" value={form.thresholdValue} onChange={(e) => setForm((v) => ({ ...v, thresholdValue: e.target.value }))} />
              </>
            )}
            <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.deviceId} onChange={(e) => setForm((v) => ({ ...v, deviceId: e.target.value }))}>
              <option value="">All devices</option>
              {(devices || []).map((d) => <option key={d.id} value={d.id}>{d.name || d.id}</option>)}
            </select>
            <select className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm" value={form.clientId} onChange={(e) => setForm((v) => ({ ...v, clientId: e.target.value }))}>
              <option value="">All customers</option>
              {(clients || []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
            <Button size="sm" onClick={submit}>Create Rule</Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Alert Rules</CardTitle>
          <CardDescription>Configured thresholds evaluated on every incoming device report.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Metric</TableHead>
                  <TableHead>Condition</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Scope</TableHead>
                  {canCommand && <TableHead>Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {alertRules.map((rule) => (
                  <TableRow key={rule.id}>
                    <TableCell>{rule.name}</TableCell>
                    <TableCell>{ALERT_METRIC_TYPES.find((m) => m.value === rule.metricType)?.label || rule.metricType}</TableCell>
                    <TableCell>{rule.metricType === "offline" ? "-" : `${rule.comparator === "lt" ? "<" : ">"} ${rule.thresholdValue}`}</TableCell>
                    <TableCell><Badge variant={rule.severity === "critical" ? "default" : "outline"}>{rule.severity}</Badge></TableCell>
                    <TableCell>{rule.deviceId ? (deviceNameById.get(rule.deviceId) || rule.deviceId) : (rule.clientId ? "1 customer" : "All devices")}</TableCell>
                    {canCommand && (
                      <TableCell>
                        <Button size="sm" variant="outline" onClick={() => deleteAlertRule(rule.id)}>Delete</Button>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {alertRules.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-muted-foreground">No alert rules configured yet.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Alerts</CardTitle>
          <CardDescription>Triggered alerts across all devices.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <select className="flex h-10 w-56 rounded-md border border-input bg-background px-3 py-2 text-sm" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="open">Open</option>
            <option value="acknowledged">Acknowledged</option>
            <option value="resolved">Resolved</option>
            <option value="all">All</option>
          </select>
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Device</TableHead>
                  <TableHead>Rule</TableHead>
                  <TableHead>Message</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Triggered</TableHead>
                  {canCommand && <TableHead>Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredAlerts.map((a) => (
                  <TableRow key={a.id}>
                    <TableCell>{deviceNameById.get(a.deviceId) || a.deviceId}</TableCell>
                    <TableCell>{a.ruleName}</TableCell>
                    <TableCell>{a.message}</TableCell>
                    <TableCell><Badge variant={a.severity === "critical" ? "default" : "outline"}>{a.severity}</Badge></TableCell>
                    <TableCell><Badge variant={a.status === "open" ? "default" : "outline"}>{a.status}</Badge></TableCell>
                    <TableCell>{a.triggeredAt ? new Date(a.triggeredAt).toLocaleString() : "-"}</TableCell>
                    {canCommand && (
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          {a.status === "open" && <Button size="sm" variant="outline" onClick={() => acknowledgeAlert(a.id)}>Acknowledge</Button>}
                          {a.status !== "resolved" && <Button size="sm" variant="outline" onClick={() => resolveAlert(a.id)}>Resolve</Button>}
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {filteredAlerts.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={7} className="text-muted-foreground">No alerts match this filter.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

function FilesPage({ token, devices, canCommand, listRemoteFiles, uploadRemoteFile, activeClientId, appStatus }) {
  const [selectedDeviceId, setSelectedDeviceId] = useState("");
  const [currentPath, setCurrentPath] = useState("");
  const [pathInput, setPathInput] = useState("");
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(false);
  const [browseError, setBrowseError] = useState("");
  const [uploadTargetName, setUploadTargetName] = useState("");
  const fileInputRef = useRef(null);

  function formatBytes(value) {
    if (!value) return "-";
    const units = ["B", "KB", "MB", "GB"];
    let size = value;
    let unit = 0;
    while (size >= 1024 && unit < units.length - 1) {
      size /= 1024;
      unit += 1;
    }
    return `${size.toFixed(size >= 10 ? 0 : 1)} ${units[unit]}`;
  }

  async function browse(path) {
    if (!selectedDeviceId) {
      setBrowseError("Select a device first");
      return;
    }
    setLoading(true);
    setBrowseError("");
    try {
      const data = await listRemoteFiles(selectedDeviceId, path);
      setEntries(Array.isArray(data) ? data : []);
      setCurrentPath(path || "");
      setPathInput(path || "");
    } catch (err) {
      setBrowseError(err.message);
    } finally {
      setLoading(false);
    }
  }

  function joinPath(base, name) {
    if (!base) return name;
    const sep = base.includes("\\") && !base.includes("/") ? "\\" : "/";
    return base.endsWith(sep) ? `${base}${name}` : `${base}${sep}${name}`;
  }

  function downloadUrl(path) {
    const clientQuery = activeClientId ? `&clientId=${encodeURIComponent(activeClientId)}` : "";
    return apiUrl(`/api/devices/${encodeURIComponent(selectedDeviceId)}/files/download?path=${encodeURIComponent(path)}&token=${encodeURIComponent(token)}${clientQuery}`);
  }

  async function handleUploadFile(e) {
    const file = e.target.files && e.target.files[0];
    if (!file || !selectedDeviceId) return;
    const destPath = joinPath(currentPath, file.name);
    setUploadTargetName(file.name);
    try {
      await uploadRemoteFile(selectedDeviceId, destPath, file);
      await browse(currentPath);
    } finally {
      setUploadTargetName("");
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">File Transfer</CardTitle>
        <CardDescription>Browse a connected device's filesystem, download files, or push a file to a destination path.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-1 gap-2 md:grid-cols-[220px_1fr_auto]">
          <select
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={selectedDeviceId}
            onChange={(e) => { setSelectedDeviceId(e.target.value); setEntries([]); setCurrentPath(""); setPathInput(""); }}
          >
            <option value="">Select a device</option>
            {(devices || []).map((d) => <option key={d.id} value={d.id}>{d.name || d.id}</option>)}
          </select>
          <Input
            placeholder="Path (blank = root)"
            value={pathInput}
            onChange={(e) => setPathInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") browse(pathInput); }}
          />
          <Button size="sm" onClick={() => browse(pathInput)} disabled={!selectedDeviceId || loading}>
            {loading ? "Loading..." : "Browse"}
          </Button>
        </div>

        {canCommand && selectedDeviceId && (
          <div className="flex items-center gap-2">
            <input ref={fileInputRef} type="file" className="hidden" onChange={handleUploadFile} />
            <Button size="sm" variant="outline" onClick={() => fileInputRef.current?.click()} disabled={!!uploadTargetName}>
              {uploadTargetName ? `Uploading ${uploadTargetName}...` : `Upload to ${currentPath || "root"}`}
            </Button>
          </div>
        )}

        {browseError && <p className="text-sm text-red-600">{browseError}</p>}

        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Modified</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map((entry) => {
                const fullPath = joinPath(currentPath, entry.name);
                return (
                  <TableRow key={entry.name}>
                    <TableCell>
                      {entry.isDir ? (
                        <button className="text-primary underline" onClick={() => browse(fullPath)}>{entry.name}/</button>
                      ) : (
                        entry.name
                      )}
                    </TableCell>
                    <TableCell>{entry.isDir ? "Folder" : "File"}</TableCell>
                    <TableCell>{entry.isDir ? "-" : formatBytes(entry.size)}</TableCell>
                    <TableCell>{entry.modTime ? new Date(entry.modTime).toLocaleString() : "-"}</TableCell>
                    <TableCell>
                      {!entry.isDir && (
                        <a href={downloadUrl(fullPath)} className="text-xs text-primary underline" download={entry.name}>Download</a>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
              {entries.length === 0 && !loading && (
                <TableRow>
                  <TableCell colSpan={5} className="text-muted-foreground">{selectedDeviceId ? "No entries loaded. Click Browse." : "Select a device to browse its filesystem."}</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
        <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
      </CardContent>
    </Card>
  );
}

function TerminalPage({ token, devices, canCommand, createTerminalSession, appStatus }) {
  const [selectedDeviceId, setSelectedDeviceId] = useState("");
  const [sessionId, setSessionId] = useState("");
  const [terminalStatus, setTerminalStatus] = useState("Disconnected");
  const terminalSocketRef = useRef(null);
  const terminalContainerRef = useRef(null);
  const xtermRef = useRef(null);
  const fitAddonRef = useRef(null);
  const terminalWrapperRef = useRef(null);

  function closeSocket() {
    if (terminalSocketRef.current) {
      terminalSocketRef.current.close();
      terminalSocketRef.current = null;
    }
    setSessionId("");
    setTerminalStatus("Disconnected");
  }

  useEffect(() => {
    const term = new XTerm({
      cursorBlink: true,
      fontFamily: "Consolas, 'Courier New', monospace",
      fontSize: 13,
      convertEol: true,
      theme: {
        background: "#000000",
        foreground: "#d1f7c4",
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    xtermRef.current = term;
    fitAddonRef.current = fitAddon;

    if (terminalContainerRef.current) {
      term.open(terminalContainerRef.current);
      fitAddon.fit();
      term.writeln("GoMeshCentral terminal ready.");
      term.writeln("Select device, then Start Session.");
      term.focus();
    }

    const disposeData = term.onData((chunk) => {
      const socket = terminalSocketRef.current;
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        return;
      }
      socket.send(JSON.stringify({ type: "terminal_data", data: chunk }));
    });

    const handleResize = () => {
      if (!fitAddonRef.current || !xtermRef.current) {
        return;
      }
      fitAddonRef.current.fit();
      const socket = terminalSocketRef.current;
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({
          type: "terminal_resize",
          cols: xtermRef.current.cols,
          rows: xtermRef.current.rows,
        }));
      }
    };
    window.addEventListener("resize", handleResize);

    const handleWrapperFocus = () => {
      if (xtermRef.current) {
        xtermRef.current.focus();
      }
    };
    const wrapper = terminalWrapperRef.current;
    if (wrapper) {
      wrapper.addEventListener("click", handleWrapperFocus);
      wrapper.addEventListener("focus", handleWrapperFocus);
    }

    return () => {
      window.removeEventListener("resize", handleResize);
      if (wrapper) {
        wrapper.removeEventListener("click", handleWrapperFocus);
        wrapper.removeEventListener("focus", handleWrapperFocus);
      }
      disposeData.dispose();
      if (terminalSocketRef.current) {
        terminalSocketRef.current.close();
        terminalSocketRef.current = null;
      }
      term.dispose();
    };
  }, []);

  async function startTerminal() {
    if (!canCommand) {
      setTerminalStatus("You do not have terminal permission");
      return;
    }
    if (!selectedDeviceId) {
      setTerminalStatus("Select a device first");
      return;
    }
    try {
      if (xtermRef.current) {
        xtermRef.current.clear();
      }
      setTerminalStatus("Opening session...");
      const cols = xtermRef.current ? xtermRef.current.cols : 120;
      const rows = xtermRef.current ? xtermRef.current.rows : 32;
      const session = await createTerminalSession(selectedDeviceId, cols, rows);
      const socket = new WebSocket(`${wsBase()}${session.wsPath}&token=${encodeURIComponent(token)}`);
      terminalSocketRef.current = socket;
      setSessionId(session.sessionId);

      socket.onopen = () => {
        setTerminalStatus(`Connected: ${selectedDeviceId}`);
        if (xtermRef.current) {
          xtermRef.current.writeln(`[session ${session.sessionId}] connected`);
        }
        if (fitAddonRef.current) {
          fitAddonRef.current.fit();
        }
        if (xtermRef.current) {
          xtermRef.current.focus();
          socket.send(JSON.stringify({
            type: "terminal_resize",
            cols: xtermRef.current.cols,
            rows: xtermRef.current.rows,
          }));
        }
      };
      socket.onmessage = (event) => {
        let payload = null;
        try {
          payload = JSON.parse(event.data);
        } catch {
          if (xtermRef.current) {
            xtermRef.current.write(event.data || "");
          }
          return;
        }
        if (payload.type === "terminal_data") {
          if (xtermRef.current) {
            xtermRef.current.write(payload.data || "");
          }
          return;
        }
        if (payload.type === "terminal_error") {
          if (xtermRef.current) {
            xtermRef.current.writeln(`[error] ${payload.error || "terminal error"}`);
          }
          return;
        }
        if (payload.type === "terminal_exit") {
          if (xtermRef.current) {
            xtermRef.current.writeln(`[session ended] exit code ${payload.exitCode ?? 0}`);
          }
          setTerminalStatus("Session ended");
          closeSocket();
        }
      };
      socket.onclose = () => {
        setTerminalStatus("Disconnected");
      };
      socket.onerror = () => {
        setTerminalStatus("Terminal websocket error");
      };
    } catch (err) {
      setTerminalStatus(err.message);
    }
  }

  function sendCtrlC() {
    if (!terminalSocketRef.current || terminalSocketRef.current.readyState !== WebSocket.OPEN) {
      setTerminalStatus("Session is not connected");
      return;
    }
    terminalSocketRef.current.send(JSON.stringify({ type: "terminal_data", data: "\u0003" }));
  }

  function endSession() {
    if (terminalSocketRef.current && terminalSocketRef.current.readyState === WebSocket.OPEN) {
      terminalSocketRef.current.send(JSON.stringify({ type: "terminal_close" }));
    }
    closeSocket();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Terminal</CardTitle>
        <CardDescription>Interactive shell relay session for connected Linux agents.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-1 gap-2 md:grid-cols-[1fr_auto_auto_auto]">
          <select
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={selectedDeviceId}
            onChange={(e) => setSelectedDeviceId(e.target.value)}
          >
            <option value="">Select a connected device</option>
            {devices.filter((d) => d.connected).map((d) => (
              <option key={d.id} value={d.id}>{d.id}{d.name ? ` (${d.name})` : ""}</option>
            ))}
          </select>
          <Button size="sm" onClick={startTerminal}>Start Session</Button>
          <Button size="sm" variant="outline" onClick={sendCtrlC}>Send Ctrl+C</Button>
          <Button size="sm" variant="outline" onClick={endSession}>End Session</Button>
        </div>

        <div
          ref={terminalWrapperRef}
          className="rounded-md border bg-black p-1"
          tabIndex={0}
        >
          <div ref={terminalContainerRef} className="h-96 w-full overflow-hidden" />
        </div>

        <p className="min-h-5 text-sm text-muted-foreground">{terminalStatus}{sessionId ? ` | Session: ${sessionId}` : ""}</p>
        <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
      </CardContent>
    </Card>
  );
}

function UsersPage({ users, canManageUsers, newUserForm, setNewUserForm, createUser }) {
  if (!canManageUsers) {
    return <PlaceholderFeature title="Users & Roles" description="Requires admin permissions." />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Users & Roles</CardTitle>
        <CardDescription>Admin management of user accounts and role assignments.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <Input
            placeholder="new username"
            value={newUserForm.username}
            onChange={(e) => setNewUserForm((v) => ({ ...v, username: e.target.value }))}
          />
          <Input
            type="email"
            placeholder="email address"
            value={newUserForm.email}
            onChange={(e) => setNewUserForm((v) => ({ ...v, email: e.target.value }))}
          />
          <Input
            type="password"
            placeholder="temporary password"
            value={newUserForm.password}
            onChange={(e) => setNewUserForm((v) => ({ ...v, password: e.target.value }))}
          />
          <select
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={newUserForm.role}
            onChange={(e) => setNewUserForm((v) => ({ ...v, role: e.target.value }))}
          >
            <option value="viewer">viewer</option>
            <option value="operator">operator</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <div className="flex items-center gap-3">
          <Button size="sm" onClick={createUser}>Create User</Button>
          <label className="flex items-center gap-2 cursor-pointer text-sm">
            <input
              type="checkbox"
              checked={newUserForm.sendEmail || false}
              onChange={(e) => setNewUserForm((v) => ({ ...v, sendEmail: e.target.checked }))}
              className="rounded border-gray-300"
            />
            <span>Send credentials via email</span>
          </label>
        </div>
        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => (
                <TableRow key={u.username}>
                  <TableCell>{u.username}</TableCell>
                  <TableCell className="text-gray-600">{u.email || "-"}</TableCell>
                  <TableCell>
                    <Badge variant={roleVariant(u.role)}>{u.role}</Badge>
                  </TableCell>
                  <TableCell>{u.createdAt ? new Date(u.createdAt).toLocaleString() : "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}

function EnrollmentPage({
  canManageUsers,
  createEnrollmentToken,
  enrollmentBootstrap,
  refreshEnrollmentBootstrap,
  latestEnrollment,
  devices,
  rotateDeviceId,
  setRotateDeviceId,
  rotateAgentCredential,
  latestRotatedCredential,
  auditEvents,
  refreshAuditEvents,
  appStatus,
}) {
  if (!canManageUsers) {
    return <PlaceholderFeature title="Enrollment" description="Requires admin permissions." />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Enrollment</CardTitle>
        <CardDescription>Create one-time tokens to enroll endpoint agents.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={() => createEnrollmentToken(60)}>Create 60-Min Token</Button>
          <Button variant="outline" size="sm" onClick={() => createEnrollmentToken(240)}>Create 4-Hour Token</Button>
          <Button variant="outline" size="sm" onClick={refreshEnrollmentBootstrap}>Refresh Endpoint</Button>
        </div>
        {enrollmentBootstrap && (
          <div className="rounded-md border p-3 text-sm space-y-2">
            <p className="font-medium text-foreground">Agent Endpoint Configuration</p>
            <p className="text-xs text-muted-foreground">Set once at startup using GMC_AGENT_PUBLIC_ADDR.</p>
            <p className="text-xs text-muted-foreground">Current endpoint</p>
            <p className="break-all font-mono text-xs">{enrollmentBootstrap.agentServer}</p>
          </div>
        )}
        {latestEnrollment && (
          <div className="rounded-md border p-3 text-sm space-y-3">
            <div>
              <p className="font-medium text-foreground">Latest Token</p>
              <p className="mt-1 break-all font-mono text-xs text-primary">{latestEnrollment.token}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Expires: {new Date(latestEnrollment.expiresAt).toLocaleString()} | Target: {latestEnrollment.agentServer}
              </p>
            </div>

            <div className="space-y-2 rounded-md border p-2 bg-muted/20">
              <p className="font-medium text-xs text-foreground">Windows Deployment</p>
              <div>
                <p className="text-xs text-muted-foreground">1-Line PowerShell Install (Downloads & Registers Service)</p>
                <p className="mt-1 select-all break-all font-mono text-xs rounded bg-muted p-1.5">{latestEnrollment.windowsOneLiner}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">MSI Silent Service Install (Standard Uninstaller in Windows Settings / ARP)</p>
                <p className="mt-1 select-all break-all font-mono text-xs rounded bg-muted p-1.5">{latestEnrollment.windowsMsiCommand}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">CLI Service Install (agent.exe)</p>
                <p className="mt-1 select-all break-all font-mono text-xs rounded bg-muted p-1.5">{latestEnrollment.windowsServiceInstallCommand}</p>
              </div>
            </div>

            <div className="space-y-2 rounded-md border p-2 bg-muted/20">
              <p className="font-medium text-xs text-foreground">Linux Deployment</p>
              <div>
                <p className="text-xs text-muted-foreground">1-Line Shell Install (curl | sudo sh)</p>
                <p className="mt-1 select-all break-all font-mono text-xs rounded bg-muted p-1.5">{latestEnrollment.linuxOneLiner}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">CLI Service Install (gomesh-agent -install-service systemd)</p>
                <p className="mt-1 select-all break-all font-mono text-xs rounded bg-muted p-1.5">{latestEnrollment.linuxInteractiveCommand}</p>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2 pt-1">
              <span className="text-xs text-muted-foreground">Direct Downloads:</span>
              <a href="/api/download/agent/windows-amd64" className="text-xs text-primary underline" download>agent.exe (Windows)</a>
              <a href="/api/download/agent/linux-amd64" className="text-xs text-primary underline" download>gomesh-agent (Linux)</a>
              <a href="/api/download/install.sh" className="text-xs text-primary underline" download>install.sh</a>
              <a href="/api/download/install.ps1" className="text-xs text-primary underline" download>install.ps1</a>
            </div>
          </div>
        )}
        <div className="rounded-md border p-3 text-sm space-y-2">
          <p className="font-medium text-foreground">Rotate Device Credential</p>
          <div className="grid grid-cols-1 gap-2 md:grid-cols-[1fr_auto]">
            <select
              className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              value={rotateDeviceId}
              onChange={(e) => setRotateDeviceId(e.target.value)}
            >
              <option value="">Select a device</option>
              {devices.map((d) => (
                <option key={d.id} value={d.id}>{d.id}{d.name ? ` (${d.name})` : ""}</option>
              ))}
            </select>
            <Button size="sm" onClick={rotateAgentCredential} disabled={!rotateDeviceId}>Rotate Credential</Button>
          </div>
          {latestRotatedCredential && (
            <div className="rounded-md border p-3 text-sm">
              <p className="text-muted-foreground">Latest rotated credential for {latestRotatedCredential.deviceId}</p>
              <p className="mt-1 break-all font-mono text-xs">{latestRotatedCredential.agentKey}</p>
              <p className="mt-1 text-xs text-muted-foreground">Store this key in the target agent state file.</p>
            </div>
          )}
        </div>
        <div className="rounded-md border p-3 text-sm space-y-2">
          <div className="flex items-center justify-between gap-2">
            <p className="font-medium text-foreground">Recent Audit Events</p>
            <Button variant="outline" size="sm" onClick={refreshAuditEvents}>Refresh Audit</Button>
          </div>
          <div className="space-y-2">
            {auditEvents.length === 0 ? (
              <p className="text-xs text-muted-foreground">No audit events yet.</p>
            ) : (
              auditEvents.map((event) => (
                <div key={event.id} className="rounded-md border px-3 py-2 text-xs">
                  <p className="font-medium">{event.action}</p>
                  <p className="text-muted-foreground">
                    Actor: {event.actor} | Target: {event.target} | {new Date(event.createdAt).toLocaleString()}
                  </p>
                  {event.details && <p className="text-muted-foreground">{event.details}</p>}
                </div>
              ))
            )}
          </div>
        </div>
        <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
      </CardContent>
    </Card>
  );
}

function SettingsPage({
  canManageUsers,
  agentEndpointSettings,
  agentPublicAddrInput,
  setAgentPublicAddrInput,
  refreshAgentEndpointSettings,
  saveAgentEndpointSettings,
  appStatus,
  applicationSettings,
  setApplicationSettings,
  saveApplicationSettings,
  onLogoUpload,
  saveAIProviderAndLoadModels,
}) {
  const [aiModels, setAIModels] = useState([]);
  const [aiModelsLoading, setAIModelsLoading] = useState(false);
  const [aiModelStatus, setAIModelStatus] = useState("");

  if (!canManageUsers) {
    return <PlaceholderFeature title="Settings" description="Requires admin permissions." />;
  }

  const themeOptions = [
    { value: "default", label: "Default" },
    { value: "midnight", label: "Midnight" },
    { value: "forest", label: "Forest" },
    { value: "brand", label: "Brand" },
  ];

  function selectAIProvider(provider) {
    setAIModels([]);
    setAIModelStatus("");
    setApplicationSettings((prev) => ({
      ...prev,
      ai: {
        ...prev.ai,
        provider,
        baseUrl: AI_PROVIDER_URLS[provider] || "",
        model: "",
      },
    }));
  }

  async function saveAIProvider() {
    setAIModelsLoading(true);
    setAIModelStatus("");
    try {
      const result = await saveAIProviderAndLoadModels();
      const models = result?.models || [];
      setAIModels(models);
      setApplicationSettings((prev) => ({
        ...prev,
        ai: { ...prev.ai, model: models.includes(prev.ai.model) ? prev.ai.model : (models[0] || "") },
      }));
      setAIModelStatus(models.length ? `${models.length} models loaded.` : "No models were returned by this provider.");
    } catch (err) {
      setAIModels([]);
      setAIModelStatus(err.message);
    } finally {
      setAIModelsLoading(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Global Settings</CardTitle>
        <CardDescription>Branding, domain, and mail routing in one place for the hosted console.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div className="rounded-md border p-3 text-sm space-y-3">
            <p className="font-medium text-foreground">Branding</p>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Theme</label>
              <select
                value={applicationSettings?.theme || "default"}
                onChange={(e) => setApplicationSettings((prev) => ({ ...prev, theme: e.target.value }))}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              >
                {themeOptions.map((option) => (
                  <option key={option.value} value={option.value}>{option.label}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Logo</label>
              <Input
                type="file"
                accept="image/*"
                onChange={onLogoUpload}
              />
              {applicationSettings?.logoDataUrl ? (
                <img src={applicationSettings.logoDataUrl} alt="Brand logo preview" className="h-16 w-auto rounded-md border bg-white p-2" />
              ) : null}
            </div>
          </div>

          <div className="rounded-md border p-3 text-sm space-y-3">
            <p className="font-medium text-foreground">Custom Domain & Mail Routing</p>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Custom Domain</label>
              <Input
                placeholder="portal.example.com"
                value={applicationSettings?.customDomain || ""}
                onChange={(e) => setApplicationSettings((prev) => ({ ...prev, customDomain: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Invoice Email Forwarding</label>
              <Input
                placeholder="invoices@yourmail.com"
                value={applicationSettings?.mailForwarding?.invoiceTo || ""}
                onChange={(e) => setApplicationSettings((prev) => ({
                  ...prev,
                  mailForwarding: { ...prev.mailForwarding, invoiceTo: e.target.value },
                }))}
              />
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Alert Email Forwarding</label>
              <Input
                placeholder="alerts@yourmail.com"
                value={applicationSettings?.mailForwarding?.alertTo || ""}
                onChange={(e) => setApplicationSettings((prev) => ({
                  ...prev,
                  mailForwarding: { ...prev.mailForwarding, alertTo: e.target.value },
                }))}
              />
            </div>
          </div>
        </div>

        <div className="rounded-md border p-3 text-sm space-y-3">
          <p className="font-medium text-foreground">SMTP / Forwarding Provider</p>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">SMTP Host</label>
              <Input
                placeholder="smtp.mailgun.org"
                value={applicationSettings?.mailForwarding?.smtpHost || ""}
                onChange={(e) => setApplicationSettings((prev) => ({
                  ...prev,
                  mailForwarding: { ...prev.mailForwarding, smtpHost: e.target.value },
                }))}
              />
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">SMTP Port</label>
              <Input
                type="number"
                placeholder="587"
                value={applicationSettings?.mailForwarding?.smtpPort || 587}
                onChange={(e) => setApplicationSettings((prev) => ({
                  ...prev,
                  mailForwarding: { ...prev.mailForwarding, smtpPort: Number(e.target.value || 587) },
                }))}
              />
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">SMTP Username</label>
              <Input
                placeholder="postmaster@example.com"
                value={applicationSettings?.mailForwarding?.smtpUsername || ""}
                onChange={(e) => setApplicationSettings((prev) => ({
                  ...prev,
                  mailForwarding: { ...prev.mailForwarding, smtpUsername: e.target.value },
                }))}
              />
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">From Address</label>
              <Input
                placeholder="noreply@example.com"
                value={applicationSettings?.mailForwarding?.fromAddress || ""}
                onChange={(e) => setApplicationSettings((prev) => ({
                  ...prev,
                  mailForwarding: { ...prev.mailForwarding, fromAddress: e.target.value },
                }))}
              />
            </div>
          </div>
          <div className="space-y-2">
            <label className="block text-xs text-muted-foreground">SMTP Password</label>
            <Input
              type="password"
              placeholder="••••••••"
              value={applicationSettings?.mailForwarding?.smtpPassword || ""}
              onChange={(e) => setApplicationSettings((prev) => ({
                ...prev,
                mailForwarding: { ...prev.mailForwarding, smtpPassword: e.target.value },
              }))}
            />
          </div>
        </div>

        <div className="rounded-md border p-3 text-sm space-y-3 bg-blue-50 dark:bg-blue-900/20">
          <div>
            <p className="font-medium text-foreground">📧 Email Notifications</p>
            <p className="text-xs text-muted-foreground mt-1">
              When SMTP is configured above, the system will automatically send email notifications for:
            </p>
            <ul className="text-xs text-muted-foreground mt-2 ml-4 list-disc space-y-1">
              <li><strong>Alerts:</strong> When device metrics breach thresholds → sent to "Alert Email Forwarding"</li>
              <li><strong>Tickets:</strong> When new support tickets are created → sent to "Invoice Email Forwarding"</li>
            </ul>
          </div>
          <div className="mt-2 p-2 bg-background rounded-md border">
            <p className="text-xs font-medium">Configuration Status:</p>
            <p className="text-xs text-muted-foreground mt-1">
              {applicationSettings?.mailForwarding?.smtpHost && applicationSettings?.mailForwarding?.fromAddress
                ? "✓ Email notifications are enabled"
                : "⚠ Email notifications are not yet configured. Fill in SMTP Host and From Address above to enable."}
            </p>
          </div>
        </div>

        <div className="rounded-md border p-3 text-sm space-y-3">
          <div>
            <p className="font-medium text-foreground">AI Provider</p>
            <p className="text-xs text-muted-foreground">Use OpenRouter, or connect Hermes through any OpenAI-compatible local endpoint.</p>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Provider</label>
              <select
                value={applicationSettings?.ai?.provider || "openrouter"}
                onChange={(e) => selectAIProvider(e.target.value)}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              >
                <option value="openrouter">OpenRouter</option>
                <option value="hermes">Hermes / Local</option>
                <option value="custom">Custom OpenAI-compatible</option>
              </select>
            </div>
            <div className="space-y-2">
              <label className="block text-xs text-muted-foreground">Model</label>
              <select
                value={applicationSettings?.ai?.model || ""}
                disabled={(!applicationSettings?.ai?.apiKey?.trim() && !applicationSettings?.ai?.apiKeyConfigured) || aiModelsLoading || aiModels.length === 0}
                onChange={(e) => setApplicationSettings((prev) => ({ ...prev, ai: { ...prev.ai, model: e.target.value } }))}
                className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
              >
                <option value="">{aiModelsLoading ? "Loading models..." : "Save provider settings to load models"}</option>
                {aiModels.map((model) => <option key={model} value={model}>{model}</option>)}
              </select>
            </div>
            <div className="space-y-2 md:col-span-2">
              <label className="block text-xs text-muted-foreground">Base URL</label>
              <Input
                placeholder="https://openrouter.ai/api/v1 or http://localhost:11434/v1"
                value={applicationSettings?.ai?.baseUrl || ""}
                disabled={applicationSettings?.ai?.provider !== "custom"}
                onChange={(e) => setApplicationSettings((prev) => ({ ...prev, ai: { ...prev.ai, baseUrl: e.target.value } }))}
              />
            </div>
            <div className="space-y-2 md:col-span-2">
              <label className="block text-xs text-muted-foreground">API Key</label>
              <Input
                type="password"
                placeholder={applicationSettings?.ai?.apiKeyConfigured ? "Key configured; leave blank to keep it" : "OpenRouter API key"}
                value={applicationSettings?.ai?.apiKey || ""}
                onChange={(e) => {
                  setAIModels([]);
                  setAIModelStatus("");
                  setApplicationSettings((prev) => ({ ...prev, ai: { ...prev.ai, apiKey: e.target.value, model: "" } }));
                }}
              />
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={(!applicationSettings?.ai?.apiKey?.trim() && !applicationSettings?.ai?.apiKeyConfigured) || aiModelsLoading || (applicationSettings?.ai?.provider === "custom" && !applicationSettings?.ai?.baseUrl?.trim())}
              onClick={saveAIProvider}
            >
              <RefreshCw className={cn("mr-2 h-4 w-4", aiModelsLoading && "animate-spin")} />
              Save
            </Button>
            <p className="text-xs text-muted-foreground">{aiModelStatus}</p>
          </div>
        </div>

        <div className="rounded-md border p-3 text-sm space-y-2">
          <p className="font-medium text-foreground">Agent Public Endpoint</p>
          <p className="text-xs text-muted-foreground">
            This host:port is injected into generated install commands and enrollment templates.
          </p>
          <Input
            placeholder="mesh.example.com:8080 or 192.168.1.50:8080"
            value={agentPublicAddrInput}
            onChange={(e) => setAgentPublicAddrInput(e.target.value)}
          />
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" onClick={saveApplicationSettings}>Save Settings</Button>
            <Button size="sm" variant="outline" onClick={saveAgentEndpointSettings}>Save Endpoint</Button>
            <Button variant="outline" size="sm" onClick={refreshAgentEndpointSettings}>Refresh</Button>
            <Button variant="outline" size="sm" onClick={() => setAgentPublicAddrInput("")}>Clear</Button>
          </div>
          <p className="text-xs text-muted-foreground">Restart required: No (applies immediately)</p>
          <p className="text-xs text-muted-foreground">
            Effective endpoint: {agentEndpointSettings?.effectiveAgentServer || "-"}
          </p>
          <p className="text-xs text-muted-foreground">
            Configured endpoint: {agentEndpointSettings?.agentPublicAddr || "(using request host fallback)"}
          </p>
        </div>
        <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
      </CardContent>
    </Card>
  );
}

function AdminBrandingPage({ token, appStatus }) {
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
      const response = await fetch(apiUrl("/api/admin/branding"), {
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

  const handleImageUpload = (field, file) => {
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (e) => {
      const base64 = e.target.result;
      setBranding((prev) => ({
        ...prev,
        [field]: base64,
      }));
      setMessage(`${field === "logo" ? "Logo" : "Icon"} uploaded`);
      setTimeout(() => setMessage(""), 3000);
    };
    reader.readAsDataURL(file);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const response = await fetch(apiUrl("/api/admin/branding"), {
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
    return (
      <Card>
        <CardContent className="p-8">Loading branding settings...</CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold mb-2">Company Branding</h2>
        <p className="text-gray-600">
          Customize your company's name and contact information that appears throughout the system and on invoices.
        </p>
      </div>

      {message && (
        <Card className="bg-blue-50 border-blue-200">
          <CardContent className="p-4 text-sm text-blue-800">{message}</CardContent>
        </Card>
      )}

      <Card>
        <CardContent className="p-6 space-y-6">
          {/* Company Name */}
          <div>
            <label className="block text-sm font-medium mb-2">Company Name</label>
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
            <label className="block text-sm font-medium mb-2">Phone Number</label>
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
            <p className="text-xs text-gray-500 mt-1">Your company's website URL</p>
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
            <p className="text-xs text-gray-500 mt-1">Support or contact email address</p>
          </div>

          {/* Logo Upload */}
          <div>
            <label className="block text-sm font-medium mb-2">Company Logo</label>
            <div className="border-2 border-dashed border-gray-300 rounded-lg p-4">
              {branding.logo && (
                <div className="mb-4">
                  <img src={branding.logo} alt="Company Logo" className="h-24 w-auto" />
                </div>
              )}
              <input
                type="file"
                accept="image/*"
                onChange={(e) => handleImageUpload("logo", e.target.files?.[0])}
                className="block w-full text-sm text-gray-600 file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
              />
              <p className="text-xs text-gray-500 mt-2">PNG or JPG, max 5MB. Used in invoices and app header.</p>
            </div>
          </div>

          {/* Icon Upload */}
          <div>
            <label className="block text-sm font-medium mb-2">App Icon</label>
            <div className="border-2 border-dashed border-gray-300 rounded-lg p-4">
              {branding.icon && (
                <div className="mb-4">
                  <img src={branding.icon} alt="App Icon" className="h-12 w-12 rounded-lg" />
                </div>
              )}
              <input
                type="file"
                accept="image/*"
                onChange={(e) => handleImageUpload("icon", e.target.files?.[0])}
                className="block w-full text-sm text-gray-600 file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
              />
              <p className="text-xs text-gray-500 mt-2">PNG or JPG, max 5MB. Square icon displayed in the app header.</p>
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
            <Button onClick={loadBranding} disabled={saving} variant="outline">
              Cancel
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function AIAssistantPage({ activeClientId, clients, askAI, executeAIAction, appStatus }) {
  const [message, setMessage] = useState("");
  const [conversation, setConversation] = useState([]);
  const [busy, setBusy] = useState(false);
  const activeClient = (clients || []).find((client) => client.id === activeClientId);

  async function submit() {
    const prompt = message.trim();
    if (!prompt || busy) return;
    setMessage("");
    setBusy(true);
    setConversation((items) => [...items, { role: "user", reply: prompt, actions: [] }]);
    try {
      const response = await askAI(prompt);
      setConversation((items) => [...items, { role: "assistant", reply: response?.reply || "No response.", actions: response?.actions || [] }]);
    } catch (err) {
      setConversation((items) => [...items, { role: "assistant", reply: err.message, actions: [] }]);
    } finally {
      setBusy(false);
    }
  }

  async function approve(action, conversationIndex, actionIndex) {
    try {
      await executeAIAction(action);
      setConversation((items) => items.map((item, index) => index === conversationIndex ? {
        ...item,
        actions: item.actions.map((candidate, candidateIndex) => candidateIndex === actionIndex ? { ...candidate, executed: true } : candidate),
      } : item));
    } catch (err) {
      setConversation((items) => [...items, { role: "assistant", reply: `Action failed: ${err.message}`, actions: [] }]);
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg"><Bot className="h-5 w-5 text-primary" /> AI Assistant</CardTitle>
          <CardDescription>Context: {activeClient ? activeClient.name : "all clients"}. Proposed changes require approval.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="min-h-[320px] space-y-3 rounded-md border bg-background p-3">
            {conversation.length === 0 && <p className="text-sm text-muted-foreground">Ask about devices, alerts, clients, contracts, tickets, invoices, or request an operational task.</p>}
            {conversation.map((item, index) => (
              <div key={index} className={cn("max-w-3xl rounded-md border p-3 text-sm", item.role === "user" ? "ml-auto bg-primary text-primary-foreground" : "bg-card")}>
                <p className="whitespace-pre-wrap">{item.reply}</p>
                {(item.actions || []).map((action, actionIndex) => (
                  <div key={`${action.type}-${actionIndex}`} className="mt-3 flex items-center justify-between gap-3 rounded-md border bg-background p-2 text-foreground">
                    <div>
                      <p className="text-xs font-semibold">{action.type.replaceAll("_", " ")}</p>
                      <p className="text-xs text-muted-foreground">{action.description}</p>
                    </div>
                    <Button size="sm" disabled={action.executed} onClick={() => approve(action, index, actionIndex)}>{action.executed ? "Completed" : "Approve"}</Button>
                  </div>
                ))}
              </div>
            ))}
            {busy && <p className="text-sm text-muted-foreground">Thinking…</p>}
          </div>
          <div className="flex gap-2">
            <Input value={message} onChange={(e) => setMessage(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") submit(); }} placeholder="What needs attention, or what should I do?" />
            <Button onClick={submit} disabled={busy || !message.trim()}>Send</Button>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>
    </div>
  );
}

function WorkQueuePage({ workQueue, refreshWorkQueue, appStatus }) {
  const milestone = workQueue?.currentMilestone || {};
  const sections = workQueue?.sections || [];

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Execution Work Queue</CardTitle>
          <CardDescription>Live status from docs/EXECUTION_TRACKER.md</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={refreshWorkQueue}>Refresh Tracker</Button>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">Phase</p>
              <p className="mt-1 text-lg font-semibold">{milestone.Phase || "-"}</p>
            </div>
            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">Milestone</p>
              <p className="mt-1 text-lg font-semibold">{milestone.Name || "-"}</p>
            </div>
            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">Status</p>
              <p className="mt-1 text-lg font-semibold">{milestone.Status || "-"}</p>
            </div>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>

      {sections.map((section) => (
        <Card key={section.title}>
          <CardHeader>
            <CardTitle className="text-base">{section.title}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {section.items.map((item, idx) => (
                <div key={`${section.title}-${idx}`} className="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
                  <span>{item.text}</span>
                  {typeof item.done === "boolean" ? (
                    <Badge variant={item.done ? "default" : "outline"}>{item.done ? "Done" : "Pending"}</Badge>
                  ) : (
                    <Badge variant="outline">Note</Badge>
                  )}
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

function EventsPage({ canManageUsers, auditEvents, refreshAuditEvents, appStatus }) {
  const [actionFilter, setActionFilter] = useState("all");
  const [query, setQuery] = useState("");

  const availableActions = useMemo(() => {
    const set = new Set();
    for (const event of auditEvents) {
      if (event.action) set.add(event.action);
    }
    return ["all", ...Array.from(set)];
  }, [auditEvents]);

  const filteredEvents = useMemo(() => {
    const q = query.trim().toLowerCase();
    return auditEvents.filter((event) => {
      if (actionFilter !== "all" && event.action !== actionFilter) return false;
      if (!q) return true;
      const haystack = `${event.action} ${event.actor} ${event.target} ${event.details || ""}`.toLowerCase();
      return haystack.includes(q);
    });
  }, [auditEvents, actionFilter, query]);

  if (!canManageUsers) {
    return <PlaceholderFeature title="Events" description="Requires admin permissions." />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Events</CardTitle>
        <CardDescription>Recent audit timeline for enrollment, credential, and admin actions.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-1 gap-2 md:grid-cols-[200px_1fr_auto]">
          <select
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            value={actionFilter}
            onChange={(e) => setActionFilter(e.target.value)}
          >
            {availableActions.map((action) => (
              <option key={action} value={action}>{action}</option>
            ))}
          </select>
          <Input
            placeholder="Search actor, target, details"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <Button variant="outline" size="sm" onClick={refreshAuditEvents}>Refresh</Button>
        </div>

        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Target</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredEvents.map((event) => (
                <TableRow key={event.id}>
                  <TableCell>{event.createdAt ? new Date(event.createdAt).toLocaleString() : "-"}</TableCell>
                  <TableCell>{event.action || "-"}</TableCell>
                  <TableCell>{event.actor || "-"}</TableCell>
                  <TableCell>{event.target || "-"}</TableCell>
                  <TableCell className="max-w-[360px] truncate">{event.details || "-"}</TableCell>
                </TableRow>
              ))}
              {filteredEvents.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="text-muted-foreground">No matching events.</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
        <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
      </CardContent>
    </Card>
  );
}

function MetricLineChart({ title, samples, valueKey, color, ySuffix = "", decimals = 1 }) {
  const width = 640;
  const height = 180;
  const pad = 24;

  const values = samples.map((s) => Number(s?.[valueKey] || 0));
  const maxValue = Math.max(1, ...values);
  const points = samples.map((sample, index) => {
    const x = pad + (index / Math.max(1, samples.length - 1)) * (width - pad * 2);
    const y = height - pad - ((Number(sample?.[valueKey] || 0) / maxValue) * (height - pad * 2));
    return `${x},${y}`;
  }).join(" ");

  const latest = values.length > 0 ? values[values.length - 1] : null;

  return (
    <div className="rounded-md border p-3 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">{title}</p>
        <p className="text-xs font-medium">{latest === null ? "-" : `${latest.toFixed(decimals)}${ySuffix}`}</p>
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-44 rounded bg-muted/20">
        <line x1={pad} y1={pad} x2={pad} y2={height - pad} stroke="currentColor" strokeOpacity="0.25" />
        <line x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} stroke="currentColor" strokeOpacity="0.25" />
        {samples.length > 1 ? (
          <polyline fill="none" stroke={color} strokeWidth="2.5" points={points} />
        ) : (
          <text x={width / 2} y={height / 2} textAnchor="middle" className="fill-current text-xs opacity-60">Need more samples</text>
        )}
      </svg>
    </div>
  );
}

function ReportsPage({
  reports,
  refreshReports,
  loadReportMetrics,
  canManageUsers,
  createEnrollmentToken,
  latestEnrollment,
  enrollmentBootstrap,
  appStatus,
}) {
  const [selectedDeviceId, setSelectedDeviceId] = useState("");
  const [windowMinutes, setWindowMinutes] = useState(180);
  const [metricSamples, setMetricSamples] = useState([]);

  function formatBytes(value) {
    if (!value) return "-";
    const units = ["B", "KB", "MB", "GB", "TB"];
    let size = value;
    let unit = 0;
    while (size >= 1024 && unit < units.length - 1) {
      size /= 1024;
      unit += 1;
    }
    return `${size.toFixed(size >= 10 ? 0 : 1)} ${units[unit]}`;
  }

  const sortedReports = useMemo(() => {
    return [...reports].sort((left, right) => {
      const leftReportedAt = left?.report?.reportedAt ? new Date(left.report.reportedAt).getTime() : 0;
      const rightReportedAt = right?.report?.reportedAt ? new Date(right.report.reportedAt).getTime() : 0;
      const leftScore =
        (left?.device?.connected ? 100 : 0) +
        (left?.enrolled ? 50 : 0) +
        (leftReportedAt > 0 ? 25 : 0);
      const rightScore =
        (right?.device?.connected ? 100 : 0) +
        (right?.enrolled ? 50 : 0) +
        (rightReportedAt > 0 ? 25 : 0);

      if (rightScore !== leftScore) {
        return rightScore - leftScore;
      }
      if (rightReportedAt !== leftReportedAt) {
        return rightReportedAt - leftReportedAt;
      }
      return (left?.device?.id || "").localeCompare(right?.device?.id || "");
    });
  }, [reports]);

  useEffect(() => {
    if (selectedDeviceId && sortedReports.some((entry) => entry?.device?.id === selectedDeviceId)) {
      return;
    }
    const preferred = sortedReports.find((entry) => entry?.report?.reportedAt) || sortedReports[0] || null;
    setSelectedDeviceId(preferred?.device?.id || "");
  }, [sortedReports, selectedDeviceId]);

  const selected = useMemo(() => {
    if (!selectedDeviceId) return null;
    return sortedReports.find((r) => r?.device?.id === selectedDeviceId) || null;
  }, [sortedReports, selectedDeviceId]);

  const unenrolledCount = reports.filter((r) => !r.enrolled).length;

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (!selected?.device?.id) {
        setMetricSamples([]);
        return;
      }
      const data = await loadReportMetrics(selected.device.id, windowMinutes);
      if (!cancelled) {
        setMetricSamples(Array.isArray(data) ? data : []);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [selected?.device?.id, windowMinutes]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Agent Reports</CardTitle>
          <CardDescription>Live machine inventory and runtime details from connected endpoints.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" onClick={refreshReports}>Refresh Reports</Button>
            <select
              className="flex h-9 rounded-md border border-input bg-background px-2 py-1 text-xs"
              value={windowMinutes}
              onChange={(e) => setWindowMinutes(Number(e.target.value))}
            >
              <option value={60}>Last 1h</option>
              <option value={180}>Last 3h</option>
              <option value={720}>Last 12h</option>
              <option value={1440}>Last 24h</option>
            </select>
            <Badge variant={unenrolledCount === 0 ? "default" : "outline"}>
              {unenrolledCount === 0 ? "All Enrolled" : `${unenrolledCount} Unenrolled`}
            </Badge>
          </div>
          <div className="overflow-x-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Device</TableHead>
                  <TableHead>Enrollment</TableHead>
                  <TableHead>Connected</TableHead>
                  <TableHead>OS/Arch</TableHead>
                  <TableHead>CPU %</TableHead>
                  <TableHead>Memory %</TableHead>
                  <TableHead>Last Report</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedReports.map((entry) => (
                  <TableRow
                    key={entry.device.id}
                    className={cn("cursor-pointer", selectedDeviceId === entry.device.id && "bg-muted/50")}
                    onClick={() => setSelectedDeviceId(entry.device.id)}
                  >
                    <TableCell>{entry.device.name || entry.device.id}</TableCell>
                    <TableCell>
                      <Badge variant={entry.enrolled ? "default" : "outline"}>{entry.enrolled ? "Enrolled" : "Unenrolled"}</Badge>
                    </TableCell>
                    <TableCell>{entry.device.connected ? "yes" : "no"}</TableCell>
                    <TableCell>{entry.report?.os ? `${entry.report.os}/${entry.report.arch}` : "-"}</TableCell>
                    <TableCell>{entry.report?.reportedAt ? `${entry.report.cpuUsagePercent.toFixed(1)}%` : "-"}</TableCell>
                    <TableCell>{entry.report?.reportedAt ? `${entry.report.memoryUsagePercent.toFixed(1)}%` : "-"}</TableCell>
                    <TableCell>{entry.report?.reportedAt ? new Date(entry.report.reportedAt).toLocaleString() : "No report yet"}</TableCell>
                  </TableRow>
                ))}
                {reports.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={7} className="text-muted-foreground">No report data available yet.</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{appStatus}</p>
        </CardContent>
      </Card>

      {selected && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Report Details: {selected.device.name || selected.device.id}</CardTitle>
            <CardDescription>Device ID: {selected.device.id}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Hostname</p><p className="mt-1">{selected.report.hostname || "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Username</p><p className="mt-1">{selected.report.username || "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">OS / Arch</p><p className="mt-1">{selected.report.os ? `${selected.report.os}/${selected.report.arch}` : "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">CPU Cores</p><p className="mt-1">{selected.report.cpuCount || "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">CPU Usage</p><p className="mt-1">{typeof selected.report.cpuUsagePercent === "number" ? `${selected.report.cpuUsagePercent.toFixed(1)}%` : "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Memory Usage</p><p className="mt-1">{typeof selected.report.memoryUsagePercent === "number" ? `${selected.report.memoryUsagePercent.toFixed(1)}%` : "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Memory Used / Total</p><p className="mt-1">{selected.report.memoryUsedBytes ? `${formatBytes(selected.report.memoryUsedBytes)} / ${formatBytes(selected.report.memoryTotalBytes)}` : "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Process ID</p><p className="mt-1">{selected.report.processId || "-"}</p></div>
              <div className="rounded-md border p-3"><p className="text-xs text-muted-foreground">Agent Uptime</p><p className="mt-1">{selected.report.agentUptimeSeconds ? `${selected.report.agentUptimeSeconds}s` : "-"}</p></div>
            </div>

            {!selected.report.reportedAt && (
              <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
                This device has not submitted a report yet. It may be a stale record from earlier testing or an endpoint that has never sent inventory data.
              </div>
            )}

            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">Local IPv4 Addresses</p>
              <p className="mt-1 break-all">{selected.report.localIps?.length ? selected.report.localIps.join(", ") : "-"}</p>
            </div>
            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">Executable Path</p>
              <p className="mt-1 break-all">{selected.report.executablePath || "-"}</p>
            </div>
            <div className="rounded-md border p-3">
              <p className="text-xs text-muted-foreground">Working Directory</p>
              <p className="mt-1 break-all">{selected.report.workingDir || "-"}</p>
            </div>

            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
              <MetricLineChart title="CPU Usage Trend" samples={metricSamples} valueKey="cpuUsagePercent" color="#22d3ee" ySuffix="%" />
              <MetricLineChart title="Memory Usage Trend" samples={metricSamples} valueKey="memoryUsagePercent" color="#f97316" ySuffix="%" />
            </div>

            {!selected.enrolled && canManageUsers && (
              <div className="rounded-md border border-dashed p-3 space-y-2">
                <p className="font-medium">Unenrolled Agent Recovery</p>
                <p className="text-muted-foreground">Create a one-time token, then restart this agent with the token to enroll.</p>
                <div className="flex flex-wrap items-center gap-2">
                  <Button size="sm" onClick={() => createEnrollmentToken(60)}>Create Recovery Token (60m)</Button>
                </div>
                {latestEnrollment && (
                  <div className="rounded-md border p-3 bg-muted/30">
                    <p className="text-xs text-muted-foreground">Enrollment command for this device</p>
                    <p className="mt-1 break-all font-mono text-xs">
                      {latestEnrollment.windowsInteractiveCommand || `go run ./cmd/agent -server ${enrollmentBootstrap?.agentServer || window.location.host} -state data/agent-state.json -enroll-token ${latestEnrollment.token}`}
                    </p>
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function LoginView({
  loginForm,
  setLoginForm,
  handleLogin,
  loginStatus,
  showSignup,
  setShowSignup,
  signupForm,
  setSignupForm,
  handleRegisterOrg,
  signupStatus,
}) {
  if (showSignup) {
    return (
      <Card className="mx-auto mt-16 w-full max-w-md">
        <CardHeader>
          <div className="mb-2 inline-flex w-fit items-center gap-2 rounded-full border px-3 py-1 text-xs text-muted-foreground">
            <Monitor className="h-3.5 w-3.5" /> New Organization
          </div>
          <CardTitle>Create Your Organization</CardTitle>
          <CardDescription>Set up an isolated workspace for your team</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <Input
            placeholder="organization name"
            value={signupForm.orgName}
            onChange={(e) => setSignupForm((v) => ({ ...v, orgName: e.target.value }))}
          />
          <Input
            placeholder="admin username"
            value={signupForm.adminUsername}
            onChange={(e) => setSignupForm((v) => ({ ...v, adminUsername: e.target.value }))}
          />
          <Input
            type="password"
            placeholder="admin password"
            value={signupForm.adminPassword}
            onChange={(e) => setSignupForm((v) => ({ ...v, adminPassword: e.target.value }))}
          />
          <div className="flex items-center gap-2 pt-1">
            <Button onClick={handleRegisterOrg}>Create Organization</Button>
            <Button variant="outline" onClick={() => setShowSignup(false)}>
              Back to Login
            </Button>
          </div>
          <p className="min-h-5 text-sm text-muted-foreground">{signupStatus}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="mx-auto mt-16 w-full max-w-md">
      <CardHeader>
        <div className="mb-2 inline-flex w-fit items-center gap-2 rounded-full border px-3 py-1 text-xs text-muted-foreground">
          <Monitor className="h-3.5 w-3.5" /> Secure Access
        </div>
        <CardTitle>GoMeshCentral Console</CardTitle>
        <CardDescription>Sign in to manage endpoints</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Input
          placeholder="username"
          value={loginForm.username}
          onChange={(e) => setLoginForm((v) => ({ ...v, username: e.target.value }))}
        />
        <Input
          type="password"
          placeholder="password"
          value={loginForm.password}
          onChange={(e) => setLoginForm((v) => ({ ...v, password: e.target.value }))}
        />
        <div className="flex items-center gap-2 pt-1">
          <Button onClick={handleLogin}>Login</Button>
          <Button variant="outline" onClick={() => setShowSignup(true)}>
            Create Organization
          </Button>
        </div>
        <p className="min-h-5 text-sm text-muted-foreground">{loginStatus}</p>
      </CardContent>
    </Card>
  );
}

function AppShell({
	token,
  session,
  logout,
  refreshDevices,
  refreshUsers,
  devices,
  users,
  appStatus,
  canCommand,
  canManageUsers,
  sendPing,
  newUserForm,
  setNewUserForm,
  createUser,
  createEnrollmentToken,
  enrollmentBootstrap,
  refreshEnrollmentBootstrap,
  latestEnrollment,
  rotateDeviceId,
  setRotateDeviceId,
  rotateAgentCredential,
  latestRotatedCredential,
  auditEvents,
  refreshAuditEvents,
  reports,
  refreshReports,
  loadReportMetrics,
  deleteDevice,
  workQueue,
  refreshWorkQueue,
  createTerminalSession,
  agentEndpointSettings,
  agentPublicAddrInput,
  setAgentPublicAddrInput,
  refreshAgentEndpointSettings,
  saveAgentEndpointSettings,
  canManagePSA,
  clients,
  createClient,
  deleteClient,
  assignDeviceClient,
  contracts,
  createContract,
  generateContractInvoice,
  tickets,
  createTicket,
  updateTicket,
  invoices,
  createInvoice,
  updateInvoice,
  timeEntries,
  createTimeEntry,
  deleteTimeEntry,
  groups,
  createDeviceGroup,
  deleteDeviceGroup,
  assignDeviceGroup,
  scripts,
  createScript,
  deleteScript,
  runScript,
  alertRules,
  alerts,
  createAlertRule,
  deleteAlertRule,
  acknowledgeAlert,
  resolveAlert,
  listRemoteFiles,
  uploadRemoteFile,
  applicationSettings,
  setApplicationSettings,
  saveApplicationSettings,
  onLogoUpload,
  saveAIProviderAndLoadModels,
  askAI,
  executeAIAction,
  activeClientId,
  setActiveClientId,
  activeRouteRef,
  refreshRouteData,
  branding,
  refreshBranding,
}) {
  const allFeatures = flattenFeatures();
  const openAlertCount = useMemo(() => (alerts || []).filter((a) => a.status === "open").length, [alerts]);
  const scopedClients = activeClientId ? (clients || []).filter((client) => client.id === activeClientId) : clients;
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [adminMenuExpanded, setAdminMenuExpanded] = useState(false);

  return (
    <>
      <LiveRouteRefresh activeRouteRef={activeRouteRef} refreshRouteData={refreshRouteData} />
      {sidebarCollapsed && (
        <div
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onClick={() => setSidebarCollapsed(false)}
        />
      )}
      <div className={`grid min-h-screen grid-cols-1 bg-gray-50 transition-all duration-300 ${sidebarCollapsed ? "lg:grid-cols-[80px_1fr]" : "lg:grid-cols-[256px_1fr]"}`}>
        <aside className={`sticky top-0 h-screen flex flex-col border-b border-gray-900 bg-[#1a1a1a] text-gray-200 lg:border-b-0 lg:border-r lg:border-gray-900 transition-all duration-300 ${sidebarCollapsed ? "w-20 items-center p-2" : "w-64 p-4"}`}>
          {!sidebarCollapsed && (
            <div className="mb-4 flex flex-col gap-3 px-2">
              <div className="flex items-center justify-between h-8">
                {/* Logo/Icon Display */}
                <div className="flex items-center gap-2 flex-shrink-0">
                  {branding?.logo ? (
                    <img src={branding.logo} alt="Company Logo" className="h-8 w-auto max-w-[120px] object-contain rounded-md border border-gray-700 bg-gray-800 p-0.5 flex-shrink-0" />
                  ) : branding?.icon ? (
                    <img src={branding.icon} alt="Company Icon" className="h-8 w-8 object-contain rounded-md border border-gray-700 bg-gray-800 p-0.5 flex-shrink-0" />
                  ) : (
                    <div className="flex h-8 w-8 items-center justify-center rounded-md bg-gradient-to-br from-cyan-400 to-blue-500 text-sm font-bold text-white flex-shrink-0">G</div>
                  )}
                </div>
                {/* Collapse Button */}
                <button
                  onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                  className="hidden lg:flex items-center justify-center h-8 w-8 rounded-md hover:bg-gray-700 transition-colors text-gray-400 hover:text-gray-200 flex-shrink-0 -mr-2"
                  title="Collapse sidebar"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              {!branding?.logo && (
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-white truncate">{branding?.companyName || "GoMeshCentral"}</p>
                  {applicationSettings?.customDomain && (
                    <p className="text-[10px] text-gray-400 truncate">{applicationSettings.customDomain}</p>
                  )}
                </div>
              )}
            </div>
          )}
          
          {sidebarCollapsed && (
            <div className="mb-4 flex flex-col items-center">
              <button
                onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                className="hidden lg:flex items-center justify-center w-10 h-10 rounded-md hover:opacity-90 transition-opacity flex-shrink-0"
                style={{
                  backgroundImage: branding?.icon && branding?.logo ? `url(${branding.icon})` : branding?.logo ? `url(${branding.logo})` : undefined,
                  backgroundSize: "contain",
                  backgroundPosition: "center",
                  backgroundRepeat: "no-repeat",
                  backgroundColor: !(branding?.icon && branding?.logo) && !branding?.logo ? "rgb(34 197 94)" : "transparent",
                }}
                title="Expand sidebar"
              >
                {!(branding?.icon || branding?.logo) && (
                  <span className="text-sm font-bold text-white">G</span>
                )}
              </button>
            </div>
          )}

          {/* Main Nav */}
          {!adminMenuExpanded ? (
            <>
              <nav className={`overflow-hidden max-h-[calc(100vh-300px)] ${sidebarCollapsed ? "flex flex-col items-center space-y-3 w-full" : "space-y-3"}`}>
                {FEATURE_SECTIONS.map((section) => {
                  const SectionIcon = SECTION_ICONS[section.title] || LayoutDashboard;
                  return (
                    <div key={section.title} className={sidebarCollapsed ? "flex flex-col items-center space-y-3 w-full" : ""}>
                      {!sidebarCollapsed && (
                        <p className="mb-2 flex items-center gap-2 px-2 text-xs font-semibold uppercase tracking-wider text-gray-500">
                          <SectionIcon className="h-3 w-3 flex-shrink-0" /> 
                          <span>{section.title}</span>
                        </p>
                      )}
                      <div className={sidebarCollapsed ? "flex flex-col items-center space-y-3 w-full" : "space-y-1"}>
                        {section.items.map((item) => {
                          const itemIcon = getNavigationIcon(item.path);
                          return (
                            <NavLink
                              key={item.path}
                              to={item.path}
                              className={({ isActive }) => cn(
                                "flex items-center rounded-lg transition-all duration-150 group relative",
                                sidebarCollapsed 
                                  ? "justify-center p-2 hover:bg-gray-800" 
                                  : "gap-3 px-3 py-2 justify-start text-sm",
                                isActive
                                  ? "bg-gray-800 text-white"
                                  : "text-gray-400 hover:text-gray-200"
                              )}
                              title={sidebarCollapsed ? item.label : ""}
                            >
                              {itemIcon}
                              {!sidebarCollapsed && <span>{item.label}</span>}
                              {sidebarCollapsed && (
                                <div className="absolute left-full ml-3 hidden group-hover:block bg-gray-900 text-white text-xs py-1.5 px-2.5 rounded whitespace-nowrap z-50 border border-gray-700">
                                  {item.label}
                                </div>
                              )}
                            </NavLink>
                          );
                        })}
                      </div>
                    </div>
                  );
                })}
              </nav>

              {/* Bottom Navigation */}
              <div className={`border-t border-gray-700 pt-3 mt-auto flex flex-col ${sidebarCollapsed ? "items-center space-y-3" : "space-y-3"}`}>
                {/* App Center */}
                {!sidebarCollapsed ? (
                  <div>
                    <p className="mb-2 flex items-center gap-2 px-2 text-xs font-semibold uppercase tracking-wider text-gray-500">
                      <Zap className="h-3 w-3 flex-shrink-0" /> 
                      <span>App Center</span>
                    </p>
                    <div className="space-y-1">
                      {APP_CENTER_ITEMS.map((item) => {
                        const itemIcon = getNavigationIcon(item.path);
                        return (
                          <NavLink
                            key={item.path}
                            to={item.path}
                            className={({ isActive }) => cn(
                              "flex items-center rounded-lg transition-all duration-150 group relative gap-3 px-3 py-2 justify-start text-sm",
                              isActive
                                ? "bg-gray-800 text-white"
                                : "text-gray-400 hover:text-gray-200"
                            )}
                          >
                            {itemIcon}
                            <span>{item.label}</span>
                          </NavLink>
                        );
                      })}
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-col items-center space-y-3 w-full">
                    {APP_CENTER_ITEMS.map((item) => {
                      const itemIcon = getNavigationIcon(item.path);
                      return (
                        <NavLink
                          key={item.path}
                          to={item.path}
                          className={({ isActive }) => cn(
                            "flex items-center rounded-lg transition-all duration-150 group relative justify-center p-2 hover:bg-gray-800",
                            isActive
                              ? "bg-gray-800 text-white"
                              : "text-gray-400 hover:text-gray-200"
                          )}
                          title={item.label}
                        >
                          {itemIcon}
                          <div className="absolute left-full ml-3 hidden group-hover:block bg-gray-900 text-white text-xs py-1.5 px-2.5 rounded whitespace-nowrap z-50 border border-gray-700">
                            {item.label}
                          </div>
                        </NavLink>
                      );
                    })}
                  </div>
                )}

                {/* Admin Button */}
                <button
                  onClick={() => setAdminMenuExpanded(true)}
                  className={cn(
                    "w-full flex items-center rounded-lg transition-all duration-150 group relative",
                    sidebarCollapsed 
                      ? "justify-center p-2 hover:bg-gray-800" 
                      : "gap-3 px-3 py-2 justify-start text-sm",
                    "text-gray-400 hover:text-gray-200"
                  )}
                  title={sidebarCollapsed ? "Admin" : ""}
                >
                  <Settings className="h-5 w-5" />
                  {!sidebarCollapsed && <span>Admin</span>}
                  {sidebarCollapsed && (
                    <div className="absolute left-full ml-3 hidden group-hover:block bg-gray-900 text-white text-xs py-1.5 px-2.5 rounded whitespace-nowrap z-50 border border-gray-700">
                      Admin
                    </div>
                  )}
                </button>
              </div>
            </>
          ) : (
            <>
              {/* Admin Menu View */}
              <div className="flex items-center gap-2 mb-4 px-2">
                <button
                  onClick={() => setAdminMenuExpanded(false)}
                  className="flex items-center justify-center p-1.5 hover:bg-gray-700 rounded transition-colors text-gray-400 hover:text-gray-200"
                  title="Back to main menu"
                >
                  <ChevronDown className="h-5 w-5 rotate-90" />
                </button>
                {!sidebarCollapsed && <span className="text-xs font-semibold uppercase tracking-wider text-gray-500">Admin</span>}
              </div>

              <nav className={`overflow-hidden max-h-[calc(100vh-200px)] ${sidebarCollapsed ? "flex flex-col items-center space-y-3 w-full" : "space-y-3"}`}>
                <div className={sidebarCollapsed ? "flex flex-col items-center space-y-3 w-full" : ""}>
                  <div className={sidebarCollapsed ? "flex flex-col items-center space-y-3 w-full" : "space-y-1"}>
                    {ADMIN_ITEMS.map((item) => {
                      const itemIcon = getNavigationIcon(item.path);
                      return (
                        <NavLink
                          key={item.path}
                          to={item.path}
                          className={({ isActive }) => cn(
                            "flex items-center rounded-lg transition-all duration-150 group relative",
                            sidebarCollapsed 
                              ? "justify-center p-2 hover:bg-gray-800" 
                              : "gap-3 px-3 py-2 justify-start text-sm",
                            isActive
                              ? "bg-gray-800 text-white"
                              : "text-gray-400 hover:text-gray-200"
                          )}
                          title={sidebarCollapsed ? item.label : ""}
                        >
                          {itemIcon}
                          {!sidebarCollapsed && <span>{item.label}</span>}
                          {sidebarCollapsed && (
                            <div className="absolute left-full ml-3 hidden group-hover:block bg-gray-900 text-white text-xs py-1.5 px-2.5 rounded whitespace-nowrap z-50 border border-gray-700">
                              {item.label}
                            </div>
                          )}
                        </NavLink>
                      );
                    })}
                  </div>
                </div>
              </nav>
            </>
          )}
        </aside>

        <div className="flex min-w-0 min-h-screen flex-col">
          <header className="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 bg-white px-6 py-4 shadow-sm md:flex-nowrap">
            <div className="flex min-w-0 items-center gap-4 flex-1">
              <button
                onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                className="lg:hidden flex items-center justify-center h-9 w-9 rounded-md border border-gray-300 hover:bg-gray-100 transition-colors text-gray-600 hover:text-gray-900"
                title="Toggle sidebar"
              >
                <Menu className="h-4 w-4" />
              </button>
              <CurrentPageTitle allFeatures={allFeatures} />
            </div>
            <div className="hidden flex-1 items-center gap-2 md:flex md:max-w-md">
              <div className="flex h-9 w-full items-center gap-2 rounded-md border border-gray-300 bg-gray-50 px-3 text-sm text-gray-600">
                <Search className="h-4 w-4 text-gray-400" />
                <span>Search…</span>
              </div>
            </div>
            <div className="flex min-w-0 w-full items-center justify-between gap-3 md:w-auto md:justify-start">
              <div className="flex min-w-0 flex-1 items-center gap-2 rounded-md border border-gray-300 bg-gray-50 px-3 py-2 text-sm md:flex-initial">
                <Building2 className="h-4 w-4 text-gray-600" />
                <select
                  value={activeClientId}
                  onChange={(e) => setActiveClientId(e.target.value)}
                  className="min-w-0 max-w-36 flex-1 cursor-pointer bg-transparent text-sm font-medium text-gray-900 focus:outline-none md:max-w-48"
                  title="Filter console by client"
                >
                  <option value="">All clients</option>
                  {(clients || []).map((client) => (
                    <option key={client.id} value={client.id}>
                      Client: {client.name}
                    </option>
                  ))}
                </select>
              </div>
              <Button variant="outline" size="sm" onClick={refreshDevices} title="Refresh" className="border-gray-300 hover:bg-gray-50">
                <RefreshCw className="h-4 w-4" />
              </Button>
              <NavLink to="/alerts" title="Notifications" className="relative inline-flex h-9 items-center justify-center rounded-md border border-gray-300 bg-gray-50 px-3 text-sm font-medium hover:bg-gray-100 transition-colors">
                <Bell className="h-4 w-4 text-gray-600" />
                {openAlertCount > 0 && (
                  <span className="absolute -right-1.5 -top-1.5 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-bold text-white">
                    {openAlertCount}
                  </span>
                )}
              </NavLink>
              <div className="h-6 w-px bg-gray-200"></div>
              <div className="flex items-center gap-3 min-w-0">
                <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-cyan-400 to-blue-500 text-xs font-semibold text-white flex-shrink-0">
                  {session.username?.[0]?.toUpperCase() || "?"}
                </div>
                <div className="hidden text-left leading-tight sm:block min-w-0">
                  <p className="text-sm font-medium text-gray-900">{session.username}</p>
                  <p className="text-xs text-gray-500 capitalize">{session.role}</p>
                </div>
                <Button variant="ghost" size="sm" onClick={logout} title="Logout" className="hover:bg-gray-100">
                  <LogOut className="h-4 w-4 text-gray-600" />
                </Button>
              </div>
            </div>
          </header>

          <main className="flex-1 p-6 md:p-8 bg-gray-50 overflow-auto">

          <Routes>
            <Route path="/" element={<Navigate to="/overview" replace />} />
            <Route path="/overview" element={<OverviewPage session={session} devices={devices} users={users} />} />
            <Route
              path="/work-queue"
              element={<WorkQueuePage workQueue={workQueue} refreshWorkQueue={refreshWorkQueue} appStatus={appStatus} />}
            />
            <Route
              path="/devices"
              element={
                <DevicesPage
                  devices={devices}
                  clients={scopedClients}
                  groups={groups}
                  canCommand={canCommand}
                  canManageUsers={canManageUsers}
                  canManagePSA={canManagePSA}
                  onSendPing={sendPing}
                  onDeleteDevice={deleteDevice}
                  onAssignDeviceClient={assignDeviceClient}
                  onAssignDeviceGroup={assignDeviceGroup}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/device-groups"
              element={
                <DeviceGroupsPage
                  groups={groups}
                  devices={devices}
                  canCommand={canCommand}
                  createDeviceGroup={createDeviceGroup}
                  deleteDeviceGroup={deleteDeviceGroup}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/assistant"
              element={<AIAssistantPage activeClientId={activeClientId} clients={clients} askAI={askAI} executeAIAction={executeAIAction} appStatus={appStatus} />}
            />
            <Route
              path="/scripts"
              element={
                <ScriptsPage
                  scripts={scripts}
                  devices={devices}
                  canCommand={canCommand}
                  createScript={createScript}
                  deleteScript={deleteScript}
                  runScript={runScript}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/alerts"
              element={
                <AlertsPage
                  alertRules={alertRules}
                  alerts={alerts}
                  devices={devices}
                  clients={scopedClients}
                  canCommand={canCommand}
                  createAlertRule={createAlertRule}
                  deleteAlertRule={deleteAlertRule}
                  acknowledgeAlert={acknowledgeAlert}
                  resolveAlert={resolveAlert}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/clients"
              element={
                <ClientsPage
                  clients={scopedClients}
                  canManagePSA={canManagePSA}
                  createClient={createClient}
                  deleteClient={deleteClient}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/contracts"
              element={
                <ContractsPage
                  contracts={contracts}
                  clients={scopedClients}
                  canManagePSA={canManagePSA}
                  createContract={createContract}
                  generateContractInvoice={generateContractInvoice}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/tickets"
              element={
                <TicketsPage
                  tickets={tickets}
                  clients={scopedClients}
                  devices={devices}
                  canManagePSA={canManagePSA}
                  createTicket={createTicket}
                  updateTicket={updateTicket}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/approvals"
              element={<PendingApprovals />}
            />
            <Route
              path="/billing"
              element={
                <BillingPage
                  token={token}
                  invoices={invoices}
                  clients={scopedClients}
                  canManagePSA={canManagePSA}
                  createInvoice={createInvoice}
                  updateInvoice={updateInvoice}
                  timeEntries={timeEntries}
                  createTimeEntry={createTimeEntry}
                  deleteTimeEntry={deleteTimeEntry}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/terminal"
              element={
                <TerminalPage
                  token={token}
                  devices={devices}
                  canCommand={canCommand}
                  createTerminalSession={createTerminalSession}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/files"
              element={
                <FilesPage
                  token={token}
                  devices={devices}
                  canCommand={canCommand}
                  listRemoteFiles={listRemoteFiles}
                  uploadRemoteFile={uploadRemoteFile}
                  activeClientId={activeClientId}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/events"
              element={<EventsPage canManageUsers={canManageUsers} auditEvents={auditEvents} refreshAuditEvents={refreshAuditEvents} appStatus={appStatus} />}
            />
            <Route
              path="/reports"
              element={
                <ReportsPage
                  reports={reports}
                  refreshReports={refreshReports}
                  loadReportMetrics={loadReportMetrics}
                  canManageUsers={canManageUsers}
                  createEnrollmentToken={createEnrollmentToken}
                  latestEnrollment={latestEnrollment}
                  enrollmentBootstrap={enrollmentBootstrap}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/users"
              element={
                <UsersPage
                  users={users}
                  canManageUsers={canManageUsers}
                  newUserForm={newUserForm}
                  setNewUserForm={setNewUserForm}
                  createUser={createUser}
                />
              }
            />
            <Route
              path="/enrollment"
              element={
                <EnrollmentPage
                  canManageUsers={canManageUsers}
                  createEnrollmentToken={createEnrollmentToken}
                  enrollmentBootstrap={enrollmentBootstrap}
                  refreshEnrollmentBootstrap={refreshEnrollmentBootstrap}
                  latestEnrollment={latestEnrollment}
                  devices={devices}
                  rotateDeviceId={rotateDeviceId}
                  setRotateDeviceId={setRotateDeviceId}
                  rotateAgentCredential={rotateAgentCredential}
                  latestRotatedCredential={latestRotatedCredential}
                  auditEvents={auditEvents}
                  refreshAuditEvents={refreshAuditEvents}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/settings"
              element={
                <SettingsPage
                  canManageUsers={canManageUsers}
                  agentEndpointSettings={agentEndpointSettings}
                  agentPublicAddrInput={agentPublicAddrInput}
                  setAgentPublicAddrInput={setAgentPublicAddrInput}
                  refreshAgentEndpointSettings={refreshAgentEndpointSettings}
                  saveAgentEndpointSettings={saveAgentEndpointSettings}
                  applicationSettings={applicationSettings}
                  setApplicationSettings={setApplicationSettings}
                  saveApplicationSettings={saveApplicationSettings}
                  onLogoUpload={onLogoUpload}
                  saveAIProviderAndLoadModels={saveAIProviderAndLoadModels}
                  appStatus={appStatus}
                />
              }
            />
            <Route
              path="/branding"
              element={<AdminBrandingPage token={token} appStatus={appStatus} />}
            />
            <Route
              path="/custom-fields"
              element={<AdminCustomFields token={token} />}
            />
            <Route
              path="/downloads"
              element={<AdminDownloads token={token} />}
            />
            {allFeatures
              .filter((f) => !["/overview", "/work-queue", "/devices", "/terminal", "/events", "/reports", "/users", "/enrollment", "/settings", "/branding", "/custom-fields", "/downloads", "/clients", "/contracts", "/tickets", "/billing", "/device-groups", "/assistant", "/scripts", "/alerts", "/files", "/approvals"].includes(f.path))
              .map((feature) => (
                <Route
                  key={feature.path}
                  path={feature.path}
                  element={<PlaceholderFeature title={feature.label} description={feature.description} />}
                />
              ))}
            <Route path="*" element={<Navigate to="/overview" replace />} />
          </Routes>
          </main>
        </div>
      </div>

    </>
  );
}

const SESSION_STORAGE_KEY = "gomeshcentral-session-v1";

function loadStoredSession() {
  if (typeof window === "undefined") {
    return { token: "", session: null };
  }

  try {
    const raw = window.localStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) return { token: "", session: null };
    const parsed = JSON.parse(raw);
    return {
      token: typeof parsed.token === "string" ? parsed.token : "",
      session: parsed.session && typeof parsed.session === "object" ? parsed.session : null,
    };
  } catch {
    return { token: "", session: null };
  }
}

const LIVE_DATA_REFRESH_MS = 15000;

function LiveRouteRefresh({ activeRouteRef, refreshRouteData }) {
  const location = useLocation();
  const refreshRef = useRef(refreshRouteData);
  refreshRef.current = refreshRouteData;

  useEffect(() => {
    activeRouteRef.current = location.pathname;
    const liveRoutes = new Set(["/overview", "/devices", "/reports", "/alerts", "/events"]);
    if (!liveRoutes.has(location.pathname)) return undefined;

    refreshRef.current(location.pathname);
    const refreshTimer = setInterval(() => {
      refreshRef.current(location.pathname);
    }, LIVE_DATA_REFRESH_MS);
    return () => clearInterval(refreshTimer);
  }, [activeRouteRef, location.pathname]);

  return null;
}

export default function App() {
  const savedAuth = useMemo(() => loadStoredSession(), []);
  const [token, setToken] = useState(savedAuth.token);
  const tokenRef = useRef(savedAuth.token);
  const [session, setSession] = useState(savedAuth.session);
  const sessionBootstrapRef = useRef(false);
  const [devices, setDevices] = useState([]);
  const [users, setUsers] = useState([]);
  const [loginStatus, setLoginStatus] = useState("");
  const [appStatus, setAppStatus] = useState("");
  const [loginForm, setLoginForm] = useState({ username: "", password: "" });
  const [showSignup, setShowSignup] = useState(false);
  const [signupForm, setSignupForm] = useState({ orgName: "", adminUsername: "", adminPassword: "" });
  const [signupStatus, setSignupStatus] = useState("");
  const [newUserForm, setNewUserForm] = useState({ username: "", password: "", email: "", role: "viewer", sendEmail: false });
  const [latestEnrollment, setLatestEnrollment] = useState(null);
  const [enrollmentBootstrap, setEnrollmentBootstrap] = useState(null);
  const [agentEndpointSettings, setAgentEndpointSettings] = useState(null);
  const [applicationSettings, setApplicationSettings] = useState({
    theme: "default",
    customDomain: "",
    logoDataUrl: "",
    mailForwarding: {
      invoiceTo: "",
      alertTo: "",
      smtpHost: "",
      smtpPort: 587,
      smtpUsername: "",
      smtpPassword: "",
      fromAddress: "",
    },
    ai: {
      provider: "openrouter",
      apiKey: "",
      apiKeyConfigured: false,
      baseUrl: "https://openrouter.ai/api/v1",
      model: "openai/gpt-4o-mini",
    },
  });
  const [branding, setBranding] = useState({
    companyName: "",
    phoneNumber: "",
    website: "",
    email: "",
    logo: "",
    icon: "",
  });
  const [agentPublicAddrInput, setAgentPublicAddrInput] = useState("");
  const [rotateDeviceId, setRotateDeviceId] = useState("");
  const [latestRotatedCredential, setLatestRotatedCredential] = useState(null);
  const [auditEvents, setAuditEvents] = useState([]);
  const [reports, setReports] = useState([]);
  const [workQueue, setWorkQueue] = useState(null);
  const [clients, setClients] = useState([]);
  const [contracts, setContracts] = useState([]);
  const [tickets, setTickets] = useState([]);
  const [invoices, setInvoices] = useState([]);
  const [timeEntries, setTimeEntries] = useState([]);
  const [groups, setGroups] = useState([]);
  const [scripts, setScripts] = useState([]);
  const [alertRules, setAlertRules] = useState([]);
  const [alerts, setAlerts] = useState([]);
  const [activeClientId, setActiveClientId] = useState("");
  const socketRef = useRef(null);
  const activeRouteRef = useRef("/overview");
  const liveRefreshersRef = useRef({});

  const canCommand = useMemo(
    () => session && Array.isArray(session.permissions) && session.permissions.includes("devices:command"),
    [session]
  );
  const canManageUsers = useMemo(
    () => session && Array.isArray(session.permissions) && session.permissions.includes("users:manage"),
    [session]
  );
  const canManagePSA = useMemo(
    () => session && Array.isArray(session.permissions) && session.permissions.includes("psa:manage"),
    [session]
  );

  useEffect(() => {
    tokenRef.current = token;
  }, [token]);

  useEffect(() => {
    if (token && session) {
      window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({ token, session }));
      return;
    }
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
  }, [token, session]);

  function getActiveToken() {
    return tokenRef.current || token;
  }

  function clearSessionState({ preserveStatus = false } = {}) {
    tokenRef.current = "";
    setToken("");
    setSession(null);
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(SESSION_STORAGE_KEY);
    }
    if (!preserveStatus) {
      setLoginStatus("Session expired. Please sign in again.");
    }
  }

  async function postJSON(path, body, auth = false) {
    const headers = { "Content-Type": "application/json" };
    const authToken = auth ? getActiveToken() : "";
    if (authToken) headers.Authorization = `Bearer ${authToken}`;
    const res = await fetch(apiUrl(path), {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    });
    if (res.status === 401 && auth) {
      clearSessionState();
      throw new Error("Session expired. Please sign in again.");
    }
    if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
    return res.status === 204 ? null : res.json().catch(() => null);
  }

  async function putJSON(path, body, auth = false) {
    const headers = { "Content-Type": "application/json" };
    const authToken = auth ? getActiveToken() : "";
    if (authToken) headers.Authorization = `Bearer ${authToken}`;
    const res = await fetch(apiUrl(path), {
      method: "PUT",
      headers,
      body: JSON.stringify(body),
    });
    if (res.status === 401 && auth) {
      clearSessionState();
      throw new Error("Session expired. Please sign in again.");
    }
    if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
    return res.status === 204 ? null : res.json().catch(() => null);
  }

  async function getJSON(path, auth = false) {
    const headers = {};
    const authToken = auth ? getActiveToken() : "";
    if (authToken) headers.Authorization = `Bearer ${authToken}`;
    const res = await fetch(apiUrl(path), { headers });
    if (res.status === 401 && auth) {
      clearSessionState();
      throw new Error("Session expired. Please sign in again.");
    }
    if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
    return res.json();
  }

  function scopedPath(path) {
    if (!activeClientId) return path;
    const separator = path.includes("?") ? "&" : "?";
    return `${path}${separator}clientId=${encodeURIComponent(activeClientId)}`;
  }

  async function refreshDevices() {
    try {
      const data = await getJSON(scopedPath("/api/devices"), true);
      setDevices(data);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshUsers() {
    if (!canManageUsers) return;
    try {
      const data = await getJSON("/api/users", true);
      setUsers(data);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshWorkQueue() {
    try {
      const data = await getJSON("/api/work-queue", true);
      setWorkQueue(data);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshReports() {
    try {
      const data = await getJSON(scopedPath("/api/reports"), true);
      setReports(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function loadReportMetrics(deviceId, minutes = 180) {
    try {
      return await getJSON(scopedPath(`/api/reports/${encodeURIComponent(deviceId)}/metrics?minutes=${minutes}`), true);
    } catch (err) {
      setAppStatus(err.message);
      return [];
    }
  }

  async function refreshAuditEvents() {
    try {
      const data = await getJSON("/api/audit-events", true);
      setAuditEvents(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function deleteDevice(deviceId) {
    try {
      const headers = {};
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl(scopedPath(`/api/devices/${deviceId}`)), {
        method: "DELETE",
        headers,
      });
      if (!res.ok) {
        throw new Error((await res.text()) || `Request failed: ${res.status}`);
      }
      setAppStatus(`Deleted stale device ${deviceId}`);
      await Promise.all([refreshDevices(), refreshReports(), canManageUsers ? refreshAuditEvents() : Promise.resolve()]);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function assignDeviceClient(deviceId, clientId) {
    try {
      await postJSON(scopedPath(`/api/devices/${encodeURIComponent(deviceId)}/client`), { clientId }, true);
      setAppStatus("Device client assignment updated");
      await refreshDevices();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshClients() {
    try {
      const data = await getJSON("/api/clients", true);
      setClients(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createClient(form) {
    try {
      await postJSON("/api/clients", form, true);
      setAppStatus(`Client ${form.name} created`);
      await refreshClients();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function deleteClient(id) {
    try {
      const headers = {};
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl(scopedPath(`/api/clients/${id}`)), { method: "DELETE", headers });
      if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
      setAppStatus("Client deleted");
      await Promise.all([refreshClients(), refreshDevices()]);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshContracts() {
    try {
      const data = await getJSON(scopedPath("/api/contracts"), true);
      setContracts(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createContract(form) {
    try {
      await postJSON(scopedPath("/api/contracts"), form, true);
      setAppStatus(`Contract ${form.name} created`);
      await refreshContracts();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function generateContractInvoice(contractId) {
    try {
      const invoice = await postJSON(scopedPath(`/api/contracts/${contractId}/generate-invoice`), {}, true);
      setAppStatus(`Invoice ${invoice.invoiceNumber} generated ($${invoice.total})`);
      await Promise.all([refreshInvoices(), refreshContracts(), refreshTimeEntries()]);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshTimeEntries() {
    try {
      const data = await getJSON(scopedPath("/api/time-entries"), true);
      setTimeEntries(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createTimeEntry(form) {
    try {
      await postJSON(scopedPath("/api/time-entries"), form, true);
      setAppStatus("Time entry logged");
      await refreshTimeEntries();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function deleteTimeEntry(id) {
    try {
      const headers = {};
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl(scopedPath(`/api/time-entries/${id}`)), { method: "DELETE", headers });
      if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
      setAppStatus("Time entry deleted");
      await refreshTimeEntries();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshTickets() {
    try {
      const data = await getJSON(scopedPath("/api/tickets"), true);
      setTickets(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createTicket(form) {
    try {
      await postJSON(scopedPath("/api/tickets"), form, true);
      setAppStatus(`Ticket "${form.subject}" created`);
      await refreshTickets();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function updateTicket(ticket) {
    try {
      await putJSON(scopedPath(`/api/tickets/${ticket.id}`), ticket, true);
      setAppStatus(`Ticket ${ticket.id} updated`);
      await refreshTickets();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshInvoices() {
    try {
      const data = await getJSON(scopedPath("/api/invoices"), true);
      setInvoices(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createInvoice(form) {
    try {
      await postJSON(scopedPath("/api/invoices"), form, true);
      setAppStatus("Invoice created");
      await refreshInvoices();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function updateInvoice(invoice) {
    try {
      await putJSON(scopedPath(`/api/invoices/${invoice.id}`), invoice, true);
      setAppStatus(`Invoice ${invoice.invoiceNumber} updated`);
      await refreshInvoices();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshDeviceGroups() {
    try {
      const data = await getJSON("/api/device-groups", true);
      setGroups(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createDeviceGroup(form) {
    try {
      await postJSON("/api/device-groups", form, true);
      setAppStatus(`Device group ${form.name} created`);
      await refreshDeviceGroups();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function deleteDeviceGroup(id) {
    try {
      const headers = {};
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl(`/api/device-groups/${id}`), { method: "DELETE", headers });
      if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
      setAppStatus("Device group deleted");
      await Promise.all([refreshDeviceGroups(), refreshDevices()]);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function assignDeviceGroup(deviceId, groupId) {
    try {
      await postJSON(scopedPath(`/api/devices/${encodeURIComponent(deviceId)}/group`), { groupId }, true);
      setAppStatus("Device group assignment updated");
      await refreshDevices();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshScripts() {
    try {
      const data = await getJSON("/api/scripts", true);
      setScripts(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createScript(form) {
    try {
      await postJSON("/api/scripts", form, true);
      setAppStatus(`Script ${form.name} saved`);
      await refreshScripts();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function deleteScript(id) {
    try {
      const headers = {};
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl(`/api/scripts/${id}`), { method: "DELETE", headers });
      if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
      setAppStatus("Script deleted");
      await refreshScripts();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function runScript(scriptId, deviceId) {
    try {
      await postJSON(`/api/scripts/${scriptId}/run`, { deviceId }, true);
      setAppStatus(`Script dispatched to ${deviceId}`);
      if (canManageUsers) await refreshAuditEvents();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshAlertRules() {
    try {
      const data = await getJSON(scopedPath("/api/alert-rules"), true);
      setAlertRules(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createAlertRule(form) {
    try {
      await postJSON(scopedPath("/api/alert-rules"), form, true);
      setAppStatus(`Alert rule ${form.name} created`);
      await refreshAlertRules();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function deleteAlertRule(id) {
    try {
      const headers = {};
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl(scopedPath(`/api/alert-rules/${id}`)), { method: "DELETE", headers });
      if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
      setAppStatus("Alert rule deleted");
      await refreshAlertRules();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshAlerts() {
    try {
      const data = await getJSON(scopedPath("/api/alerts"), true);
      setAlerts(Array.isArray(data) ? data : []);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function acknowledgeAlert(id) {
    try {
      await postJSON(scopedPath(`/api/alerts/${id}/acknowledge`), {}, true);
      await refreshAlerts();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function resolveAlert(id) {
    try {
      await postJSON(scopedPath(`/api/alerts/${id}/resolve`), {}, true);
      await refreshAlerts();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function listRemoteFiles(deviceId, path) {
    const query = path ? `?path=${encodeURIComponent(path)}` : "";
    return getJSON(scopedPath(`/api/devices/${encodeURIComponent(deviceId)}/files/list${query}`), true);
  }

  async function uploadRemoteFile(deviceId, destPath, file) {
    const headers = { "Content-Type": "application/octet-stream" };
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(apiUrl(scopedPath(`/api/devices/${encodeURIComponent(deviceId)}/files/upload?path=${encodeURIComponent(destPath)}`)), {
      method: "POST",
      headers,
      body: file,
    });
    if (!res.ok) throw new Error((await res.text()) || `Request failed: ${res.status}`);
    setAppStatus(`Uploaded ${file.name} to ${destPath}`);
  }

  function connectDashboardWS(authToken) {
    const socket = new WebSocket(`${wsBase()}/ws/dashboard?token=${encodeURIComponent(authToken)}`);
    socket.onopen = () => setAppStatus("Dashboard WebSocket connected");
    socket.onmessage = () => refreshRouteData(activeRouteRef.current);
    socket.onclose = () => setAppStatus("Dashboard WebSocket disconnected");
    return socket;
  }

  useEffect(() => {
    if (!token) {
      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
      return;
    }

    if (!sessionBootstrapRef.current && session) {
      sessionBootstrapRef.current = true;
      bootSessionData(session.permissions || []);
    }

    socketRef.current = connectDashboardWS(token);
    return () => {
      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
    };
  }, [token, session]);

  useEffect(() => {
    if (applicationSettings?.theme && applicationSettings.theme !== "default") {
      document.documentElement.setAttribute("data-theme", applicationSettings.theme);
    } else {
      document.documentElement.removeAttribute("data-theme");
    }
  }, [applicationSettings?.theme]);

  useEffect(() => {
    if (!token || !session) return;
    refreshDevices();
    refreshClients();
    refreshReports();
    refreshContracts();
    refreshTickets();
    refreshInvoices();
    refreshTimeEntries();
    refreshAlertRules();
    refreshAlerts();
    refreshUsers();
  }, [token, session, activeClientId, canManageUsers]);

  function refreshRouteData(path) {
    const refreshers = liveRefreshersRef.current;
    switch (path) {
    case "/overview":
      return Promise.all([refreshers.devices(), refreshers.reports(), refreshers.alerts()]);
    case "/devices":
      return refreshers.devices();
    case "/reports":
      return refreshers.reports();
    case "/alerts":
      return refreshers.alerts();
    case "/events":
      return refreshers.auditEvents();
    default:
      return Promise.resolve();
    }
  }

  liveRefreshersRef.current = {
    devices: refreshDevices,
    reports: refreshReports,
    alerts: refreshAlerts,
    auditEvents: canManageUsers ? refreshAuditEvents : () => Promise.resolve(),
  };

  async function bootSessionData(permissions = []) {
    await Promise.all([
      refreshDevices(),
      refreshReports(),
      refreshWorkQueue(),
      refreshClients(),
      refreshContracts(),
      refreshTickets(),
      refreshInvoices(),
      refreshTimeEntries(),
      refreshDeviceGroups(),
      refreshScripts(),
      refreshAlertRules(),
      refreshAlerts(),
      refreshBranding(),
      permissions.includes("users:manage") ? refreshUsers() : Promise.resolve(),
      permissions.includes("users:manage") ? refreshAuditEvents() : Promise.resolve(),
      permissions.includes("users:manage") ? refreshEnrollmentBootstrap() : Promise.resolve(),
      permissions.includes("users:manage") ? refreshAgentEndpointSettings() : Promise.resolve(),
      permissions.includes("users:manage") ? refreshApplicationSettings() : Promise.resolve(),
    ]);
  }

  async function afterAuthSuccess(data) {
    const authToken = data.token;
    sessionBootstrapRef.current = false;
    tokenRef.current = authToken;
    setToken(authToken);
    setSession({
      username: data.username,
      role: data.role,
      permissions: data.permissions || [],
      orgId: data.orgId || "default-org",
      orgName: data.orgName || "Default Organization",
    });
    setLoginStatus("");
    setSignupStatus("");
    setAppStatus(`Authenticated - Tenant: ${data.orgName || "Default Organization"}`);
    setDevices([]);
    setUsers([]);
    sessionBootstrapRef.current = true;
    await bootSessionData(data.permissions || []);
  }

  async function handleLogin() {
    try {
      const data = await postJSON("/api/login", loginForm);
      await afterAuthSuccess(data);
    } catch (err) {
      setLoginStatus(err.message);
    }
  }

  async function handleRegisterOrg() {
    try {
      const data = await postJSON("/api/organizations/register", signupForm);
      await afterAuthSuccess(data);
    } catch (err) {
      setSignupStatus(err.message);
    }
  }

  async function sendPing(deviceId) {
    try {
      await postJSON(scopedPath(`/api/devices/${deviceId}/command`), { command: "ping" }, true);
      setAppStatus(`Sent ping command to ${deviceId}`);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createUser() {
    try {
      await postJSON("/api/users", newUserForm, true);
      setAppStatus(`User ${newUserForm.username} created as ${newUserForm.role}`);
      setNewUserForm({ username: "", password: "", role: "viewer" });
      await refreshUsers();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createEnrollmentToken(ttlMinutes) {
    try {
      const data = await postJSON("/api/enrollment-tokens", { ttlMinutes }, true);
      setLatestEnrollment(data);
      setAppStatus("Enrollment token created");
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshEnrollmentBootstrap() {
    if (!canManageUsers) return;
    try {
      const data = await getJSON("/api/enrollment-bootstrap", true);
      setEnrollmentBootstrap(data);
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshAgentEndpointSettings() {
    if (!canManageUsers) return;
    try {
      const data = await getJSON("/api/settings/agent-endpoint", true);
      setAgentEndpointSettings(data);
      setAgentPublicAddrInput(data?.agentPublicAddr || "");
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshApplicationSettings() {
    if (!canManageUsers) return;
    try {
      const data = await getJSON("/api/settings/application", true);
      setApplicationSettings({
        theme: data?.theme || "default",
        customDomain: data?.customDomain || "",
        logoDataUrl: data?.logoDataUrl || "",
        mailForwarding: {
          invoiceTo: data?.mailForwarding?.invoiceTo || "",
          alertTo: data?.mailForwarding?.alertTo || "",
          smtpHost: data?.mailForwarding?.smtpHost || "",
          smtpPort: Number(data?.mailForwarding?.smtpPort || 587),
          smtpUsername: data?.mailForwarding?.smtpUsername || "",
          smtpPassword: data?.mailForwarding?.smtpPassword || "",
          fromAddress: data?.mailForwarding?.fromAddress || "",
        },
        ai: {
          provider: data?.ai?.provider || "openrouter",
          apiKey: "",
          apiKeyConfigured: Boolean(data?.ai?.apiKeyConfigured),
          baseUrl: data?.ai?.baseUrl || "https://openrouter.ai/api/v1",
          model: data?.ai?.model || "openai/gpt-4o-mini",
        },
      });
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function refreshBranding() {
    try {
      const data = await getJSON("/api/admin/branding", true);
      setBranding(data || {
        companyName: "",
        phoneNumber: "",
        website: "",
        email: "",
        logo: "",
        icon: "",
      });
    } catch (err) {
      console.error("Failed to load branding:", err);
    }
  }

  async function saveApplicationSettings() {
    if (!canManageUsers) return;
    try {
      const payload = {
        ...applicationSettings,
        mailForwarding: {
          ...applicationSettings.mailForwarding,
          smtpPort: Number(applicationSettings.mailForwarding.smtpPort || 587),
        },
      };
      const saved = await putJSON("/api/settings/application", payload, true);
      if (saved) {
        setApplicationSettings({
          theme: saved?.theme || "default",
          customDomain: saved?.customDomain || "",
          logoDataUrl: saved?.logoDataUrl || "",
          mailForwarding: {
            invoiceTo: saved?.mailForwarding?.invoiceTo || "",
            alertTo: saved?.mailForwarding?.alertTo || "",
            smtpHost: saved?.mailForwarding?.smtpHost || "",
            smtpPort: Number(saved?.mailForwarding?.smtpPort || 587),
            smtpUsername: saved?.mailForwarding?.smtpUsername || "",
            smtpPassword: saved?.mailForwarding?.smtpPassword || "",
            fromAddress: saved?.mailForwarding?.fromAddress || "",
          },
          ai: {
            provider: saved?.ai?.provider || "openrouter",
            apiKey: "",
            apiKeyConfigured: Boolean(saved?.ai?.apiKeyConfigured),
            baseUrl: saved?.ai?.baseUrl || "https://openrouter.ai/api/v1",
            model: saved?.ai?.model || "openai/gpt-4o-mini",
          },
        });
      }
      setAppStatus("Branding and mail settings saved.");
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  function onLogoUpload(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      setApplicationSettings((prev) => ({ ...prev, logoDataUrl: String(reader.result || "") }));
    };
    reader.readAsDataURL(file);
  }

  async function askAI(message) {
    return postJSON("/api/ai/chat", { message, clientId: activeClientId || "" }, true);
  }

  async function saveAIProviderAndLoadModels() {
    const payload = {
      ...applicationSettings,
      mailForwarding: {
        ...applicationSettings.mailForwarding,
        smtpPort: Number(applicationSettings.mailForwarding.smtpPort || 587),
      },
    };
    const saved = await putJSON("/api/settings/application", payload, true);
    const savedAI = {
      provider: saved?.ai?.provider || "openrouter",
      apiKey: "",
      apiKeyConfigured: Boolean(saved?.ai?.apiKeyConfigured),
      baseUrl: saved?.ai?.baseUrl || "https://openrouter.ai/api/v1",
      model: saved?.ai?.model || "",
    };
    setApplicationSettings((prev) => ({ ...prev, ai: savedAI }));
    const result = await postJSON("/api/ai/models", {
      provider: savedAI.provider,
      baseUrl: savedAI.baseUrl,
    }, true);
    setAppStatus("AI provider settings saved.");
    return result;
  }

  async function executeAIAction(action) {
    const result = await postJSON("/api/ai/actions", { action, clientId: activeClientId || "" }, true);
    await Promise.all([refreshDevices(), refreshTickets(), refreshAlerts()]);
    setAppStatus("AI-proposed action completed after approval.");
    return result;
  }

  async function saveAgentEndpointSettings() {
    if (!canManageUsers) return;
    try {
      const headers = { "Content-Type": "application/json" };
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(apiUrl("/api/settings/agent-endpoint"), {
        method: "PUT",
        headers,
        body: JSON.stringify({ agentPublicAddr: agentPublicAddrInput }),
      });
      if (!res.ok) {
        throw new Error((await res.text()) || `Request failed: ${res.status}`);
      }
      const data = await res.json().catch(() => null);
      if (data) {
        setAgentEndpointSettings(data);
        setAgentPublicAddrInput(data?.agentPublicAddr || "");
      }
      await refreshEnrollmentBootstrap();
      setAppStatus("Settings saved. New enrollment commands will use the updated endpoint.");
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function rotateAgentCredential() {
    if (!rotateDeviceId) {
      setAppStatus("Select a device to rotate credential");
      return;
    }
    try {
      const data = await postJSON("/api/agents/admin-rotate-key", { deviceId: rotateDeviceId }, true);
      setLatestRotatedCredential(data);
      setAppStatus(`Credential rotated for ${rotateDeviceId}`);
      await refreshAuditEvents();
    } catch (err) {
      setAppStatus(err.message);
    }
  }

  async function createTerminalSession(deviceId, cols = 120, rows = 32) {
    return postJSON(scopedPath(`/api/devices/${encodeURIComponent(deviceId)}/terminal/sessions`), { cols, rows }, true);
  }

  function logout() {
    clearSessionState({ preserveStatus: true });
    setDevices([]);
    setUsers([]);
    setAppStatus("");
    setLoginStatus("Logged out");
    setLatestEnrollment(null);
    setEnrollmentBootstrap(null);
    setAgentEndpointSettings(null);
    setAgentPublicAddrInput("");
    setRotateDeviceId("");
    setLatestRotatedCredential(null);
    setAuditEvents([]);
    setReports([]);
    setWorkQueue(null);
    setClients([]);
    setContracts([]);
    setTickets([]);
    setInvoices([]);
    setTimeEntries([]);
    setGroups([]);
    setScripts([]);
    setAlertRules([]);
    setAlerts([]);
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/client" element={<ClientLogin />} />
        <Route path="/client/portal/*" element={<ClientPortal />} />
        <Route path="/*" element={
          <div className="bg-atmo min-h-screen">
            {!session ? (
              <main className="mx-auto w-full max-w-[1600px] p-4 md:p-6">
                <LoginView
                  loginForm={loginForm}
                  setLoginForm={setLoginForm}
                  handleLogin={handleLogin}
                  loginStatus={loginStatus}
                  showSignup={showSignup}
                  setShowSignup={setShowSignup}
                  signupForm={signupForm}
            setSignupForm={setSignupForm}
            handleRegisterOrg={handleRegisterOrg}
            signupStatus={signupStatus}
          />
        </main>
      ) : (
        <AppShell
            token={token}
            session={session}
            logout={logout}
            refreshDevices={refreshDevices}
            refreshUsers={refreshUsers}
            devices={devices}
            users={users}
            appStatus={appStatus}
            canCommand={canCommand}
            canManageUsers={canManageUsers}
            sendPing={sendPing}
            newUserForm={newUserForm}
            setNewUserForm={setNewUserForm}
            createUser={createUser}
            createEnrollmentToken={createEnrollmentToken}
            enrollmentBootstrap={enrollmentBootstrap}
            refreshEnrollmentBootstrap={refreshEnrollmentBootstrap}
            latestEnrollment={latestEnrollment}
            rotateDeviceId={rotateDeviceId}
            setRotateDeviceId={setRotateDeviceId}
            rotateAgentCredential={rotateAgentCredential}
            latestRotatedCredential={latestRotatedCredential}
            auditEvents={auditEvents}
            refreshAuditEvents={refreshAuditEvents}
            reports={reports}
            refreshReports={refreshReports}
            loadReportMetrics={loadReportMetrics}
            deleteDevice={deleteDevice}
            workQueue={workQueue}
            refreshWorkQueue={refreshWorkQueue}
			createTerminalSession={createTerminalSession}
            agentEndpointSettings={agentEndpointSettings}
            agentPublicAddrInput={agentPublicAddrInput}
            setAgentPublicAddrInput={setAgentPublicAddrInput}
            refreshAgentEndpointSettings={refreshAgentEndpointSettings}
            saveAgentEndpointSettings={saveAgentEndpointSettings}
            applicationSettings={applicationSettings}
            setApplicationSettings={setApplicationSettings}
            saveApplicationSettings={saveApplicationSettings}
            branding={branding}
            refreshBranding={refreshBranding}
            onLogoUpload={onLogoUpload}
            saveAIProviderAndLoadModels={saveAIProviderAndLoadModels}
            askAI={askAI}
            executeAIAction={executeAIAction}
            canManagePSA={canManagePSA}
            clients={clients}
            createClient={createClient}
            deleteClient={deleteClient}
            assignDeviceClient={assignDeviceClient}
            contracts={contracts}
            createContract={createContract}
            generateContractInvoice={generateContractInvoice}
            tickets={tickets}
            createTicket={createTicket}
            updateTicket={updateTicket}
            invoices={invoices}
            createInvoice={createInvoice}
            updateInvoice={updateInvoice}
            timeEntries={timeEntries}
            createTimeEntry={createTimeEntry}
            deleteTimeEntry={deleteTimeEntry}
            groups={groups}
            createDeviceGroup={createDeviceGroup}
            deleteDeviceGroup={deleteDeviceGroup}
            assignDeviceGroup={assignDeviceGroup}
            scripts={scripts}
            createScript={createScript}
            deleteScript={deleteScript}
            runScript={runScript}
            alertRules={alertRules}
            alerts={alerts}
            createAlertRule={createAlertRule}
            deleteAlertRule={deleteAlertRule}
            acknowledgeAlert={acknowledgeAlert}
            resolveAlert={resolveAlert}
            listRemoteFiles={listRemoteFiles}
            uploadRemoteFile={uploadRemoteFile}
            activeClientId={activeClientId}
            setActiveClientId={setActiveClientId}
            activeRouteRef={activeRouteRef}
            refreshRouteData={refreshRouteData}
          />
      )}
          </div>
        } />
      </Routes>
    </BrowserRouter>
  );
}
