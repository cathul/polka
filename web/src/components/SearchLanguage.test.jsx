import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { t } from "../i18n";
import TopBar from "./TopBar";
import SearchSettingsDialog from "./SearchSettingsDialog";
import { fetchSearchSettings, saveSearchSettings } from "../api/searchSettings";

vi.mock("../api/searchSettings", () => ({ fetchSearchSettings: vi.fn(), saveSearchSettings: vi.fn() }));

const Location = () => <output data-testid="location">{useLocation().search}</output>;

describe("Independent search language controls", () => {
  it("uses a single language for the current web search and keeps it for a new query", () => {
    render(<MemoryRouter initialEntries={["/search?q=Толкин&scope=authors"]}><TopBar /><Location /></MemoryRouter>);
    const select = screen.getByRole("combobox", { name: t("search.language") });
    expect(select.multiple).toBe(false);
    fireEvent.change(select, { target: { value: "en" } });
    expect(screen.getByTestId("location").textContent).toContain("searchLanguage=en");
    expect(screen.getByTestId("location").textContent).toContain("scope=authors");
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "Новый запрос" } });
    fireEvent.submit(screen.getByRole("search"));
    expect(screen.getByTestId("location").textContent).toContain("searchLanguage=en");
    expect(screen.getByTestId("location").textContent).not.toContain("scope=authors");
    expect(saveSearchSettings).not.toHaveBeenCalled();
  });

  it("loads and saves the separate personal OPDS preference", async () => {
    fetchSearchSettings.mockResolvedValue({ opdsSearchLanguage: "uk" });
    saveSearchSettings.mockResolvedValue({ opdsSearchLanguage: "ru" });
    const onClose = vi.fn();
    render(<SearchSettingsDialog onClose={onClose} />);
    const select = screen.getByRole("combobox", { name: t("search.opds.language") });
    await waitFor(() => expect(select).toHaveValue("uk"));
    fireEvent.change(select, { target: { value: "ru" } });
    fireEvent.click(screen.getByRole("button", { name: t("kosync.save") }));
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(screen.queryByText(t("search.settings.saved"))).not.toBeInTheDocument();
    expect(saveSearchSettings).toHaveBeenCalledWith("ru");
  });
});

it("keeps OPDS settings open and shows only an error when saving fails", async () => {
  fetchSearchSettings.mockResolvedValue({ opdsSearchLanguage: "uk" });
  saveSearchSettings.mockRejectedValueOnce(new Error("failed"));
  const onClose = vi.fn();
  render(<SearchSettingsDialog onClose={onClose} />);
  const save = screen.getByRole("button", { name: t("kosync.save") });
  await waitFor(() => expect(save).toBeEnabled());
  fireEvent.click(save);
  expect(await screen.findByRole("alert")).toHaveTextContent(t("search.settings.fail"));
  expect(onClose).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: t("kosync.close") }));
  expect(onClose).toHaveBeenCalledOnce();
});
