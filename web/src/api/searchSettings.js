import api from "./api";

export const fetchSearchSettings = () => api.getJson("api/v1/me/search-settings");

export const saveSearchSettings = async (opdsSearchLanguage) => {
  const response = await fetch(api.buildUrl("api/v1/me/search-settings"), {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ opdsSearchLanguage }),
  });
  if (!response.ok) throw new Error(`Request failed: ${response.status}`);
  return response.json();
};
