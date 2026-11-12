import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { t } from "../i18n";
import BookPage from "./BookPage";
import TopBar from "../components/TopBar";
import { fetchBookForm, fetchExternalEnrichment, fetchSimilarBooks } from "../api/fetchBooks";
import { fetchProgress, removeProgress } from "../api/reader";
import { fetchReaderEmail, setReaderEmail, sendBook } from "../api/send";

vi.mock("../api/fetchBooks", () => ({ fetchBookForm: vi.fn(), fetchExternalEnrichment: vi.fn(), fetchSimilarBooks: vi.fn() }));
vi.mock("../api/reader", () => ({ fetchProgress: vi.fn(), removeProgress: vi.fn() }));
vi.mock("../api/send", () => ({ fetchReaderEmail: vi.fn(), setReaderEmail: vi.fn(), sendBook: vi.fn() }));
vi.mock("../api/lists", () => ({ fetchLists: vi.fn().mockResolvedValue({ lists: [] }), toggleWishlist: vi.fn() }));

afterEach(() => vi.restoreAllMocks());

beforeEach(() => {
  vi.clearAllMocks();
  vi.spyOn(window, "scrollTo").mockImplementation(() => {});
  fetchBookForm.mockResolvedValue({ bookForm: { BookID: 7, Title: "Test book", Ext: ".fb2" } });
  fetchExternalEnrichment.mockResolvedValue(null);
  fetchSimilarBooks.mockResolvedValue(null);
  fetchProgress.mockResolvedValue({ stored: true, exists: true, progress: 0.37 });
  fetchReaderEmail.mockResolvedValue({ smtpReady: true, conversionEnabled: true, email: "reader@example.org" });
  removeProgress.mockResolvedValue({ removed: true });
  sendBook.mockResolvedValue({});
});

const renderBook = (user = { role: "admin" }) => render(
  <MemoryRouter initialEntries={["/book/7"]}>
    <TopBar user={user} />
    <Routes><Route path="/book/:bookId" element={<BookPage user={user} />} /></Routes>
  </MemoryRouter>
);

it("links progress to reading, sends the selected format and keeps personal removal on the book page", async () => {
  renderBook();
  const progress = await screen.findByRole("progressbar");
  expect(progress).toHaveAttribute("aria-valuenow", "37");
  expect(screen.getByRole("link", { name: t("book.continue", { p: 37 }) })).toHaveAttribute("href", "/read/7");
  const actions = document.querySelector(".book-page__actions");
  expect(actions.firstElementChild).toHaveClass("book-page__send");
  expect(actions.querySelector("a[href*='Images']")).toBeNull();
  expect(screen.getByRole("button", { name: t("book.delete") })).toHaveTextContent(t("book.delete"));
  fireEvent.change(screen.getByRole("combobox", { name: t("book.send.format") }), { target: { value: "epub3" } });
  fireEvent.click(screen.getByRole("button", { name: t("book.send") }));
  await waitFor(() => expect(sendBook).toHaveBeenCalledWith("7", "epub3"));
  fireEvent.click(screen.getByRole("button", { name: t("book.progress.remove") }));
  await waitFor(() => expect(screen.queryByRole("progressbar")).not.toBeInTheDocument());
  expect(removeProgress).toHaveBeenCalledWith("7");
  expect(screen.getByRole("link", { name: t("book.read") })).toHaveAttribute("href", "/read/7");
  expect(fetchProgress).toHaveBeenCalledTimes(1);
});

it("shows unread books without progress, admin controls, SMTP or an invalid compact download", async () => {
  fetchProgress.mockResolvedValue({ stored: true, exists: false, progress: 0 });
  fetchReaderEmail.mockResolvedValue({ smtpReady: false });
  fetchBookForm.mockResolvedValue({ bookForm: { BookID: 7, Title: "Unread EPUB", Ext: ".epub" } });
  renderBook({ role: "user" });
  await screen.findByRole("heading", { name: "Unread EPUB" });
  expect(screen.getByRole("link", { name: t("book.read") })).toHaveAttribute("href", "/read/7");
  expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: t("book.progress.remove") })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: t("book.delete") })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: t("book.send") })).not.toBeInTheDocument();
  expect(document.querySelector('a[href*="fb2compact"]')).toBeNull();
});

it("uses an address set and changed in the header without prompting or reloading the book", async () => {
  let address = "";
  const deliveredTo = [];
  fetchReaderEmail.mockImplementation(async () => ({ smtpReady: true, email: address }));
  setReaderEmail.mockImplementation(async (value) => { address = value; return { email: address }; });
  sendBook.mockImplementation(async () => { deliveredTo.push(address); });
  const prompt = vi.spyOn(window, "prompt");
  renderBook({ role: "user", login: "reader" });
  await screen.findByRole("button", { name: t("book.send") });
  fireEvent.click(screen.getByRole("button", { name: `${t("nav.userMenu")}: reader` }));
  const settings = screen.getByRole("button", { name: "Send to Email" });
  expect(settings).toHaveAttribute("title", "Send to Email");
  expect(settings).toHaveTextContent("Send to Email");
  fireEvent.click(settings);
  const input = await screen.findByRole("textbox", { name: t("email.settings.address") });
  await waitFor(() => expect(input).toBeEnabled());
  fireEvent.change(input, { target: { value: "first@example.org" } });
  fireEvent.click(screen.getByRole("button", { name: t("kosync.save") }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  fireEvent.click(screen.getByRole("button", { name: t("book.send") }));
  await waitFor(() => expect(deliveredTo).toEqual(["first@example.org"]));

  fireEvent.click(screen.getByRole("button", { name: `${t("nav.userMenu")}: reader` }));
  fireEvent.click(screen.getByRole("button", { name: "Send to Email" }));
  const changed = await screen.findByRole("textbox", { name: t("email.settings.address") });
  await waitFor(() => expect(changed).toHaveValue("first@example.org"));
  fireEvent.change(changed, { target: { value: "second@example.org" } });
  fireEvent.click(screen.getByRole("button", { name: t("kosync.save") }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  fireEvent.click(screen.getByRole("button", { name: t("book.send.done") }));
  await waitFor(() => expect(deliveredTo).toEqual(["first@example.org", "second@example.org"]));
  expect(prompt).not.toHaveBeenCalled();
});

it("shows an address saved by the first-delivery prompt in the header setting", async () => {
  let address = "";
  fetchReaderEmail.mockImplementation(async () => ({ smtpReady: true, email: address }));
  setReaderEmail.mockImplementation(async (value) => { address = value; return { email: address }; });
  vi.spyOn(window, "prompt").mockReturnValue("prompt@example.org");
  renderBook({ role: "user", login: "reader" });
  fireEvent.click(await screen.findByRole("button", { name: t("book.send") }));
  await waitFor(() => expect(setReaderEmail).toHaveBeenCalledWith("prompt@example.org"));
  await screen.findByRole("button", { name: t("book.send.done") });
  fireEvent.click(screen.getByRole("button", { name: `${t("nav.userMenu")}: reader` }));
  fireEvent.click(screen.getByRole("button", { name: "Send to Email" }));
  const input = await screen.findByRole("textbox", { name: t("email.settings.address") });
  await waitFor(() => expect(input).toHaveValue("prompt@example.org"));
});
