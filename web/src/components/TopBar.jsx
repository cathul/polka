import { t, currentLang, setLang } from "../i18n";
import KosyncDialog from "./KosyncDialog";
import SearchLanguageSelect from "./SearchLanguageSelect";
import SearchSettingsDialog from "./SearchSettingsDialog";
import EmailSettingsDialog from "./EmailSettingsDialog";
import { useEffect, useId, useRef, useState } from "react";
import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import "./TopBar.css";

const HeaderIcon = ({ kind }) => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    {kind === "reader" && <><path d="M4 4h13v16H4z" /><path d="M8 17h5M20 8v8" /></>}
    {kind === "search" && <><path d="M3 6h18M3 12h18M3 18h18" /><circle cx="8" cy="6" r="2" /><circle cx="16" cy="12" r="2" /><circle cx="8" cy="18" r="2" /></>}
    {kind === "email" && <><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m4 7 8 6 8-6" /></>}
    {kind === "logout" && <><path d="M9 4H4v16h5M10 12h11m-4-4 4 4-4 4" /></>}
  </svg>
);

const TopBar = ({ user, onLogout, desktop = false, sync = null }) => {
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const userMenuRef = useRef(null);
  const userButtonRef = useRef(null);
  const userMenuId = useId();
  const [emailSettingsOpen, setEmailSettingsOpen] = useState(false);
  const [searchSettingsOpen, setSearchSettingsOpen] = useState(false);
  const [kosyncOpen, setKosyncOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();

  const [searchLanguage, setSearchLanguage] = useState(searchParams.get("searchLanguage") ?? "");
  const [query, setQuery] = useState(searchParams.get("q") ?? "");

  useEffect(() => {
    setQuery(searchParams.get("q") ?? "");
    setSearchLanguage(searchParams.get("searchLanguage") ?? "");
  }, [searchParams]);

  useEffect(() => {
    if (!userMenuOpen) return undefined;
    const outside = (event) => {
      if (!userMenuRef.current?.contains(event.target)) setUserMenuOpen(false);
    };
    const escape = (event) => {
      if (event.key !== "Escape") return;
      setUserMenuOpen(false);
      userButtonRef.current?.focus();
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", escape);
    };
  }, [userMenuOpen]);

  useEffect(() => { setUserMenuOpen(false); }, [location.pathname]);

  const openSettings = (kind) => {
    setUserMenuOpen(false);
    setKosyncOpen(kind === "reader");
    setSearchSettingsOpen(kind === "search");
    setEmailSettingsOpen(kind === "email");
  };

  const submit = (event) => {
    event.preventDefault();
    const trimmed = query.trim();
    if (trimmed.length < 3) return;
    const params = new URLSearchParams({ q: trimmed });
    if (searchLanguage) params.set("searchLanguage", searchLanguage);
    navigate(`/search?${params}`);
  };

  const settingsAvailable = !desktop && !sync;
  const menuAvailable = settingsAvailable || Boolean(onLogout);
  const userName = desktop ? t("owner") : user?.displayName || user?.login || "";

  const isActive = (path) =>
    path === "/" ? location.pathname === "/" : location.pathname.startsWith(path);

  return (
    <header className="topbar">
      <div className="container topbar__row">
        <Link to="/" className="topbar__brand" aria-label={t("brand")}>
          <img className="topbar__brand-mark" src="/icons/logo.png" alt="" aria-hidden="true" />
          <span className="topbar__brand-text">
            <span className="topbar__brand-name">{t("brand")}</span>
          </span>
        </Link>

        <nav className="topbar__nav" aria-label="nav">
          <Link to="/" className={`topbar__nav-link ${isActive("/") ? "is-active" : ""}`}>
            {t("nav.home")}
          </Link>
          <Link
            to="/search?scope=bookTitles"
            className={`topbar__nav-link ${isActive("/search") ? "is-active" : ""}`}
          >
            {t("nav.catalog")}
          </Link>
          {user && (
            <Link to="/lists" className={`topbar__nav-link ${isActive("/lists") ? "is-active" : ""}`}>
              {t("nav.lists")}
            </Link>
          )}
          {(sync || desktop) && (
            <Link to="/sync" className={`topbar__nav-link ${isActive("/sync") ? "is-active" : ""}`}>
              {t("nav.server")}
              {sync && !sync.online && <span className="topbar__offline-dot" title={t("nav.serverDown")} />}
            </Link>
          )}
          {user?.role === "admin" && (
            <>
              <Link
                to="/admin/library"
                className={`topbar__nav-link ${isActive("/admin/library") ? "is-active" : ""}`}
              >
                {t("nav.manage")}
              </Link>
              {!desktop && (
                <Link
                  to="/admin/users"
                  className={`topbar__nav-link ${isActive("/admin/users") ? "is-active" : ""}`}
                >
                  {t("nav.users")}
                </Link>
              )}
            </>
          )}
        </nav>

        <button
          type="button"
          className="topbar__lang"
          title="Language"
          onClick={() => setLang(currentLang() === "ru" ? "en" : "ru")}
        >
          {currentLang() === "ru" ? "EN" : "RU"}
        </button>

        {user && (
          <div className="topbar__user" ref={userMenuRef} onBlur={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget)) setUserMenuOpen(false);
          }}>
            <button type="button" className="topbar__user-name" ref={userButtonRef} title={user.login}
              aria-label={`${t("nav.userMenu")}: ${userName}`} aria-expanded={menuAvailable ? userMenuOpen : undefined}
              aria-controls={menuAvailable ? userMenuId : undefined} disabled={!menuAvailable}
              onClick={() => setUserMenuOpen((open) => !open)}>
              <span className="topbar__avatar" aria-hidden="true">{Array.from(userName)[0]?.toUpperCase()}</span>
              <span className="topbar__user-label">{userName}</span>
              {menuAvailable && <svg className="topbar__user-chevron" width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true"><path d="m3 4 3 3 3-3" /></svg>}
            </button>
            {userMenuOpen && menuAvailable && (
              <div className="topbar__user-menu" id={userMenuId}>
                <ul aria-label={t("nav.userMenu")}>
                  {settingsAvailable && <>
                    <li><button type="button" className="topbar__user-item" onClick={() => openSettings("reader")}>
                      <HeaderIcon kind="reader" /><span>{t("kosync.menu")}</span><span className="topbar__item-arrow" aria-hidden="true">›</span>
                    </button></li>
                    <li><button type="button" className="topbar__user-item" onClick={() => openSettings("search")}>
                      <HeaderIcon kind="search" /><span>{t("search.opds.menu")}</span><span className="topbar__item-arrow" aria-hidden="true">›</span>
                    </button></li>
                    <li><button type="button" className="topbar__user-item" title="Send to Email" onClick={() => openSettings("email")}>
                      <HeaderIcon kind="email" /><span>Send to Email</span><span className="topbar__item-arrow" aria-hidden="true">›</span>
                    </button></li>
                  </>}
                  {onLogout && <>
                    {settingsAvailable && <li className="topbar__user-divider" role="separator" />}
                    <li><button type="button" className="topbar__user-item topbar__user-item--logout" onClick={() => { setUserMenuOpen(false); onLogout(); }}>
                      <HeaderIcon kind="logout" /><span>{t("nav.logout")}</span>
                    </button></li>
                  </>}
                </ul>
                {settingsAvailable && <p className="topbar__user-menu-hint">{t("nav.accountSettings")}</p>}
              </div>
            )}
          </div>
        )}
      </div>
      <div className="container topbar__search-row">
        <form
          className="topbar__search"
          role="search"
          onSubmit={submit}
        >
          <span className="topbar__search-icon" aria-hidden="true">
            🔍
          </span>
          <input
            type="search"
            className="topbar__search-input"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("search.placeholder")}
            aria-label={t("search.aria")}
          />
          <SearchLanguageSelect value={searchLanguage} onChange={(language) => {
            setSearchLanguage(language);
            if (location.pathname === "/search" && searchParams.get("q")) {
              const params = new URLSearchParams(searchParams);
              if (language) params.set("searchLanguage", language);
              else params.delete("searchLanguage");
              navigate(`/search?${params}`, { replace: true });
            }
          }} />
          <button
            type="submit"
            className="topbar__search-submit"
            disabled={query.trim().length < 3}
          >
            {t("search.go")}
          </button>
        </form>
      </div>
      {emailSettingsOpen && <EmailSettingsDialog onClose={() => setEmailSettingsOpen(false)} />}
      {searchSettingsOpen && <SearchSettingsDialog onClose={() => setSearchSettingsOpen(false)} />}
      {kosyncOpen && <KosyncDialog onClose={() => setKosyncOpen(false)} />}
    </header>
  );
};

export default TopBar;
