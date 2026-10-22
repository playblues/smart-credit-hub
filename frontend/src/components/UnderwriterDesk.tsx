import React, { useState, useEffect } from 'react';
import type { Application, RiskZone } from '../api/client';
import { bankApi } from '../api/client';
import { STATUS_RU, RISK_ZONE_RU } from '../utils/formatters';
import {
  Check,
  X,
  FileText,
  Sparkles,
  Send,
  Loader2,
  ChevronRight,
  TrendingDown,
  Inbox,
  Filter,
} from 'lucide-react';

interface UnderwriterDeskProps {
  currentApp?: Application | null;
  onSelectApplication: (app: Application) => void;
  onDecisionApplied: (updated: Application) => void;
}

const INITIAL_BANK_APPLICATIONS: Application[] = [
  {
    id: 'app-techno-7701000003',
    inn: '7701000003',
    status: 'MANUAL_REVIEW',
    product_type: 'OVERDRAFT',
    requested_amount: 2500000,
    requested_term_months: 12,
    created_at: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    purpose: 'Кассовый разрыв IT-проекта',
    company_snapshot: {
      inn: '7701000003',
      name: 'ООО «ТехноСтарт»',
      annual_revenue: 12000000,
      business_age_months: 8,
      is_liquidating: false,
      bki_active_loans: 4,
      bki_max_overdue_days: 25,
      bki_had_past_delinquency: true,
    },
    scoring_result: {
      score: 870,
      pd: 0.1301,
      risk_zone: 'GREY',
      factors_bank: [
        'Возраст компании 8 месяцев (высокая неопределенность)',
        '4 активных кредитных договора в БКИ',
        'Максимальная просрочка 25 дней',
      ],
    },
  },
  {
    id: 'app-smirnov-7701000002',
    inn: '7701000002',
    status: 'COUNTER_OFFERED',
    product_type: 'WORKING_CAPITAL',
    requested_amount: 5000000,
    requested_term_months: 24,
    created_at: new Date(Date.now() - 1000 * 60 * 45).toISOString(),
    purpose: 'Расширение торговой точки',
    company_snapshot: {
      inn: '7701000002',
      name: 'ИП Смирнов А.П.',
      annual_revenue: 8000000,
      business_age_months: 18,
      is_liquidating: false,
      bki_active_loans: 2,
      bki_max_overdue_days: 12,
      bki_had_past_delinquency: true,
    },
    scoring_result: {
      score: 925,
      pd: 0.0748,
      risk_zone: 'YELLOW',
      factors_bank: ['Высокая запрашиваемая сумма к выручке (PTI = 37% > 35%)'],
    },
    counter_offer: {
      amount: 3250000,
      term_months: 24,
      rate: 0.205,
      pti: 0.249,
      reason: 'Банк оптимизировал размер платежа под оборот компании',
    },
  },
  {
    id: 'app-alfa-7701000001',
    inn: '7701000001',
    status: 'AUTO_APPROVED',
    product_type: 'WORKING_CAPITAL',
    requested_amount: 3000000,
    requested_term_months: 12,
    created_at: new Date(Date.now() - 1000 * 60 * 120).toISOString(),
    purpose: 'Закупка автозапчастей',
    company_snapshot: {
      inn: '7701000001',
      name: 'ООО «Альфа Логистика»',
      annual_revenue: 50000000,
      business_age_months: 48,
      is_liquidating: false,
      bki_active_loans: 1,
      bki_max_overdue_days: 0,
      bki_had_past_delinquency: false,
    },
    scoring_result: {
      score: 995,
      pd: 0.0055,
      risk_zone: 'GREEN',
      factors_bank: ['Высокая выручка', 'Идеальная кредитная история в БКИ'],
    },
  },
  {
    id: 'app-negative-7701000004',
    inn: '7701000004',
    status: 'REJECTED',
    product_type: 'WORKING_CAPITAL',
    requested_amount: 4000000,
    requested_term_months: 12,
    created_at: new Date(Date.now() - 1000 * 60 * 180).toISOString(),
    purpose: 'Пополнение оборотных средств',
    company_snapshot: {
      inn: '7701000004',
      name: 'ООО «Негатив Трейд»',
      annual_revenue: 20000000,
      business_age_months: 36,
      is_liquidating: false,
      bki_active_loans: 4,
      bki_max_overdue_days: 115,
      bki_had_past_delinquency: true,
    },
    scoring_result: {
      score: 410,
      pd: 0.38,
      risk_zone: 'RED',
      stop_factor: 'bki_overdue_90',
      factors_bank: ['Стоп-фактор: историческая просрочка 115 дней в БКИ'],
    },
  },
  {
    id: 'app-mega-7701000005',
    inn: '7701000005',
    status: 'COUNTER_OFFERED',
    product_type: 'WORKING_CAPITAL',
    requested_amount: 15000000,
    requested_term_months: 24,
    created_at: new Date(Date.now() - 1000 * 60 * 240).toISOString(),
    purpose: 'Закупка оборудования',
    company_snapshot: {
      inn: '7701000005',
      name: 'ООО «Мегастрой»',
      annual_revenue: 200000000,
      business_age_months: 60,
      is_liquidating: false,
      bki_active_loans: 0,
      bki_max_overdue_days: 0,
      bki_had_past_delinquency: false,
    },
    scoring_result: {
      score: 998,
      pd: 0.0024,
      risk_zone: 'GREEN',
      factors_bank: ['Превышение лимита продукта (15 млн > 10 млн)'],
    },
    counter_offer: {
      amount: 10000000,
      term_months: 24,
      rate: 0.185,
      pti: 0.03,
      reason: 'Банк предлагает максимально доступный лимит продукта 10 млн ₽',
    },
  },
];

