import React, { useState, useEffect } from "react";
import { AlertCircle, Plus, Trash2, Loader2 } from "lucide-react";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/table";

const apiUrl = () => (window.location.origin === "http://localhost:5173" ? "http://localhost:8080" : window.location.origin);

export default function AdminCustomFields({ token }) {
	const [fields, setFields] = useState([]);
	const [newFieldName, setNewFieldName] = useState("");
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState("");
	const [deleting, setDeleting] = useState(null);

	// Load custom fields on mount
	useEffect(() => {
		loadFields();
	}, [token]);

	const loadFields = async () => {
		try {
			setLoading(true);
			setError("");
			const response = await fetch(`${apiUrl()}/api/devices/custom-fields`, {
				method: "GET",
				headers: {
					Authorization: `Bearer ${token}`,
					"Content-Type": "application/json",
				},
			});

			if (response.ok) {
				const data = await response.json();
				setFields(data || []);
			} else if (response.status === 401) {
				setError("Unauthorized - please log in");
			} else {
				setError("Failed to load custom fields");
			}
		} catch (err) {
			setError("Error loading custom fields: " + err.message);
		} finally {
			setLoading(false);
		}
	};

	const createField = async () => {
		if (!newFieldName.trim()) {
			setError("Field name is required");
			return;
		}

		try {
			setLoading(true);
			setError("");
			const response = await fetch(`${apiUrl()}/api/devices/custom-fields`, {
				method: "POST",
				headers: {
					Authorization: `Bearer ${token}`,
					"Content-Type": "application/json",
				},
				body: JSON.stringify({ fieldName: newFieldName.trim() }),
			});

			if (response.ok) {
				setNewFieldName("");
				await loadFields();
			} else if (response.status === 401) {
				setError("Unauthorized - please log in");
			} else {
				const text = await response.text();
				setError(text || "Failed to create field");
			}
		} catch (err) {
			setError("Error creating field: " + err.message);
		} finally {
			setLoading(false);
		}
	};

	const deleteField = async (fieldName) => {
		if (!confirm(`Delete custom field "${fieldName}"? This will remove it from all devices.`)) {
			return;
		}

		try {
			setDeleting(fieldName);
			setError("");
			const response = await fetch(`${apiUrl()}/api/devices/custom-fields/${fieldName}`, {
				method: "DELETE",
				headers: {
					Authorization: `Bearer ${token}`,
					"Content-Type": "application/json",
				},
			});

			if (response.ok) {
				await loadFields();
			} else if (response.status === 401) {
				setError("Unauthorized - please log in");
			} else {
				const text = await response.text();
				setError(text || "Failed to delete field");
			}
		} catch (err) {
			setError("Error deleting field: " + err.message);
		} finally {
			setDeleting(null);
		}
	};

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-3xl font-bold tracking-tight">Custom Device Fields</h1>
				<p className="text-gray-400 mt-2">Define custom metadata fields for devices in your organization</p>
			</div>

			{error && (
				<div className="flex gap-2 bg-red-900/20 border border-red-800 rounded-md p-4">
					<AlertCircle className="h-5 w-5 text-red-500 flex-shrink-0 mt-0.5" />
					<span className="text-sm text-red-200">{error}</span>
				</div>
			)}

			<Card className="bg-slate-900 border-slate-700">
				<CardHeader>
					<CardTitle>Create New Field</CardTitle>
					<CardDescription>Add a new custom field definition</CardDescription>
				</CardHeader>
				<CardContent>
					<div className="flex gap-2">
						<Input
							placeholder="e.g., Location, Cost Center, Serial Number"
							value={newFieldName}
							onChange={(e) => setNewFieldName(e.target.value)}
							onKeyPress={(e) => e.key === "Enter" && createField()}
							disabled={loading}
							className="flex-1"
						/>
						<Button
							onClick={createField}
							disabled={loading || !newFieldName.trim()}
							className="bg-blue-600 hover:bg-blue-700 text-white"
						>
							{loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
							<span className="hidden sm:inline ml-2">Create Field</span>
						</Button>
					</div>
				</CardContent>
			</Card>

			<Card className="bg-slate-900 border-slate-700">
				<CardHeader>
					<CardTitle>Defined Fields</CardTitle>
					<CardDescription>
						{fields.length === 0 ? "No custom fields defined yet" : `${fields.length} field${fields.length !== 1 ? "s" : ""} defined`}
					</CardDescription>
				</CardHeader>
				<CardContent>
					{loading && !fields.length ? (
						<div className="flex items-center justify-center py-8">
							<Loader2 className="h-6 w-6 animate-spin text-blue-500" />
						</div>
					) : fields.length === 0 ? (
						<div className="text-center py-8">
							<p className="text-gray-400">No custom fields defined. Create one to get started.</p>
						</div>
					) : (
						<Table>
							<TableHeader>
								<TableRow className="border-slate-700">
									<TableHead>Field Name</TableHead>
									<TableHead className="text-right">Actions</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{fields.map((field) => (
									<TableRow key={field} className="border-slate-700 hover:bg-slate-800/50">
										<TableCell className="font-medium">{field}</TableCell>
										<TableCell className="text-right">
											<Button
												variant="ghost"
												size="sm"
												onClick={() => deleteField(field)}
												disabled={deleting === field}
												className="text-red-400 hover:text-red-300 hover:bg-red-900/20"
											>
												{deleting === field ? (
													<Loader2 className="h-4 w-4 animate-spin" />
												) : (
													<Trash2 className="h-4 w-4" />
												)}
											</Button>
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					)}
				</CardContent>
			</Card>

			<Card className="bg-slate-900 border-slate-700">
				<CardHeader>
					<CardTitle>Usage</CardTitle>
					<CardDescription>How to use custom fields</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4 text-sm text-gray-300">
					<div>
						<h4 className="font-semibold text-white mb-2">Viewing Device Fields</h4>
						<p className="text-gray-400">Navigate to any device and view its custom field values in the details panel.</p>
					</div>
					<div>
						<h4 className="font-semibold text-white mb-2">Editing Device Fields</h4>
						<p className="text-gray-400">Click "Edit" on a device to modify any custom field values.</p>
					</div>
					<div>
						<h4 className="font-semibold text-white mb-2">Field Names</h4>
						<p className="text-gray-400">Field names are unique per organization and can contain letters, numbers, spaces, and common punctuation.</p>
					</div>
				</CardContent>
			</Card>
		</div>
	);
}
