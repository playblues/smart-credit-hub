import axios from 'axios';
import type { components } from './schema';

export type Application = components['schemas']['Application'];
export type ApplicationStatus = components['schemas']['ApplicationStatus'];
export type ProductType = components['schemas']['ProductType'];
export type RiskZone = components['schemas']['RiskZone'];
export type CompanySnapshot = components['schemas']['CompanySnapshot'];
export type CounterOffer = components['schemas']['CounterOffer'];
export type ScoringResult = components['schemas']['ScoringResult'];
export type Document = components['schemas']['Document'];
export type DocumentType = components['schemas']['DocumentType'];
export type AuditLogEntry = components['schemas']['AuditLogEntry'];

export interface UserSession {
  token: string;
  role: 'CLIENT' | 'UNDERWRITER';
  inn?: string | null;
  display_name?: string;
  login?: string;
}

const TOKEN_KEY = 'alfa_credit_token';
const SESSION_KEY = 'alfa_credit_session';

export const apiClient = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export const authStorage = {
  get: (): UserSession | null => {
    try {
      const data = localStorage.getItem(SESSION_KEY);
      return data ? JSON.parse(data) : null;
    } catch {
      return null;
    }
  },
  set: (session: UserSession) => {
    localStorage.setItem(TOKEN_KEY, session.token);
    localStorage.setItem(SESSION_KEY, JSON.stringify(session));
  },
  clear: () => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(SESSION_KEY);
  },
};

// API Services
export const authApi = {
  login: async (login: string, password = 'demo123') => {
    const res = await apiClient.post<{
      token: string;
      role: 'CLIENT' | 'UNDERWRITER';
      inn?: string | null;
      display_name?: string;
    }>('/auth/login', { login, password });
    const session: UserSession = {
      ...res.data,
      login,
    };
    authStorage.set(session);
    return session;
  },
};

export const fnsApi = {
  getByInn: async (inn: string) => {
    const res = await apiClient.get<{
      inn: string;
      name: string;
      annual_revenue: number;
      business_age_months: number;
      is_liquidating: boolean;
      okved?: string;
    }>(`/mock/fns/${inn}`);
    return res.data;
  },
};

export const bkiApi = {
  getByInn: async (inn: string) => {
    const res = await apiClient.get<{
      inn?: string;
      active_loans_count?: number;
      max_overdue_days?: number;
      had_past_delinquency?: boolean;
    }>(`/mock/bki/${inn}`);
    return res.data;
  },
};

export const applicationsApi = {
  list: async () => {
    const res = await apiClient.get<{ items: Application[]; total: number }>('/applications');
    return res.data;
  },
  get: async (id: string) => {
    const res = await apiClient.get<Application>(`/applications/${id}`);
    return res.data;
  },
  create: async (payload: {
    inn: string;
    product_type: ProductType;
    requested_amount: number;
    requested_term_months: number;
    purpose?: string;
  }) => {
    const res = await apiClient.post<Application>('/applications', payload);
    return res.data;
  },
  acceptCounterOffer: async (id: string) => {
    const res = await apiClient.post<Application>(`/applications/${id}/counter-offer/accept`);
    return res.data;
  },
  rejectCounterOffer: async (id: string) => {
    const res = await apiClient.post<Application>(`/applications/${id}/counter-offer/reject`);
    return res.data;
  },
  sign: async (id: string) => {
    const res = await apiClient.post<Application>(`/applications/${id}/sign`);
    return res.data;
  },
  uploadDocument: async (id: string, file: File, document_type: DocumentType = 'FINANCIAL_STATEMENT') => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('document_type', document_type);
    const res = await apiClient.post<Document>(`/applications/${id}/documents`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return res.data;
  },
};

export const bankApi = {
  list: async (params?: { status?: ApplicationStatus; risk_zone?: RiskZone }) => {
    const res = await apiClient.get<{ items: Application[]; total: number }>('/bank/applications', { params });
    return res.data;
  },
  get: async (id: string) => {
    const res = await apiClient.get<Application>(`/bank/applications/${id}`);
    return res.data;
  },
  decision: async (
    id: string,
    decision: {
      action: 'APPROVE' | 'REJECT' | 'REQUEST_DOCUMENTS' | 'COUNTER_OFFER';
      comment?: string;
      counter_offer_amount?: number;
      counter_offer_term_months?: number;
      counter_offer?: {
        amount: number;
        term_months: number;
        rate: number;
        reason?: string;
      };
    }
  ) => {
    const res = await apiClient.post<Application>(`/bank/applications/${id}/decision`, decision);
    return res.data;
  },
};
