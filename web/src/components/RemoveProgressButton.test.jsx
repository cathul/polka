import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import RemoveProgressButton from "./RemoveProgressButton";
import { t } from "../i18n";

vi.mock("../api/reader", () => ({ removeProgress: vi.fn() }));
import { removeProgress } from "../api/reader";

describe("RemoveProgressButton", () => {
  it("waits for successful deletion and disables repeated clicks", async () => {
    let finish;
    removeProgress.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const onRemoved = vi.fn();
    render(<RemoveProgressButton bookId={7} onRemoved={onRemoved} />);
    const button = screen.getByRole("button");
    fireEvent.click(button);
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(removeProgress).toHaveBeenCalledTimes(1);
    expect(onRemoved).not.toHaveBeenCalled();
    finish({ removed: true });
    await waitFor(() => expect(onRemoved).toHaveBeenCalledWith(7));
  });
  it("keeps the displayed progress when deletion fails", async () => {
    removeProgress.mockRejectedValue(new Error("offline"));
    const alert = vi.spyOn(window, "alert").mockImplementation(() => {});
    const onRemoved = vi.fn();
    render(<RemoveProgressButton bookId={8} onRemoved={onRemoved} />);
    fireEvent.click(screen.getByRole("button"));
    await waitFor(() => expect(alert).toHaveBeenCalledWith(t("book.progress.removeFail")));
    expect(onRemoved).not.toHaveBeenCalled();
    expect(screen.getByRole("button")).not.toBeDisabled();
    alert.mockRestore();
  });
});
