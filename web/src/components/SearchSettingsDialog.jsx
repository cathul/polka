import { useEffect, useState } from "react";
import { t } from "../i18n";
import { fetchSearchSettings, saveSearchSettings } from "../api/searchSettings";
import SearchLanguageSelect from "./SearchLanguageSelect";
import "./KosyncDialog.css";

const SearchSettingsDialog = ({ onClose }) => {
  const [language, setLanguage] = useState("");
  const [ready, setReady] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    let cancelled = false;
    fetchSearchSettings().then((settings) => {
      if (cancelled) return;
      setLanguage(settings.opdsSearchLanguage ?? "");
      setReady(true);
    }).catch(() => { if (!cancelled) setMessage(t("search.settings.fail")); });
    return () => { cancelled = true; };
  }, []);

  const save = async () => {
    if (!ready || saving) return;
    setSaving(true);
    setMessage("");
    try {
      await saveSearchSettings(language);
      onClose();
    } catch {
      setMessage(t("search.settings.fail"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="kosync__overlay" onClick={onClose} role="presentation">
      <div className="kosync__card" onClick={(event) => event.stopPropagation()} role="dialog" aria-label={t("search.settings")}>
        <h2>{t("search.settings")}</h2>
        <p className="kosync__hint">{t("search.opds.hint")}</p>
        <div className="kosync__row">
          <label>
            {t("search.opds.language")}{" "}
            <SearchLanguageSelect value={language} onChange={setLanguage} label={t("search.opds.language")} disabled={!ready || saving} />
          </label>
        </div>
        {message && <p className="kosync__error" role="alert">{message}</p>}
        <div className="kosync__row kosync__row--close">
          <button type="button" className="btn btn-primary" disabled={!ready || saving} onClick={save}>{t("kosync.save")}</button>
          <button type="button" className="btn btn-ghost" onClick={onClose}>{t("kosync.close")}</button>
        </div>
      </div>
    </div>
  );
};

export default SearchSettingsDialog;
