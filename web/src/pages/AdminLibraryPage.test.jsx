import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { t } from "../i18n";
import AdminLibraryPage from "./AdminLibraryPage";

vi.mock("../api/manage", () => ({
 fetchSettings: vi.fn(), importStatus: vi.fn(), importInpx: vi.fn(),
 saveSettings: vi.fn(), testSmtp: vi.fn(), uploadBooks: vi.fn(),
}));
vi.mock("../api/collections", () => ({
 fetchCollections: vi.fn().mockResolvedValue({ collections: [] }),
 deleteCollection: vi.fn(), importCollection: vi.fn(), rematchCollection: vi.fn(), syncCollectionSource: vi.fn(),
}));
import { fetchSettings, importStatus, importInpx } from "../api/manage";

const settings = { enrichment: {}, similar: {}, smtp: {}, lastInpxPath: "/books/saved.inpx" };
const open = () => render(<MemoryRouter><AdminLibraryPage /></MemoryRouter>);

beforeEach(() => {
 vi.clearAllMocks();
 fetchSettings.mockResolvedValue(settings);
 importStatus.mockResolvedValue({ running: false, phase: "done", books: 1, authors: 1, series: 0 });
 importInpx.mockResolvedValue({ started: true });
});

describe("remembered INPX path", () => {
 it("prefills the saved path, submits an edited path and keeps it visible", async () => {
  open();
  const field = screen.getByPlaceholderText(t("admin.import.pathPlaceholder"));
  await waitFor(() => expect(field).toHaveValue("/books/saved.inpx"));
  fireEvent.change(field, { target: { value: "/books/new.inpx" } });
  fireEvent.click(screen.getByRole("button", { name: t("admin.import.go") }));
  await waitFor(() => expect(importInpx).toHaveBeenCalledWith({ file: null, path: "/books/new.inpx", replace: false }));
  expect(field).toHaveValue("/books/new.inpx");
 });
 it("does not overwrite input typed before settings finish loading", async () => {
  let resolveSettings;
  fetchSettings.mockReturnValue(new Promise((resolve) => { resolveSettings = resolve; }));
  open();
  const field = screen.getByPlaceholderText(t("admin.import.pathPlaceholder"));
  fireEvent.change(field, { target: { value: "/books/manual.inpx" } });
  resolveSettings(settings);
  await waitFor(() => expect(screen.getByPlaceholderText(t("admin.smtp.host"))).toBeInTheDocument());
  expect(field).toHaveValue("/books/manual.inpx");
 });
});
