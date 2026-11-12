import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import ReaderPage from "./ReaderPage";
import { fetchChapter, fetchReadMeta, fetchProgress, fetchReaderPrefs, saveProgress } from "../api/reader";
import { restoreChapterPosition, restorePdfPage } from "./readerPosition";

vi.mock("../api/reader", () => ({
  fetchChapter: vi.fn(), fetchReadMeta: vi.fn(), fetchProgress: vi.fn(), fetchReaderPrefs: vi.fn(),
  saveProgress: vi.fn(), saveReaderPrefs: vi.fn(), localProgress: { get: vi.fn(), set: vi.fn() },
}));

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("restores overall-only and legacy KOReader progress without using stale web coordinates", () => {
  expect(restoreChapterPosition({ chapter: 0, position: 0, progress: 0.03 }, 100)).toEqual({ index: 3, ratio: 0 });
  expect(restoreChapterPosition({ chapter: 20, position: 0.5, progress: 0.03, locator: "kosync:overall" }, 100)).toEqual({ index: 3, ratio: 0 });
  expect(restoreChapterPosition({ chapter: 20, position: 0.5, progress: 0.205 }, 100)).toEqual({ index: 20, ratio: 0.5 });
  expect(restoreChapterPosition({ chapter: 2, position: 0.5, progress: 0 }, 100)).toEqual({ index: 2, ratio: 0.5 });
  expect(restoreChapterPosition({ chapter: 4, position: 0.25, progress: 0.03, anchor: { chapter: 4 } }, 100)).toEqual({ index: 4, ratio: 0.25 });
  expect(restorePdfPage({ chapter: 1, progress: 0.03, locator: "kosync:overall" }, 200)).toBe(6);
});

it("opening a KOReader book restores 3% and never writes until the reader deliberately scrolls", async () => {
  vi.stubGlobal("IntersectionObserver", class { observe() {} disconnect() {} });
  vi.spyOn(window, "scrollTo").mockImplementation(() => {});
  vi.spyOn(HTMLElement.prototype, "offsetTop", "get").mockReturnValue(100);
  vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(1000);
  fetchReadMeta.mockResolvedValue({ title: "KOReader book", chapters: Array.from({ length: 100 }, (_, index) => ({ index, title: `Chapter ${index}` })) });
  fetchProgress.mockResolvedValue({ stored: true, exists: true, chapter: 0, position: 0, progress: 0.03 });
  fetchReaderPrefs.mockResolvedValue({});
  fetchChapter.mockImplementation(async (_, index) => ({ index, html: "<p>Book text</p>" }));
  saveProgress.mockResolvedValue({});
  render(<MemoryRouter initialEntries={["/read/7"]}><Routes><Route path="/read/:bookId" element={<ReaderPage />} /></Routes></MemoryRouter>);
  await screen.findByText("Book text");
  expect(fetchChapter).toHaveBeenCalledWith("7", 3);
  expect(screen.getByText("3%")).toBeInTheDocument();
  fireEvent.scroll(window);
  fireEvent(document, new Event("visibilitychange"));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 1400)); });
  expect(saveProgress).not.toHaveBeenCalled();
  fireEvent.wheel(window, { deltaY: 100 });
  fireEvent.scroll(window);
  await waitFor(() => expect(saveProgress).toHaveBeenCalled());
  expect(saveProgress.mock.calls[0][0]).toBe("7");
});
