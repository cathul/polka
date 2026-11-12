import { getLang } from "../i18n";
import api from "./api";

export const fetchReadMeta = (bookId) => api.getJson(`api/v1/read/${bookId}`);

export const fetchChapter = (bookId, n) => api.getJson(`api/v1/read/${bookId}/chapter/${n}`);

export const fetchProgress = (bookId, resolve = false) => api.getJson(`api/v1/read/${bookId}/progress${resolve ? "?resolve=1" : ""}`);

export const saveProgress = (bookId, chapter, position, progress, locator = "") =>
  fetch(api.buildUrl(`api/v1/read/${bookId}/progress`), {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", "X-Polka-Lang": getLang() },
    body: JSON.stringify({ chapter, position, progress, locator }),
  }).catch(() => {});

// Fallback when the server does not store the position (public mode, no login).
export const localProgress = {
  get(bookId) {
    try {
      return JSON.parse(localStorage.getItem(`polka-read-${bookId}`));
    } catch {
      return null;
    }
  },
  set(bookId, chapter, position) {
    try {
      localStorage.setItem(`polka-read-${bookId}`, JSON.stringify({ chapter, position }));
    } catch {
      /* quota exceeded — no big deal */
    }
  },
};

// Per-user reader preferences on the server (401 in public mode is fine —
// the caller falls back to localStorage).
export const fetchReaderPrefs = () => api.getJson("api/v1/me/reader-prefs");

export const saveReaderPrefs = (prefs) =>
  fetch(api.buildUrl("api/v1/me/reader-prefs"), {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(prefs),
  });

export const removeProgress = async (bookId) => {
  const res = await fetch(api.buildUrl(`api/v1/read/${bookId}/progress`), {
    method: "DELETE",
    credentials: "include",
    headers: { "X-Polka-Lang": getLang() },
  });
  if (!res.ok) throw new Error("Could not remove reading progress");
  const result = await res.json();
  if (result.removed !== true) throw new Error("Progress deletion was not confirmed");
  try {
    localStorage.removeItem(`polka-read-${bookId}`);
  } catch {
    // Server progress was removed even when browser storage is disabled.
  }
  return result;
};
