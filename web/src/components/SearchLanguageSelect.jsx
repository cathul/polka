import { t } from "../i18n";

const SearchLanguageSelect = ({ value, onChange, label = t("search.language"), disabled = false }) => (
  <select aria-label={label} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
    <option value="">{t("search.language.all")}</option>
    <option value="ru">Русский</option>
    <option value="uk">Українська</option>
    <option value="en">English</option>
  </select>
);

export default SearchLanguageSelect;
