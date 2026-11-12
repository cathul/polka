import { t } from "./i18n";
import { useCallback, useEffect, useState } from "react";
import { HashRouter, Navigate, Route, Routes } from "react-router-dom";
import TopBar from "./components/TopBar";
import HomePage from "./pages/HomePage";
import BookPage from "./pages/BookPage";
import BookListPage from "./pages/BookListPage";
import SearchPage from "./pages/SearchPage";
import LoginPage from "./pages/LoginPage";
import AdminUsersPage from "./pages/AdminUsersPage";
import AdminLibraryPage from "./pages/AdminLibraryPage";
import ReaderPage from "./pages/ReaderPage";
import ListsPage from "./pages/ListsPage";
import SyncPage from "./pages/SyncPage";
import { fetchConfig } from "./api/fetchBooks";
import { fetchMe, logout } from "./api/auth";
import "./theme.css";
import "./Root.css";

const Root = () => {
  const [config, setConfig] = useState(null);
  const [user, setUser] = useState(null);
  const [authRequired, setAuthRequired] = useState(false);
  const [isDesktop, setIsDesktop] = useState(false);
  const [sync, setSync] = useState(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [version, setVersion] = useState("");

  const loadConfig = useCallback(() => {
    fetchConfig()
      .then(setConfig)
      .catch(() => setConfig(null));
  }, []);

  useEffect(() => {
    fetchMe()
      .then((res) => {
        setUser(res.user ?? null);
        setAuthRequired(Boolean(res.authRequired));
        setIsDesktop(Boolean(res.desktop));
        setSync(res.sync ?? null);
        setVersion(res.version ?? "");
      })
      .catch(() => {
        setUser(null);
        setAuthRequired(false);
      })
      .finally(() => setAuthChecked(true));
  }, []);

  useEffect(() => {
    if (!authChecked) return;
    if (!authRequired || user) loadConfig();
  }, [authChecked, authRequired, user, loadConfig]);

  const handleLogout = () => {
    logout().finally(() => {
      setUser(null);
      setConfig(null);
    });
  };

  if (!authChecked) {
    return <div style={{ padding: 48, textAlign: "center" }}>{t("loading")}</div>;
  }

  if (authRequired && !user) {
    return <LoginPage onLogin={setUser} />;
  }

  const layout = (children) => (
    <>
      <TopBar
        user={user}
        desktop={isDesktop}
        sync={sync}
        onLogout={isDesktop ? undefined : handleLogout}
      />
      <main className="root-main">{children}</main>
      <footer className="root-footer">
        <div className="container root-footer__row">
          <span className="root-footer__identity">
            {t("footer")}
            {version && <span className="root-footer__version"> · v.{version}</span>}
          </span>
          <a className="root-footer__github" href="https://github.com/cathul/polka" target="_blank" rel="noopener noreferrer"
            aria-label="GitHub · cathul/polka" title="GitHub · cathul/polka">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M12 .75a11.25 11.25 0 0 0-3.558 21.923c.563.104.768-.244.768-.542 0-.267-.01-1.156-.015-2.097-3.13.68-3.79-1.327-3.79-1.327-.512-1.3-1.25-1.646-1.25-1.646-1.022-.699.077-.685.077-.685 1.13.08 1.724 1.16 1.724 1.16 1.005 1.723 2.636 1.225 3.278.937.103-.729.393-1.225.715-1.507-2.499-.284-5.126-1.25-5.126-5.56 0-1.228.439-2.232 1.16-3.02-.117-.285-.503-1.43.11-2.98 0 0 .944-.302 3.094 1.153a10.789 10.789 0 0 1 2.813-.378c.956.005 1.918.13 2.813.378 2.148-1.455 3.09-1.153 3.09-1.153.615 1.55.23 2.695.114 2.98.723.788 1.158 1.792 1.158 3.02 0 4.322-2.632 5.273-5.139 5.552.404.35.764 1.038.764 2.09 0 1.51-.014 2.728-.014 3.1 0 .301.203.651.774.54A11.252 11.252 0 0 0 12 .75Z" />
            </svg>
          </a>
        </div>
      </footer>
    </>
  );

  return (
    <HashRouter>
      <Routes>
        <Route path="/" element={layout(<HomePage config={config} />)} />
        <Route path="/search" element={layout(<SearchPage config={config} />)} />
        <Route path="/lists" element={user ? layout(<ListsPage />) : <Navigate to="/" replace />} />
        <Route path="/book/:bookId" element={layout(<BookPage user={user} sync={sync} />)} />
        {(sync || isDesktop) && <Route path="/sync" element={layout(<SyncPage />)} />}
        {/* Читалка — без общей шапки, у неё свой минимальный интерфейс */}
        <Route path="/read/:bookId" element={<ReaderPage />} />
        <Route path="/author/:authorId" element={layout(<BookListPage kind="author" />)} />
        <Route path="/series/:seriesId" element={layout(<BookListPage kind="series" />)} />
        <Route path="/shelf/:shelfId" element={layout(<BookListPage kind="shelf" />)} />
        <Route
          path="/admin/users"
          element={
            user?.role === "admin" && !isDesktop
              ? layout(<AdminUsersPage me={user} />)
              : <Navigate to="/" replace />
          }
        />
        <Route
          path="/admin/library"
          element={
            user?.role === "admin" ? layout(<AdminLibraryPage />) : <Navigate to="/" replace />
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </HashRouter>
  );
};

export default Root;
