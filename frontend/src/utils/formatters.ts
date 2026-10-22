import type { ApplicationStatus, RiskZone } from '../api/client';

export const STATUS_RU: Record<ApplicationStatus, { label: string; badgeClass: string }> = {
  SUBMITTED: { label: 'Подана', badgeClass: 'bg-slate-100 text-slate-700 border-slate-200' },
  ENRICHMENT: { label: 'Обогащение', badgeClass: 'bg-slate-100 text-slate-700 border-slate-200' },
  SCORING: { label: 'Скоринг', badgeClass: 'bg-slate-100 text-slate-700 border-slate-200' },
  AUTO_APPROVED: { label: 'Автоодобрено', badgeClass: 'bg-emerald-50 text-emerald-800 border-emerald-200' },
  APPROVED: { label: 'Одобрено', badgeClass: 'bg-emerald-50 text-emerald-800 border-emerald-200' },
  COUNTER_OFFERED: { label: 'Встречные условия', badgeClass: 'bg-amber-50 text-amber-800 border-amber-200' },
  CLIENT_DECLINED: { label: 'Отклонено', badgeClass: 'bg-slate-100 text-slate-600 border-slate-200' },
  MANUAL_REVIEW: { label: 'На рассмотрении', badgeClass: 'bg-slate-100 text-slate-800 border-slate-300' },
  WAITING_DOCUMENTS: { label: 'Запрос документов', badgeClass: 'bg-slate-100 text-slate-800 border-slate-300' },
  REJECTED: { label: 'Отказ', badgeClass: 'bg-rose-50 text-rose-800 border-rose-200' },
  SIGNED: { label: 'Подписан', badgeClass: 'bg-emerald-50 text-emerald-800 border-emerald-200' },
  DISBURSED: { label: 'Деньги выданы', badgeClass: 'bg-emerald-50 text-emerald-800 border-emerald-300' },
};

export const RISK_ZONE_RU: Record<RiskZone, { label: string; badgeClass: string; title: string }> = {
  GREEN: {
    label: 'Низкий риск',
    title: 'Зеленая зона скоринга (Автоодобрение)',
    badgeClass: 'bg-emerald-50 text-emerald-800 border-emerald-200',
  },
  YELLOW: {
    label: 'Умеренный риск',
    title: 'Желтая зона скоринга (Контр-оффер)',
    badgeClass: 'bg-amber-50 text-amber-800 border-amber-200',
  },
  GREY: {
    label: 'Серая зона',
    title: 'Серая зона (Кредитный эксперт)',
    badgeClass: 'bg-slate-100 text-slate-700 border-slate-200',
  },
  RED: {
    label: 'Высокий риск',
    title: 'Красная зона (Отказ по стоп-факторам)',
    badgeClass: 'bg-rose-50 text-rose-800 border-rose-200',
  },
};

export const ROLE_RU: Record<string, string> = {
  CLIENT: 'Клиент',
  UNDERWRITER: 'Кредитный эксперт',
};
