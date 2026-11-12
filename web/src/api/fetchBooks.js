import api from "./api";

const buildQuery = (params) => {
  const usp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    usp.set(key, String(value));
  }
  const qs = usp.toString();
  return qs ? `?${qs}` : "";
};

const get = (endpoint, params = {}) => api.getJson(`main/getBooks/${endpoint}${buildQuery(params)}`);

export const fetchConfig = () => get("getConfig");

export const fetchHomeShelves = ({ limit } = {}) => get("getHomeShelves", { limit });

export const fetchCatalogShelves = ({ limit, count } = {}) =>
  get("getCatalogShelves", { limit, count });

export const fetchShelfBooks = ({ shelfId, limit, offset } = {}) =>
  get("getShelfBooks", { shelfId, limit, offset });

export const fetchSearchStats = ({ search, selectedGroupID, searchLanguage } = {}) => get("getSearchStats", { search, selectedGroupID, searchLanguage });

export const fetchSearchTitles = ({ search, selectedGroupID, searchLanguage } = {}) => get("getSearchTitles", { search, selectedGroupID, searchLanguage });

export const fetchSearchAuthors = ({ search, selectedGroupID, searchLanguage } = {}) => get("getSearchAuthors", { search, selectedGroupID, searchLanguage });

export const fetchSearchSeries = ({ search, selectedGroupID, searchLanguage } = {}) => get("getSearchSeries", { search, selectedGroupID, searchLanguage });

export const fetchSearchGenres = ({ search } = {}) => get("getSearchGenres", { search });

export const fetchAuthorBooks = ({ selectedItemID, selectedGroupID, searchLanguage } = {}) =>
  get("getSearchAuthorBooks", { selectedItemID, selectedGroupID, searchLanguage });

export const fetchSeriesBooks = ({ selectedItemID, selectedGroupID, searchLanguage } = {}) =>
  get("getSearchSeriesBooks", { selectedItemID, selectedGroupID, searchLanguage });

export const fetchBookForm = ({ selectedItemID } = {}) => get("getBookForm", { selectedItemID });

export const fetchExternalEnrichment = ({ bookId, title, author, isbn } = {}) =>
  get("getExternalEnrichment", { bookId, title, author, isbn });

export const fetchSimilarBooks = ({ bookId, title, author } = {}) =>
  get("getSimilarBooks", { bookId, title, author });
