const AUTH_TOKEN_KEY = 'token';

export const getAuthToken = (): string | null => {
  try {
    const token = localStorage.getItem(AUTH_TOKEN_KEY);
    if (!token) return null;
    const normalized = token.trim();
    return normalized || null;
  } catch {
    return null;
  }
};

export const setAuthToken = (token: string) => {
  const normalized = token.trim();
  if (!normalized) return;
  localStorage.setItem(AUTH_TOKEN_KEY, normalized);
};

export const clearAuthToken = () => {
  localStorage.removeItem(AUTH_TOKEN_KEY);
};
