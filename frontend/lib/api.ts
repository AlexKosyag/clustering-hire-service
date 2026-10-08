import axios, { AxiosError } from 'axios';

/**
 * Базовый адрес API.
 *  - по умолчанию '/api' — same-origin запрос, проксируется Next (rewrites в next.config.js)
 *    на backend: работает и в dev, и в Docker, и за nginx — CORS не нужен;
 *  - можно переопределить NEXT_PUBLIC_API_URL='http://host:3000' для прямых запросов.
 */
function resolveApiUrl(): string {
  const raw = process.env.NEXT_PUBLIC_API_URL;
  if (!raw) return '/api';
  const clean = raw.replace(/\/+$/, '');
  return clean.endsWith('/api') ? clean : `${clean}/api`;
}

export const API_URL = resolveApiUrl();

const TOKEN_KEY = 'fsp.token';
const REFRESH_KEY = 'fsp.refresh';
const USER_KEY = 'fsp.user';

export const storage = {
  get token(): string | null {
    return typeof window === 'undefined' ? null : localStorage.getItem(TOKEN_KEY);
  },
  get refresh(): string | null {
    return typeof window === 'undefined' ? null : localStorage.getItem(REFRESH_KEY);
  },
  save(tokens: { accessToken: string; refreshToken?: string; user?: unknown }) {
    localStorage.setItem(TOKEN_KEY, tokens.accessToken);
    if (tokens.refreshToken) localStorage.setItem(REFRESH_KEY, tokens.refreshToken);
    if (tokens.user) localStorage.setItem(USER_KEY, JSON.stringify(tokens.user));
  },
  saveUser(user: unknown) {
    localStorage.setItem(USER_KEY, JSON.stringify(user));
  },
  user<T = any>(): T | null {
    if (typeof window === 'undefined') return null;
    try {
      const raw = localStorage.getItem(USER_KEY);
      return raw ? (JSON.parse(raw) as T) : null;
    } catch {
      return null;
    }
  },
  clear() {
    [TOKEN_KEY, REFRESH_KEY, USER_KEY].forEach((k) => localStorage.removeItem(k));
  },
};

export const api = axios.create({ baseURL: API_URL, timeout: 30000 });

api.interceptors.request.use((config) => {
  const token = storage.token;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

let refreshing: Promise<string | null> | null = null;

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original: any = error.config;
    const status = error.response?.status;
    if (status === 401 && original && !original._retry && storage.refresh) {
      original._retry = true;
      refreshing =
        refreshing ||
        axios
          .post(`${API_URL}/auth/refresh`, { refreshToken: storage.refresh })
          .then((r) => {
            storage.save(r.data);
            return r.data.accessToken as string;
          })
          .catch(() => {
            storage.clear();
            return null;
          })
          .finally(() => {
            refreshing = null;
          });
      const token = await refreshing;
      if (token) {
        original.headers = { ...original.headers, Authorization: `Bearer ${token}` };
        return api(original);
      }
    }
    return Promise.reject(error);
  },
);

/** Человекочитаемая ошибка из ответа API. */
export function errMsg(error: any): string {
  const data = error?.response?.data;
  const message = data?.message ?? error?.message;
  if (Array.isArray(message)) return message.join(', ');
  if (typeof message === 'string' && message) return message;
  if (error?.response?.status === 401) return 'Сессия истекла — войдите заново';
  if (error?.code === 'ERR_NETWORK') return 'Нет связи с сервером';
  return 'Ошибка запроса';
}
