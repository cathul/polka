import { expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { t } from "../i18n";
import TopBar from "./TopBar";

vi.mock("./SearchSettingsDialog", () => ({ default: () => <div role="dialog">OPDS settings</div> }));

const renderHeader = (props = {}) => render(<MemoryRouter><TopBar user={{ login: "cathul" }} {...props} /></MemoryRouter>);

it("opens account actions from the user button and closes on Escape and outside click", () => {
  renderHeader();
  const toggle = screen.getByRole("button", { name: `${t("nav.userMenu")}: cathul` });
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  expect(screen.queryByRole("button", { name: "Send to Email" })).not.toBeInTheDocument();
  fireEvent.click(toggle);
  expect(screen.getByRole("button", { name: "Send to Email" })).toBeInTheDocument();
  fireEvent.keyDown(document, { key: "Escape" });
  expect(toggle).toHaveFocus();
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  fireEvent.click(toggle);
  fireEvent.pointerDown(document.body);
  expect(toggle).toHaveAttribute("aria-expanded", "false");
});

it("closes the dropdown before opening existing settings and keeps logout inside it", () => {
  const onLogout = vi.fn();
  renderHeader({ onLogout });
  const toggle = screen.getByRole("button", { name: `${t("nav.userMenu")}: cathul` });
  expect(screen.queryByRole("button", { name: t("nav.logout") })).not.toBeInTheDocument();
  fireEvent.click(toggle);
  fireEvent.click(screen.getByRole("button", { name: t("search.opds.menu") }));
  expect(screen.getByRole("dialog")).toHaveTextContent("OPDS settings");
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  fireEvent.click(toggle);
  fireEvent.click(screen.getByRole("button", { name: t("nav.logout") }));
  expect(onLogout).toHaveBeenCalledOnce();
  expect(toggle).toHaveAttribute("aria-expanded", "false");
});
