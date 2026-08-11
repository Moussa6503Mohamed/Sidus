"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Message } from "@/components/ui/Message";
import { Skeleton } from "@/components/ui/Skeleton";
import type { EditorialRole } from "@/lib/editorial/permissions";
import styles from "./styles.module.css";

type Upload = {
  id: string;
  originalFilename: string;
  byteSize: number;
  status: string;
  retentionState: string;
  createdAt: string;
};
type Source = { id: string; title: string; status: string; catalogueSyllabusId: string | null };

const api = async (path: string, init?: RequestInit) => {
  const response = await fetch(path, init);
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(typeof body.message === "string" ? body.message : "Request failed.");
  return body;
};

function uploadStatusLabel(status: string) {
  return status.replaceAll("_", " ");
}

export function PrivateUploadsScreen({ role }: { role: EditorialRole }) {
  const [items, setItems] = useState<Upload[] | null>(null);
  const [sources, setSources] = useState<Source[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [message, setMessage] = useState<{ tone: "success" | "error"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try {
      const [uploads, contentSources] = await Promise.all([
        api("/api/editorial/private-uploads"),
        api("/api/editorial/content-sources?status=approved"),
      ]);
      setItems(uploads.items ?? []);
      setSources((contentSources.items ?? []).filter((source: Source) => source.catalogueSyllabusId));
    } catch (error) {
      setItems([]);
      setMessage({ tone: "error", text: error instanceof Error ? error.message : "Could not load private uploads." });
    }
  };

  useEffect(() => {
    if (role === "admin") void load();
  }, [role]);

  if (role !== "admin") {
    return (
      <section className={styles.page}>
        <div className={styles.header}>
          <p className={styles.eyebrow}>Editorial intake</p>
          <h1>Private PDF intake</h1>
        </div>
        <Message tone="warning" title="Admin access required." role="alert">
          Private source intake is unavailable for this account.
        </Message>
      </section>
    );
  }

  const mutate = async (operation: () => Promise<unknown>, success: string) => {
    setBusy(true);
    setMessage(null);
    try {
      await operation();
      await load();
      setMessage({ tone: "success", text: success });
    } catch (error) {
      setMessage({ tone: "error", text: error instanceof Error ? error.message : "Request failed." });
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className={styles.page} aria-labelledby="private-intake-title">
      <header className={styles.header}>
        <p className={styles.eyebrow}>Editorial intake</p>
        <h1 id="private-intake-title">Private PDF intake</h1>
        <p className={styles.lede}>
          Files stay in private quarantine. Learners never receive upload access. Review cannot start until a scan-clean file is linked to an approved source.
        </p>
      </header>

      {message && <Message tone={message.tone} title={message.tone === "success" ? "Updated" : "Action failed"}>{message.text}</Message>}

      <form
        className={styles.uploadForm}
        onSubmit={(event) => {
          event.preventDefault();
          if (!file) {
            setMessage({ tone: "error", text: "Select a PDF before uploading." });
            return;
          }
          void mutate(async () => {
            const formData = new FormData();
            formData.set("file", file);
            await api("/api/editorial/private-uploads", { method: "POST", body: formData });
            setFile(null);
          }, "File moved into private quarantine.");
        }}
      >
        <div className={styles.formHeading}>
          <h2>Upload to quarantine</h2>
          <p>PDF only. Maximum file size: 25 MiB.</p>
        </div>
        <label className={styles.fileField} htmlFor="private-pdf-file">
          <span>PDF file</span>
          <input
            id="private-pdf-file"
            aria-label="PDF file"
            type="file"
            accept="application/pdf,.pdf"
            disabled={busy}
            onChange={(event) => setFile(event.target.files?.[0] ?? null)}
          />
          <small>{file ? `${file.name} selected` : "No file selected"}</small>
        </label>
        <Button type="submit" disabled={busy}>{busy ? "Working…" : "Upload to quarantine"}</Button>
      </form>

      <section className={styles.registry} aria-labelledby="upload-registry-title">
        <div className={styles.registryHeading}>
          <div>
            <p className={styles.eyebrow}>Private register</p>
            <h2 id="upload-registry-title">Uploaded files</h2>
          </div>
          {items !== null && <span className={styles.count}>{items.length} files</span>}
        </div>
        {items === null ? (
          <div className={styles.loading} role="status"><Skeleton width="38%" /><Skeleton width="88%" /><Skeleton width="66%" /><span>Loading uploads…</span></div>
        ) : items.length === 0 ? (
          <Message tone="neutral" title="No private uploads yet.">Upload a PDF when an approved source is ready for intake.</Message>
        ) : (
          <div className={styles.tableWrap}>
            <table>
              <caption className="sidus-visually-hidden">Private PDF uploads</caption>
              <thead><tr><th scope="col">File</th><th scope="col">Size</th><th scope="col">Status</th><th scope="col">Actions</th></tr></thead>
              <tbody>{items.map((item) => (
                <tr key={item.id}>
                  <td className={styles.filename}>{item.originalFilename}</td>
                  <td className={styles.mono}>{Math.ceil(item.byteSize / 1024)} KiB</td>
                  <td><span className={styles.status} data-status={item.status}>{uploadStatusLabel(item.status)}</span></td>
                  <td>
                    <div className={styles.actions}>
                      {item.status === "quarantined" && <span className={styles.muted}>Awaiting scanner attestation</span>}
                      {item.status === "scan_clean" && <>
                        <label className="sidus-visually-hidden" htmlFor={`source-${item.id}`}>Approved source for {item.originalFilename}</label>
                        <select id={`source-${item.id}`} value={selected[item.id] ?? ""} disabled={busy} onChange={(event) => setSelected({ ...selected, [item.id]: event.target.value })}>
                          <option value="">Choose approved source</option>
                          {sources.map((source) => <option key={source.id} value={source.id}>{source.title}</option>)}
                        </select>
                        <Button size="sm" disabled={busy || !selected[item.id]} onClick={() => void mutate(() => api(`/api/editorial/private-uploads/${item.id}/review-jobs`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ contentSourceId: selected[item.id] }) }), "AI review job queued.")}>Queue review</Button>
                      </>}
                      {item.status !== "deletion_requested" && <Button type="button" size="sm" variant="danger" disabled={busy} onClick={() => void mutate(() => api(`/api/editorial/private-uploads/${item.id}/deletion-request`, { method: "POST" }), "Deletion request recorded.")}>Request deletion</Button>}
                    </div>
                  </td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  );
}
