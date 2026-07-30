"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { motion, AnimatePresence } from "framer-motion";
import {
  apiGetAdminStats,
  apiGetAdminUsers,
  apiUpdateUserStatus,
  apiUpdateUserRole,
  UserUsageStat,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export default function AdminDashboardPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { user, isAuthenticated, isLoading: authLoading, logout } = useAuth();

  const [searchTerm, setSearchTerm] = useState("");
  const [filterTab, setFilterTab] = useState<"all" | "active" | "restricted" | "admins">("all");
  const [actionError, setActionError] = useState<string | null>(null);

  // Client-side Admin Route Guard
  useEffect(() => {
    if (!authLoading) {
      if (!isAuthenticated) {
        router.replace("/login");
      } else if (user?.role !== "admin") {
        router.replace("/chat");
      }
    }
  }, [authLoading, isAuthenticated, user, router]);

  // Fetch Admin Stats
  const { data: stats } = useQuery({
    queryKey: ["admin-stats"],
    queryFn: () => apiGetAdminStats(),
    enabled: isAuthenticated && user?.role === "admin",
    refetchInterval: 5000,
  });

  // Fetch Users Usage List
  const { data: usersData, isLoading: usersLoading } = useQuery({
    queryKey: ["admin-users"],
    queryFn: () => apiGetAdminUsers(),
    enabled: isAuthenticated && user?.role === "admin",
    refetchInterval: 5000,
  });

  const usersList = usersData?.items ?? [];

  // Toggle User Restricted/Active Status
  const statusMutation = useMutation({
    mutationFn: ({ userId, isDisabled }: { userId: string; isDisabled: boolean }) =>
      apiUpdateUserStatus(userId, isDisabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      queryClient.invalidateQueries({ queryKey: ["admin-stats"] });
      setActionError(null);
    },
    onError: (err: Error) => {
      setActionError(err.message || "Failed to update user status");
    },
  });

  // Toggle User Role (Admin / User)
  const roleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: "user" | "admin" }) =>
      apiUpdateUserRole(userId, role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin-users"] });
      queryClient.invalidateQueries({ queryKey: ["admin-stats"] });
      setActionError(null);
    },
    onError: (err: Error) => {
      setActionError(err.message || "Failed to update user role");
    },
  });

  if (authLoading || !isAuthenticated || user?.role !== "admin") {
    return (
      <div
        style={{
          display: "flex",
          height: "100vh",
          alignItems: "center",
          justifyContent: "center",
          background: "var(--color-background)",
          color: "var(--color-on-surface-variant)",
          fontFamily: "var(--font-geist)",
        }}
      >
        <span className="material-symbols-outlined spin" style={{ fontSize: "32px", color: "var(--color-primary)" }}>
          sync
        </span>
      </div>
    );
  }

  // Filter users based on search term & tab filter
  const filteredUsers = usersList.filter((u) => {
    const matchesSearch =
      u.full_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      u.email.toLowerCase().includes(searchTerm.toLowerCase());

    if (!matchesSearch) return false;

    if (filterTab === "active") return !u.is_disabled;
    if (filterTab === "restricted") return u.is_disabled;
    if (filterTab === "admins") return u.role === "admin";
    return true;
  });

  return (
    <div
      style={{
        minHeight: "100vh",
        background: "var(--color-background)",
        color: "var(--color-on-surface)",
        fontFamily: "var(--font-inter)",
        padding: "24px clamp(16px, 4vw, 48px)",
      }}
    >
      {/* ── Top Header ── */}
      <header
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: "32px",
          paddingBottom: "20px",
          borderBottom: "1px solid var(--color-outline-variant)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "14px" }}>
          <div
            style={{
              width: "44px",
              height: "44px",
              borderRadius: "var(--radius-lg)",
              background: "linear-gradient(135deg, var(--color-primary), #E5BA6A)",
              display: "grid",
              placeItems: "center",
              boxShadow: "0 4px 16px rgba(217,164,65,0.25)",
            }}
          >
            <span className="material-symbols-outlined" style={{ color: "#141311", fontSize: "24px" }}>
              admin_panel_settings
            </span>
          </div>
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
              <h1 style={{ margin: 0, fontSize: "20px", fontWeight: 700, fontFamily: "var(--font-geist)" }}>
                Admin Control Center
              </h1>
              <span
                style={{
                  padding: "3px 9px",
                  borderRadius: "var(--radius-full)",
                  background: "var(--color-accent-light)",
                  border: "1px solid var(--color-accent-border)",
                  color: "var(--color-primary-fixed)",
                  fontSize: "11px",
                  fontWeight: 700,
                  fontFamily: "var(--font-geist)",
                }}
              >
                SUPERADMIN
              </span>
            </div>
            <p style={{ margin: "2px 0 0 0", fontSize: "13px", color: "var(--color-on-surface-variant)" }}>
              Monitor system usage, data sources, and manage user access controls.
            </p>
          </div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
          <button
            type="button"
            onClick={() => router.push("/chat")}
            style={{
              display: "flex",
              alignItems: "center",
              gap: "6px",
              padding: "9px 16px",
              borderRadius: "var(--radius-md)",
              background: "var(--color-surface-container-high)",
              border: "1px solid var(--color-outline-variant)",
              color: "var(--color-on-surface)",
              fontFamily: "var(--font-geist)",
              fontSize: "13px",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>
              chat
            </span>
            Go to Chat
          </button>
          <button
            type="button"
            onClick={logout}
            style={{
              display: "flex",
              alignItems: "center",
              gap: "6px",
              padding: "9px 16px",
              borderRadius: "var(--radius-md)",
              background: "var(--color-surface-container)",
              border: "1px solid var(--color-outline-variant)",
              color: "var(--color-error)",
              fontFamily: "var(--font-geist)",
              fontSize: "13px",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            <span className="material-symbols-outlined" style={{ fontSize: "18px" }}>
              logout
            </span>
            Sign Out
          </button>
        </div>
      </header>

      {/* ── System Telemetry Cards ── */}
      <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: "16px", marginBottom: "36px" }}>
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          className="glass-card"
          style={{ padding: "20px", borderRadius: "var(--radius-xl)" }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
            <span style={{ fontSize: "11px", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)" }}>
              Total Accounts
            </span>
            <span className="material-symbols-outlined" style={{ color: "var(--color-primary)", fontSize: "20px" }}>
              group
            </span>
          </div>
          <div style={{ fontSize: "28px", fontWeight: 700, margin: "10px 0 4px", fontFamily: "var(--font-geist)" }}>
            {stats?.total_users ?? 0}
          </div>
          <span style={{ fontSize: "12px", color: "var(--color-tertiary)" }}>
            {stats?.active_users ?? 0} active • {stats?.restricted_users ?? 0} restricted
          </span>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.05 }}
          className="glass-card"
          style={{ padding: "20px", borderRadius: "var(--radius-xl)" }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
            <span style={{ fontSize: "11px", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)" }}>
              Conversations
            </span>
            <span className="material-symbols-outlined" style={{ color: "var(--color-tertiary)", fontSize: "20px" }}>
              forum
            </span>
          </div>
          <div style={{ fontSize: "28px", fontWeight: 700, margin: "10px 0 4px", fontFamily: "var(--font-geist)" }}>
            {stats?.total_conversations ?? 0}
          </div>
          <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)" }}>
            Notebooks & chat sessions
          </span>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          className="glass-card"
          style={{ padding: "20px", borderRadius: "var(--radius-xl)" }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
            <span style={{ fontSize: "11px", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)" }}>
              Data Sources (Docs)
            </span>
            <span className="material-symbols-outlined" style={{ color: "#60A5FA", fontSize: "20px" }}>
              folder_open
            </span>
          </div>
          <div style={{ fontSize: "28px", fontWeight: 700, margin: "10px 0 4px", fontFamily: "var(--font-geist)" }}>
            {stats?.total_documents ?? 0}
          </div>
          <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)" }}>
            PDFs, Videos, Text & Web URLs
          </span>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.15 }}
          className="glass-card"
          style={{ padding: "20px", borderRadius: "var(--radius-xl)" }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
            <span style={{ fontSize: "11px", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--color-on-surface-variant)", fontFamily: "var(--font-geist)" }}>
              Total Messages / Chats
            </span>
            <span className="material-symbols-outlined" style={{ color: "#F472B6", fontSize: "20px" }}>
              chat_bubble
            </span>
          </div>
          <div style={{ fontSize: "28px", fontWeight: 700, margin: "10px 0 4px", fontFamily: "var(--font-geist)" }}>
            {stats?.total_messages ?? 0}
          </div>
          <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)" }}>
            {stats?.total_chunks ?? 0} vector chunks indexed
          </span>
        </motion.div>
      </section>

      {/* ── Error Banner ── */}
      <AnimatePresence>
        {actionError && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            style={{
              padding: "12px 16px",
              borderRadius: "var(--radius-md)",
              background: "rgba(239, 68, 68, 0.1)",
              border: "1px solid rgba(239, 68, 68, 0.3)",
              color: "#FCA5A5",
              fontSize: "13px",
              marginBottom: "20px",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <span>⚠️ {actionError}</span>
            <button onClick={() => setActionError(null)} style={{ background: "none", border: "none", color: "#FCA5A5", cursor: "pointer" }}>
              ✕
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      {/* ── User Telemetry & Access Control Table ── */}
      <section
        className="glass-card"
        style={{ padding: "24px", borderRadius: "var(--radius-xl)", background: "var(--color-surface-container-low)" }}
      >
        {/* Table Control Bar */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: "16px", flexWrap: "wrap", marginBottom: "20px" }}>
          <div>
            <h2 style={{ margin: 0, fontSize: "16px", fontWeight: 700, fontFamily: "var(--font-geist)" }}>
              User Accounts & Usage Metrics
            </h2>
            <p style={{ margin: "2px 0 0 0", fontSize: "12.5px", color: "var(--color-on-surface-variant)" }}>
              Track usage per user and toggle account restriction or admin privileges.
            </p>
          </div>

          {/* Search & Tabs */}
          <div style={{ display: "flex", alignItems: "center", gap: "12px", flexWrap: "wrap" }}>
            {/* Search Input */}
            <div style={{ position: "relative", minWidth: "240px" }}>
              <span className="material-symbols-outlined" style={{ position: "absolute", left: "12px", top: "50%", transform: "translateY(-50%)", fontSize: "18px", color: "var(--color-on-surface-variant)" }}>
                search
              </span>
              <input
                type="text"
                placeholder="Search user or email..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                style={{
                  width: "100%",
                  padding: "8px 12px 8px 36px",
                  borderRadius: "var(--radius-md)",
                  background: "var(--color-surface-container-lowest)",
                  border: "1px solid var(--color-outline-variant)",
                  color: "var(--color-on-surface)",
                  fontSize: "13px",
                  outline: "none",
                }}
              />
            </div>

            {/* Filter Tabs */}
            <div style={{ display: "flex", background: "var(--color-surface-container)", padding: "3px", borderRadius: "var(--radius-md)", border: "1px solid var(--color-outline-variant)" }}>
              {(["all", "active", "restricted", "admins"] as const).map((tab) => (
                <button
                  key={tab}
                  type="button"
                  onClick={() => setFilterTab(tab)}
                  style={{
                    padding: "5px 12px",
                    borderRadius: "var(--radius-sm)",
                    border: "none",
                    background: filterTab === tab ? "var(--color-surface-container-high)" : "transparent",
                    color: filterTab === tab ? "var(--color-primary)" : "var(--color-on-surface-variant)",
                    fontSize: "12px",
                    fontWeight: 600,
                    cursor: "pointer",
                    textTransform: "capitalize",
                  }}
                >
                  {tab}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Table Content */}
        {usersLoading ? (
          <div style={{ padding: "40px", textAlign: "center", color: "var(--color-on-surface-variant)" }}>
            <span className="material-symbols-outlined spin" style={{ fontSize: "28px" }}>
              sync
            </span>
            <p style={{ margin: "8px 0 0 0", fontSize: "13px" }}>Loading user statistics...</p>
          </div>
        ) : filteredUsers.length === 0 ? (
          <div style={{ padding: "40px", textAlign: "center", color: "var(--color-on-surface-variant)" }}>
            <span className="material-symbols-outlined" style={{ fontSize: "36px", opacity: 0.5 }}>
              group_off
            </span>
            <p style={{ margin: "8px 0 0 0", fontSize: "14px" }}>No users match the search criteria.</p>
          </div>
        ) : (
          <div style={{ overflowX: "auto" }}>
            <table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left", fontSize: "13px" }}>
              <thead>
                <tr style={{ borderBottom: "1px solid var(--color-outline-variant)", color: "var(--color-on-surface-variant)" }}>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>User</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Status</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Role</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Conversations</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Data Sources</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Messages</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Chunks</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)" }}>Joined Date</th>
                  <th style={{ padding: "12px 14px", fontWeight: 600, fontFamily: "var(--font-geist)", textAlign: "right" }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredUsers.map((u) => {
                  const isSelf = u.id === user?.id;
                  return (
                    <tr
                      key={u.id}
                      style={{
                        borderBottom: "1px solid var(--color-outline-variant)",
                        transition: "background var(--transition-fast)",
                      }}
                    >
                      {/* User details */}
                      <td style={{ padding: "14px" }}>
                        <div style={{ fontWeight: 600, color: "var(--color-on-surface)" }}>{u.full_name}</div>
                        <div style={{ fontSize: "12px", color: "var(--color-on-surface-variant)" }}>{u.email}</div>
                      </td>

                      {/* Status */}
                      <td style={{ padding: "14px" }}>
                        {u.is_disabled ? (
                          <span
                            style={{
                              display: "inline-flex",
                              alignItems: "center",
                              gap: "4px",
                              padding: "3px 8px",
                              borderRadius: "var(--radius-full)",
                              background: "rgba(239, 68, 68, 0.15)",
                              color: "#FCA5A5",
                              border: "1px solid rgba(239, 68, 68, 0.3)",
                              fontSize: "11px",
                              fontWeight: 600,
                            }}
                          >
                            🔒 Restricted
                          </span>
                        ) : (
                          <span
                            style={{
                              display: "inline-flex",
                              alignItems: "center",
                              gap: "4px",
                              padding: "3px 8px",
                              borderRadius: "var(--radius-full)",
                              background: "rgba(34, 197, 94, 0.15)",
                              color: "#86EFAC",
                              border: "1px solid rgba(34, 197, 94, 0.3)",
                              fontSize: "11px",
                              fontWeight: 600,
                            }}
                          >
                            ✓ Active
                          </span>
                        )}
                      </td>

                      {/* Role */}
                      <td style={{ padding: "14px" }}>
                        <span
                          style={{
                            padding: "3px 8px",
                            borderRadius: "var(--radius-sm)",
                            background: u.role === "admin" ? "var(--color-accent-light)" : "var(--color-surface-container)",
                            border: `1px solid ${u.role === "admin" ? "var(--color-accent-border)" : "var(--color-outline-variant)"}`,
                            color: u.role === "admin" ? "var(--color-primary-fixed)" : "var(--color-on-surface-variant)",
                            fontSize: "11px",
                            fontWeight: 700,
                            fontFamily: "var(--font-geist)",
                            textTransform: "uppercase",
                          }}
                        >
                          {u.role}
                        </span>
                      </td>

                      {/* Stats */}
                      <td style={{ padding: "14px", fontWeight: 600 }}>{u.conversation_count}</td>
                      <td style={{ padding: "14px", fontWeight: 600 }}>{u.document_count}</td>
                      <td style={{ padding: "14px", fontWeight: 600 }}>{u.message_count}</td>
                      <td style={{ padding: "14px", color: "var(--color-on-surface-variant)" }}>{u.chunk_count}</td>

                      {/* Joined Date */}
                      <td style={{ padding: "14px", fontSize: "12px", color: "var(--color-on-surface-variant)" }}>
                        {new Date(u.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}
                      </td>

                      {/* Actions */}
                      <td style={{ padding: "14px", textAlign: "right" }}>
                        {isSelf ? (
                          <span style={{ fontSize: "12px", color: "var(--color-on-surface-variant)", fontStyle: "italic" }}>
                            You (Current)
                          </span>
                        ) : (
                          <div style={{ display: "flex", gap: "8px", justifyContent: "flex-end" }}>
                            {/* Toggle Status Button */}
                            <button
                              type="button"
                              onClick={() => statusMutation.mutate({ userId: u.id, isDisabled: !u.is_disabled })}
                              disabled={statusMutation.isPending}
                              style={{
                                padding: "5px 10px",
                                borderRadius: "var(--radius-md)",
                                background: u.is_disabled ? "rgba(34, 197, 94, 0.15)" : "rgba(239, 68, 68, 0.15)",
                                border: `1px solid ${u.is_disabled ? "rgba(34, 197, 94, 0.3)" : "rgba(239, 68, 68, 0.3)"}`,
                                color: u.is_disabled ? "#86EFAC" : "#FCA5A5",
                                fontSize: "11.5px",
                                fontWeight: 600,
                                cursor: "pointer",
                              }}
                            >
                              {u.is_disabled ? "Enable" : "Restrict"}
                            </button>

                            {/* Toggle Role Button */}
                            <button
                              type="button"
                              onClick={() => roleMutation.mutate({ userId: u.id, role: u.role === "admin" ? "user" : "admin" })}
                              disabled={roleMutation.isPending}
                              style={{
                                padding: "5px 10px",
                                borderRadius: "var(--radius-md)",
                                background: "var(--color-surface-container-high)",
                                border: "1px solid var(--color-outline-variant)",
                                color: "var(--color-on-surface)",
                                fontSize: "11.5px",
                                fontWeight: 600,
                                cursor: "pointer",
                              }}
                            >
                              {u.role === "admin" ? "Demote" : "Make Admin"}
                            </button>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
