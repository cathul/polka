import { useEffect, useState } from "react";
import { t } from "../i18n";
import { fetchReaderEmail, setReaderEmail } from "../api/send";
import "./KosyncDialog.css";

const EmailSettingsDialog = ({ onClose }) => {
  const [email, setEmail] = useState("");
  const [ready, setReady] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    fetchReaderEmail().then((settings) => {
      if (cancelled) return;
      setEmail(settings.email ?? "");
      setReady(true);
    }).catch(() => { if (!cancelled) setError(t("email.settings.fail")); });
    return () => { cancelled = true; };
  }, []);

  const save = async (event) => {
    event.preventDefault();
    if (!ready || saving) return;
    setSaving(true);
    setError("");
    try {
      await setReaderEmail(email.trim());
      onClose();
    } catch {
      setError(t("email.settings.fail"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="kosync__overlay" onClick={onClose} role="presentation">
      <div className="kosync__card" onClick={(event) => event.stopPropagation()} role="dialog" aria-label={t("email.settings.title")}>
        <h2>{t("email.settings.title")}</h2>
        <p className="kosync__hint">{t("email.settings.hint")}</p>
        <form onSubmit={save}>
          <div className="kosync__row">
            <input type="email" aria-label={t("email.settings.address")} placeholder="reader@example.com"
              autoComplete="email" value={email} disabled={!ready || saving}
              onChange={(event) => setEmail(event.target.value)} />
          </div>
          {error && <p className="kosync__error" role="alert">{error}</p>}
          <div className="kosync__row kosync__row--close">
            <button type="submit" className="btn btn-primary" disabled={!ready || saving}>{t("kosync.save")}</button>
            <button type="button" className="btn btn-ghost" onClick={onClose}>{t("kosync.close")}</button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default EmailSettingsDialog;
