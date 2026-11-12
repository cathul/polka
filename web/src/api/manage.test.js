import { afterEach, describe, expect, it, vi } from "vitest";
import { importInpx } from "./manage";

afterEach(() => vi.unstubAllGlobals());

describe("INPX import source", () => {
  it("uploads the selected file without a remembered server path", async () => {
    const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ started: true }) });
    vi.stubGlobal("fetch", fetch);
    const file = new File(["test"], "new.inpx");
    await importInpx({ file, path: "/books/old.inpx", replace: true });
    const form = fetch.mock.calls[0][1].body;
    expect(form.get("file")).toBe(file);
    expect(form.has("path")).toBe(false);
    expect(form.get("replace")).toBe("1");
  });
  it("uses the edited server path when no file is selected", async () => {
    const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ started: true }) });
    vi.stubGlobal("fetch", fetch);
    await importInpx({ path: "/books/changed.inpx" });
    expect(fetch.mock.calls[0][1].body.get("path")).toBe("/books/changed.inpx");
  });
});
