import { useState } from "react";
import { t } from "../i18n";
import { removeProgress } from "../api/reader";

const RemoveProgressButton = ({ bookId, title, onRemoved }) => {
  const [pending, setPending] = useState(false);
  const remove = async () => {
    if (pending) return;
    setPending(true);
    try {
      await removeProgress(bookId);
      onRemoved?.(bookId);
    } catch {
      alert(t("book.progress.removeFail"));
    } finally {
      setPending(false);
    }
  };

  return (
    <button type="button" className="btn btn-ghost" disabled={pending} onClick={remove}
      aria-label={title ? `${t("book.progress.remove")}: ${title}` : t("book.progress.remove")}>
      {t("book.progress.remove")}
    </button>
  );
};

export default RemoveProgressButton;