export const UnderwriterDesk: React.FC<UnderwriterDeskProps> = ({
  currentApp,
  onDecisionApplied,
}) => {
  const [applications, setApplications] = useState<Application[]>(() => {
    if (currentApp && !INITIAL_BANK_APPLICATIONS.some((a) => a.id === currentApp.id)) {
      return [currentApp, ...INITIAL_BANK_APPLICATIONS];
    }
    return INITIAL_BANK_APPLICATIONS;
  });

  const [selectedId, setSelectedId] = useState<string>(
    currentApp?.id || INITIAL_BANK_APPLICATIONS[0].id
  );

  const [statusFilter, setStatusFilter] = useState<string>('ALL');
  const [riskFilter, setRiskFilter] = useState<string>('ALL');

  // Автоматическая загрузка живых заявок с бэкенда при доступности API
  useEffect(() => {
    bankApi
      .list()
      .then((res) => {
        if (res && res.items && res.items.length > 0) {
          setApplications(res.items);
          setSelectedId((prev) => (res.items.some((a) => a.id === prev) ? prev : res.items[0].id));
        }
      })
      .catch(() => {
        // Если роут на бэкенде еще не развернут, остаемся на эталонных кейсах
      });
  }, []);

  const selectedApp =
    applications.find((a) => a.id === selectedId) || applications[0];

  const [decisionAction, setDecisionAction] = useState<
    'APPROVE' | 'REJECT' | 'REQUEST_DOCUMENTS' | 'COUNTER_OFFER' | null
  >(null);
  const [comment, setComment] = useState('');
  const [counterAmount, setCounterAmount] = useState(1500000);
  const [counterTerm, setCounterTerm] = useState(12);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const filteredApps = applications.filter((app) => {
    if (statusFilter !== 'ALL' && app.status !== statusFilter) return false;
    if (riskFilter !== 'ALL' && app.scoring_result?.risk_zone !== riskFilter) return false;
    return true;
  });

  const handleDecision = async () => {
    if (!decisionAction || !selectedApp) return;
    setIsSubmitting(true);

    try {
      const payload: any = { action: decisionAction, comment };
      if (decisionAction === 'COUNTER_OFFER') {
        payload.counter_offer = {
          amount: counterAmount,
          term_months: counterTerm,
          rate: 0.21,
          reason: comment || 'Корректировка условий кредитным экспертом банка',
        };
      }
      const backendUpdated = await bankApi.decision(selectedApp.id, payload);
      if (backendUpdated) {
        setApplications((prev) =>
          prev.map((a) => (a.id === backendUpdated.id ? backendUpdated : a))
        );
        onDecisionApplied(backendUpdated);
        setIsSubmitting(false);
        setDecisionAction(null);
        setComment('');
        return;
      }
    } catch {
      // локальный фоллбек для демо
    } finally {
      let updatedStatus: any = 'APPROVED';
      if (decisionAction === 'REJECT') updatedStatus = 'REJECTED';
      if (decisionAction === 'REQUEST_DOCUMENTS') updatedStatus = 'WAITING_DOCUMENTS';
      if (decisionAction === 'COUNTER_OFFER') updatedStatus = 'COUNTER_OFFERED';

      const updated: Application = {
        ...selectedApp,
        status: updatedStatus,
        underwriter_comment: comment,
        approved_amount: decisionAction === 'APPROVE' ? selectedApp.requested_amount : undefined,
        counter_offer: decisionAction === 'COUNTER_OFFER' ? {
          amount: counterAmount,
          term_months: counterTerm,
          rate: 0.21,
          reason: comment || 'Индивидуальные условия от кредитного эксперта',
        } : selectedApp.counter_offer,
      };

      setApplications((prev) =>
        prev.map((a) => (a.id === updated.id ? updated : a))
      );
      onDecisionApplied(updated);
      setIsSubmitting(false);
      setDecisionAction(null);
      setComment('');
    }
  };

  const renderRiskBadge = (zone?: RiskZone) => {
    if (!zone) return null;
    const config = RISK_ZONE_RU[zone];
    return (
      <span
        title={config.title}
        className={`px-2 py-0.5 rounded text-[11px] font-bold border ${config.badgeClass}`}
      >
        {config.label}
      </span>
    );
  };

  const renderStatusBadge = (status: Application['status']) => {
    const config = STATUS_RU[status];
    return (
      <span className={`px-2 py-0.5 rounded text-[11px] font-bold border ${config.badgeClass}`}>
        {config.label}
      </span>
    );
  };

  return (
    <div className="max-w-7xl mx-auto py-6 px-3 sm:px-6">
      {/* Заголовок и фильтры: на мобилках в столбик, на десктопе в ряд */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6 pb-4 border-b border-gray-200">
        <div>
          <div className="flex items-center gap-2">
            <span className="p-1 rounded bg-slate-100 text-slate-800 font-bold text-xs uppercase">
              Служба банка
            </span>
            <span className="text-xs text-gray-500 font-medium">Консоль рассмотрения кредитных заявок</span>
          </div>
          <h1 className="text-xl sm:text-2xl font-black text-gray-900 mt-1">
            Рабочее место кредитного эксперта
          </h1>
        </div>

        {/* Фильтры */}
        <div className="flex flex-wrap sm:flex-nowrap items-center gap-2 w-full sm:w-auto">
          <div className="flex items-center gap-1.5 text-xs text-gray-400">
            <Filter className="w-3.5 h-3.5" />
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="text-xs font-semibold px-2.5 py-1.5 rounded-lg border border-gray-300 bg-white flex-1 sm:flex-initial"
          >
            <option value="ALL">Все статусы ({applications.length})</option>
            <option value="MANUAL_REVIEW">На рассмотрении</option>
            <option value="WAITING_DOCUMENTS">Запрос документов</option>
            <option value="COUNTER_OFFERED">Встречные условия</option>
            <option value="AUTO_APPROVED">Автоодобрено</option>
            <option value="REJECTED">Отказ</option>
          </select>

          <select
            value={riskFilter}
            onChange={(e) => setRiskFilter(e.target.value)}
            className="text-xs font-semibold px-2.5 py-1.5 rounded-lg border border-gray-300 bg-white flex-1 sm:flex-initial"
          >
            <option value="ALL">Все зоны риска</option>
            <option value="GREY">Серая зона</option>
            <option value="YELLOW">Желтая зона</option>
            <option value="GREEN">Зеленая зона</option>
            <option value="RED">Красная зона</option>
          </select>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* ЛЕВАЯ КОЛОНКА: Очередь заявок (на мобилке компактно прокручивается) */}
        <div className="lg:col-span-4 bg-white rounded-xl border border-gray-200 p-4 max-h-[320px] lg:max-h-[750px] flex flex-col">
          <div className="flex items-center justify-between pb-2.5 border-b border-gray-100 mb-2.5">
            <span className="text-xs font-bold uppercase tracking-wider text-gray-500 flex items-center gap-1.5">
              <Inbox className="w-3.5 h-3.5" /> Очередь ({filteredApps.length})
            </span>
          </div>

          <div className="flex-1 overflow-y-auto space-y-2 pr-1">
            {filteredApps.map((app) => {
              const isSelected = app.id === selectedId;
              return (
                <div
                  key={app.id}
                  onClick={() => setSelectedId(app.id)}
                  className={`p-3 rounded-lg border text-left cursor-pointer transition-all ${
                    isSelected
                      ? 'border-[#EF3124] bg-red-50/30 ring-1 ring-[#EF3124]'
                      : 'border-gray-200 hover:border-gray-300 hover:bg-gray-50'
                  }`}
                >
                  <div className="flex items-start justify-between gap-1 mb-1">
                    <span className="font-semibold text-xs text-gray-900 truncate">
                      {app.company_snapshot?.name || `Компания ${app.inn}`}
                    </span>
                    {renderRiskBadge(app.scoring_result?.risk_zone)}
                  </div>

                  <div className="text-xs font-bold text-gray-900 mb-1">
                    {app.requested_amount.toLocaleString('ru-RU')} ₽
                    <span className="text-gray-400 font-normal ml-1">на {app.requested_term_months} мес.</span>
                  </div>

                  <div className="flex items-center justify-between text-[11px] text-gray-500 pt-1 border-t border-gray-100">
                    <div>{renderStatusBadge(app.status)}</div>
                    <span className="font-mono text-[10px] text-gray-400">{app.inn}</span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* ПРАВАЯ ОБЛАСТЬ: Детальная карточка и решение */}
        <div className="lg:col-span-8 space-y-6">
          <div className="bg-white rounded-xl border border-gray-200 p-5 sm:p-6">
            <div className="flex flex-wrap items-center justify-between gap-2 mb-4 pb-3 border-b border-gray-100">
              <div>
                <span className="text-xs font-mono text-gray-400 block">
                  #{selectedApp.id.slice(-8)}
                </span>
                <h3 className="text-base sm:text-lg font-bold text-gray-900">
                  {selectedApp.company_snapshot?.name || 'Компания'}
                </h3>
              </div>
              <div className="flex items-center gap-2">
                {renderRiskBadge(selectedApp.scoring_result?.risk_zone)}
                {renderStatusBadge(selectedApp.status)}
              </div>
            </div>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
              <div>
                <span className="text-gray-400 block">ИНН:</span>
                <span className="font-semibold text-gray-800">{selectedApp.inn}</span>
              </div>
              <div>
                <span className="text-gray-400 block">Выручка:</span>
                <span className="font-semibold text-gray-800">
                  {(selectedApp.company_snapshot?.annual_revenue || 0).toLocaleString('ru-RU')} ₽
                </span>
              </div>
              <div>
                <span className="text-gray-400 block">Срок работы:</span>
                <span className="font-semibold text-gray-800">
                  {selectedApp.company_snapshot?.business_age_months || 0} мес.
                </span>
              </div>
              <div>
                <span className="text-gray-400 block">Кредитов в БКИ:</span>
                <span className="font-semibold text-gray-800">
                  {selectedApp.company_snapshot?.bki_active_loans ?? 0} шт.
                </span>
              </div>
            </div>
          </div>

          {/* Скоринговая аналитика */}
          <div className="bg-white rounded-xl border border-gray-200 p-5 sm:p-6">
            <h3 className="text-sm font-bold text-gray-900 mb-3 flex items-center gap-1.5">
              <Sparkles className="w-4 h-4 text-purple-600" />
              Оценка риска конвейера
            </h3>
            <div className="grid grid-cols-2 gap-4 mb-4 p-3.5 rounded-lg bg-gray-50 border border-gray-200">
              <div>
                <span className="text-xs text-gray-500 block">Вероятность дефолта (PD):</span>
                <span className="text-xl sm:text-2xl font-black text-gray-900">
                  {((selectedApp.scoring_result?.pd || 0.13) * 100).toFixed(2)}%
                </span>
              </div>
              <div>
                <span className="text-xs text-gray-500 block">Скорбалл (0..1000):</span>
                <span className="text-xl sm:text-2xl font-black text-gray-900">
                  {selectedApp.scoring_result?.score || 870}
                </span>
              </div>
            </div>

            <div className="text-xs">
              <span className="font-semibold text-gray-700 block mb-1">Факторы риска:</span>
              <ul className="space-y-1 list-disc pl-4 text-gray-500">
                {(selectedApp.scoring_result?.factors_bank || [
                  'Молодой бизнес менее 12 месяцев',
                  'Повышенная долговая нагрузка в БКИ',
                ]).map((f, i) => (
                  <li key={i}>{f}</li>
                ))}
              </ul>
            </div>
          </div>

          {/* Панель решения */}
          <div className="bg-white rounded-xl border border-gray-200 p-5 sm:p-6">
            <h3 className="text-sm font-bold text-gray-900 mb-4 flex items-center gap-1.5">
              <Send className="w-4 h-4 text-[#EF3124]" />
              Принятие решения
            </h3>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 mb-4">
              <button
                type="button"
                onClick={() => setDecisionAction('APPROVE')}
                className={`p-2.5 rounded-lg border text-xs font-semibold transition-all text-center ${
                  decisionAction === 'APPROVE'
                    ? 'border-emerald-600 bg-emerald-50 text-emerald-900 ring-1 ring-emerald-500'
                    : 'border-gray-200 hover:bg-gray-50 text-gray-700'
                }`}
              >
                <Check className="w-4 h-4 text-emerald-600 mx-auto mb-1" />
                Одобрить
              </button>

              <button
                type="button"
                onClick={() => setDecisionAction('COUNTER_OFFER')}
                className={`p-2.5 rounded-lg border text-xs font-semibold transition-all text-center ${
                  decisionAction === 'COUNTER_OFFER'
                    ? 'border-amber-500 bg-amber-50 text-amber-900 ring-1 ring-amber-400'
                    : 'border-gray-200 hover:bg-gray-50 text-gray-700'
                }`}
              >
                <TrendingDown className="w-4 h-4 text-amber-600 mx-auto mb-1" />
                Контр-оффер
              </button>

              <button
                type="button"
                onClick={() => setDecisionAction('REQUEST_DOCUMENTS')}
                className={`p-2.5 rounded-lg border text-xs font-semibold transition-all text-center ${
                  decisionAction === 'REQUEST_DOCUMENTS'
                    ? 'border-indigo-500 bg-indigo-50 text-indigo-900 ring-1 ring-indigo-400'
                    : 'border-gray-200 hover:bg-gray-50 text-gray-700'
                }`}
              >
                <FileText className="w-4 h-4 text-indigo-600 mx-auto mb-1" />
                Запрос документов
              </button>

              <button
                type="button"
                onClick={() => setDecisionAction('REJECT')}
                className={`p-2.5 rounded-lg border text-xs font-semibold transition-all text-center ${
                  decisionAction === 'REJECT'
                    ? 'border-rose-500 bg-rose-50 text-rose-900 ring-1 ring-rose-400'
                    : 'border-gray-200 hover:bg-gray-50 text-gray-700'
                }`}
              >
                <X className="w-4 h-4 text-rose-600 mx-auto mb-1" />
                Отказать
              </button>
            </div>

            {decisionAction === 'COUNTER_OFFER' && (
              <div className="p-3.5 rounded-lg bg-amber-50/50 border border-amber-200 space-y-3 mb-4 text-xs">
                <div>
                  <label className="font-semibold text-amber-900 block mb-1">
                    Новая сумма: {counterAmount.toLocaleString('ru-RU')} ₽
                  </label>
                  <input
                    type="range"
                    min="500000"
                    max={selectedApp.requested_amount}
                    step="50000"
                    value={counterAmount}
                    onChange={(e) => setCounterAmount(Number(e.target.value))}
                    className="w-full accent-amber-600 h-1.5 bg-amber-200 rounded"
                  />
                </div>
                <div>
                  <label className="font-semibold text-amber-900 block mb-1">
                    Срок: {counterTerm} мес.
                  </label>
                  <input
                    type="range"
                    min="3"
                    max="36"
                    value={counterTerm}
                    onChange={(e) => setCounterTerm(Number(e.target.value))}
                    className="w-full accent-amber-600 h-1.5 bg-amber-200 rounded"
                  />
                </div>
              </div>
            )}

            {decisionAction && (
              <div className="mb-4">
                <label className="text-xs font-semibold text-gray-700 block mb-1">
                  Комментарий клиенту {decisionAction === 'REQUEST_DOCUMENTS' ? '(обязательно)' : ''}:
                </label>
                <textarea
                  rows={2}
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder={
                    decisionAction === 'REQUEST_DOCUMENTS'
                      ? 'Укажите перечень запрашиваемых документов'
                      : 'Пояснение к принятому решению'
                  }
                  className="w-full p-2.5 rounded-lg border border-gray-300 text-xs focus:ring-1 focus:ring-[#EF3124] focus:outline-none"
                />
              </div>
            )}

            <button
              onClick={handleDecision}
              disabled={!decisionAction || isSubmitting || (decisionAction === 'REQUEST_DOCUMENTS' && !comment.trim())}
              className="w-full py-3 rounded-lg bg-[#0B1F35] hover:bg-black text-white font-semibold text-xs transition-all flex items-center justify-center gap-1.5 disabled:opacity-40"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" /> Обработка...
                </>
              ) : (
                <>
                  Применить решение
                  <ChevronRight className="w-3.5 h-3.5" />
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
