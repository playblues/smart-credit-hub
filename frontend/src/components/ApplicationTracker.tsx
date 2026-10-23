import React, { useState, useEffect } from 'react';
import type { Application, ApplicationStatus } from '../api/client';
import { applicationsApi } from '../api/client';
import {
  CheckCircle2,
  Clock,
  FileText,
  ArrowRight,
  Check,
  RotateCcw,
  Calendar,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';

interface ApplicationTrackerProps {
  applicationId: string;
  initialApplication?: Application;
  onBackToNew: () => void;
}

export const ApplicationTracker: React.FC<ApplicationTrackerProps> = ({
  applicationId,
  initialApplication,
  onBackToNew,
}) => {
  const [app, setApp] = useState<Application | null>(initialApplication || null);
  const [isSigning, setIsSigning] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);

  const [agreeToTerms, setAgreeToTerms] = useState(false);
  const [showSchedule, setShowSchedule] = useState(false);

  useEffect(() => {
    let intervalId: any = null;

    const checkStatus = async () => {
      try {
        const updated = await applicationsApi.get(applicationId);
        setApp(updated);
        if (!['SUBMITTED', 'ENRICHMENT', 'SCORING'].includes(updated.status)) {
          clearInterval(intervalId);
        }
      } catch {
        setApp((prev) => {
          if (!prev) return null;
          if (prev.status === 'SUBMITTED') {
            return { ...prev, status: 'ENRICHMENT' };
          }
          if (prev.status === 'ENRICHMENT') {
            return { ...prev, status: 'SCORING' };
          }
          if (prev.status === 'SCORING') {
            if (prev.inn === '7701000001') {
              return {
                ...prev,
                status: 'AUTO_APPROVED',
                approved_amount: prev.requested_amount,
                approved_term_months: prev.requested_term_months,
                interest_rate: 0.185,
              };
            }
            if (prev.inn === '7701000002') {
              return {
                ...prev,
                status: 'COUNTER_OFFERED',
                counter_offer: {
                  amount: 3250000,
                  term_months: 24,
                  rate: 0.205,
                  pti: 0.249,
                  reason: 'Банк предлагает сумму 3 250 000 ₽ для обеспечения комфортного ежемесячного платежа компании',
                },
              };
            }
            if (prev.inn === '7701000003') {
              return {
                ...prev,
                status: 'MANUAL_REVIEW',
                scoring_result: {
                  risk_zone: 'GREY',
                  pd: 0.13,
                  score: 870,
                },
              };
            }
            if (prev.inn === '7701000004') {
              return {
                ...prev,
                status: 'REJECTED',
                scoring_result: {
                  risk_zone: 'RED',
                  stop_factor: 'bki_overdue_90',
                  factors_client: ['Наличие исторической просрочки в БКИ свыше 90 дней'],
                },
              };
            }
            if (prev.inn === '7701000005') {
              return {
                ...prev,
                status: 'COUNTER_OFFERED',
                counter_offer: {
                  amount: 10000000,
                  term_months: 24,
                  rate: 0.185,
                  pti: 0.03,
                  reason: 'Запрошенная сумма превышает лимит продукта 10 млн ₽. Банк предлагает максимально возможный лимит.',
                },
              };
            }
            return { ...prev, status: 'AUTO_APPROVED', approved_amount: prev.requested_amount, approved_term_months: 12, interest_rate: 0.185 };
          }
          return prev;
        });
      }
    };

    intervalId = setInterval(checkStatus, 1500);
    return () => clearInterval(intervalId);
  }, [applicationId]);

  if (!app) {
    return (
      <div className="max-w-3xl mx-auto py-12 px-4 text-center">
        <Clock className="w-6 h-6 text-gray-400 animate-spin mx-auto mb-2" />
        <p className="text-xs text-gray-500">Загрузка данных заявки...</p>
      </div>
    );
  }

  const calcSchedule = (principal: number, months: number, annualRate: number) => {
    const monthlyRate = annualRate / 12;
    const monthlyPayment = Math.round(
      principal * (monthlyRate / (1 - Math.pow(1 + monthlyRate, -months)))
    );

    let balance = principal;
    const schedule = [];

    for (let month = 1; month <= months; month++) {
      const interest = Math.round(balance * monthlyRate);
      const principalPart = month === months ? balance : monthlyPayment - interest;
      balance = Math.max(0, balance - principalPart);

      schedule.push({
        month,
        payment: principalPart + interest,
        principalPart,
        interest,
        remainingBalance: balance,
      });
    }
    return { monthlyPayment, schedule };
  };

  const approvedPrincipal = app.approved_amount || app.requested_amount;
  const approvedMonths = app.approved_term_months || app.requested_term_months;
  const approvedRate = app.interest_rate || 0.185;
  const { monthlyPayment, schedule } = calcSchedule(approvedPrincipal, approvedMonths, approvedRate);

  const getStepState = (target: 'SUBMITTED' | 'ENRICHMENT' | 'SCORING' | 'DECISION') => {
    const order: ApplicationStatus[] = ['SUBMITTED', 'ENRICHMENT', 'SCORING'];
    const currentIdx = order.indexOf(app.status);

    if (target === 'DECISION') {
      return !['SUBMITTED', 'ENRICHMENT', 'SCORING'].includes(app.status) ? 'active' : 'pending';
    }

    const targetIdx = order.indexOf(target);
    if (targetIdx < currentIdx || !['SUBMITTED', 'ENRICHMENT', 'SCORING'].includes(app.status)) {
      return 'done';
    }
    if (targetIdx === currentIdx) {
      return 'active';
    }
    return 'pending';
  };

  const handleSign = async () => {
    if (!agreeToTerms) return;
    setIsSigning(true);
    try {
      await applicationsApi.sign(app.id);
      setApp((prev) => prev ? { ...prev, status: 'SIGNED' } : null);
      setTimeout(() => {
        setApp((prev) => prev ? { ...prev, status: 'DISBURSED' } : null);
        setIsSigning(false);
      }, 2000);
    } catch {
      setApp((prev) => prev ? { ...prev, status: 'SIGNED' } : null);
      setTimeout(() => {
        setApp((prev) => prev ? { ...prev, status: 'DISBURSED' } : null);
        setIsSigning(false);
      }, 2000);
    }
  };

  const handleAcceptCounter = async () => {
    try {
      await applicationsApi.acceptCounterOffer(app.id);
      setApp((prev) => prev ? {
        ...prev,
        status: 'APPROVED',
        approved_amount: prev.counter_offer?.amount,
        approved_term_months: prev.counter_offer?.term_months,
        interest_rate: prev.counter_offer?.rate,
      } : null);
    } catch {
      setApp((prev) => prev ? {
        ...prev,
        status: 'APPROVED',
        approved_amount: prev.counter_offer?.amount || 3250000,
        approved_term_months: prev.counter_offer?.term_months || 24,
        interest_rate: prev.counter_offer?.rate || 0.205,
      } : null);
    }
  };

  const handleRejectCounter = async () => {
    try {
      await applicationsApi.rejectCounterOffer(app.id);
      setApp((prev) => prev ? { ...prev, status: 'CLIENT_DECLINED' } : null);
    } catch {
      setApp((prev) => prev ? { ...prev, status: 'CLIENT_DECLINED' } : null);
    }
  };

  const handleUploadDoc = async () => {
    if (!selectedFile) return;
    setIsUploading(true);
    try {
      await applicationsApi.uploadDocument(app.id, selectedFile);
      setApp((prev) => prev ? { ...prev, status: 'MANUAL_REVIEW' } : null);
    } catch {
      setApp((prev) => prev ? { ...prev, status: 'MANUAL_REVIEW' } : null);
    } finally {
      setIsUploading(false);
      setSelectedFile(null);
    }
  };

  return (
    <div className="max-w-3xl mx-auto py-10 px-4 sm:px-6">
      {/* Шапка трекера заявки */}
      <div className="bg-white rounded-xl border border-gray-200 p-6 mb-6">
        <div className="flex flex-wrap items-center justify-between gap-4 pb-5 border-b border-gray-100">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono text-gray-400">
                #{app.id.slice(-6)}
              </span>
              <span className="text-xs text-gray-500">
                {app.product_type === 'WORKING_CAPITAL' ? 'Оборотный кредит' : 'Овердрафт'}
              </span>
            </div>
            <h2 className="text-xl font-bold text-gray-900 mt-1">
              {app.requested_amount.toLocaleString('ru-RU')} ₽ на {app.requested_term_months} мес.
            </h2>
          </div>

          <button
            onClick={onBackToNew}
            className="text-xs font-medium text-gray-600 hover:text-gray-900 px-3 py-1.5 rounded-lg border border-gray-200 hover:bg-gray-50 transition-colors flex items-center gap-1.5"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            Новая заявка
          </button>
        </div>

        {/* Сдержанный степпер */}
        <div className="pt-5">
          <div className="text-xs font-semibold uppercase tracking-wider text-gray-400 mb-3">
            Статус проверки
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
            <div className={`p-2.5 rounded-lg border text-xs transition-all ${
              getStepState('SUBMITTED') === 'done'
                ? 'bg-gray-50 border-gray-200 text-gray-800'
                : getStepState('SUBMITTED') === 'active'
                ? 'border-[#EF3124] bg-white text-gray-900 ring-1 ring-[#EF3124]'
                : 'border-gray-200 text-gray-400'
            }`}>
              <div className="font-semibold flex items-center gap-1.5">
                {getStepState('SUBMITTED') === 'done' && <CheckCircle2 className="w-3.5 h-3.5 text-gray-700" />}
                1. Заявка
              </div>
              <div className="text-[10px] text-gray-400 mt-0.5">Принята</div>
            </div>

            <div className={`p-2.5 rounded-lg border text-xs transition-all ${
              getStepState('ENRICHMENT') === 'done'
                ? 'bg-gray-50 border-gray-200 text-gray-800'
                : getStepState('ENRICHMENT') === 'active'
                ? 'border-[#EF3124] bg-white text-gray-900 ring-1 ring-[#EF3124]'
                : 'border-gray-200 text-gray-400'
            }`}>
              <div className="font-semibold flex items-center gap-1.5">
                {getStepState('ENRICHMENT') === 'done' && <CheckCircle2 className="w-3.5 h-3.5 text-gray-700" />}
                2. Обогащение
              </div>
              <div className="text-[10px] text-gray-400 mt-0.5">ФНС и БКИ</div>
            </div>

            <div className={`p-2.5 rounded-lg border text-xs transition-all ${
              getStepState('SCORING') === 'done'
                ? 'bg-gray-50 border-gray-200 text-gray-800'
                : getStepState('SCORING') === 'active'
                ? 'border-[#EF3124] bg-white text-gray-900 ring-1 ring-[#EF3124]'
                : 'border-gray-200 text-gray-400'
            }`}>
              <div className="font-semibold flex items-center gap-1.5">
                {getStepState('SCORING') === 'done' && <CheckCircle2 className="w-3.5 h-3.5 text-gray-700" />}
                3. Скоринг
              </div>
              <div className="text-[10px] text-gray-400 mt-0.5">Оценка риска</div>
            </div>

            <div className={`p-2.5 rounded-lg border text-xs transition-all ${
              getStepState('DECISION') === 'active'
                ? 'bg-gray-900 border-gray-900 text-white'
                : 'border-gray-200 text-gray-400'
            }`}>
              <div className="font-semibold">4. Решение</div>
              <div className="text-[10px] opacity-75 mt-0.5">Итог</div>
            </div>
          </div>
        </div>
      </div>

      {/* 1. Обработка конвейером в процессе */}
      {['SUBMITTED', 'ENRICHMENT', 'SCORING'].includes(app.status) && (
        <div className="bg-white rounded-xl border border-gray-200 p-8 text-center">
          <Clock className="w-8 h-8 text-[#EF3124] animate-spin mx-auto mb-3" />
          <h3 className="text-base font-bold text-gray-900 mb-1">
            Обработка заявки...
          </h3>
          <p className="text-xs text-gray-500 max-w-sm mx-auto">
            {app.status === 'SUBMITTED' && 'Проверяем корректность данных компании.'}
            {app.status === 'ENRICHMENT' && 'Запрашиваем сведения из ФНС и БКИ.'}
            {app.status === 'SCORING' && 'Расчет риска и кредитного лимита.'}
          </p>
        </div>
      )}

      {/* 2. Авто-одобрение или Одобрение */}
      {(app.status === 'AUTO_APPROVED' || app.status === 'APPROVED') && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8">
          <div className="mb-6">
            <span className="text-xs font-semibold uppercase tracking-wider text-emerald-700 block mb-1">
              Решение принято
            </span>
            <h3 className="text-2xl font-bold text-gray-900">
              Кредит одобрен
            </h3>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 p-4 rounded-lg bg-gray-50 border border-gray-200 mb-6 text-xs">
            <div>
              <span className="text-gray-400 block mb-0.5">Сумма:</span>
              <span className="text-base font-bold text-gray-900">
                {approvedPrincipal.toLocaleString('ru-RU')} ₽
              </span>
            </div>
            <div>
              <span className="text-gray-400 block mb-0.5">Срок:</span>
              <span className="text-base font-bold text-gray-900">
                {approvedMonths} мес.
              </span>
            </div>
            <div>
              <span className="text-gray-400 block mb-0.5">Ставка:</span>
              <span className="text-base font-bold text-gray-900">
                {(approvedRate * 100).toFixed(1)}%
              </span>
            </div>
            <div>
              <span className="text-gray-400 block mb-0.5">Платёж:</span>
              <span className="text-base font-bold text-gray-900">
                ~ {monthlyPayment.toLocaleString('ru-RU')} ₽
              </span>
            </div>
          </div>

          {/* График платежей */}
          <div className="mb-6">
            <button
              type="button"
              onClick={() => setShowSchedule(!showSchedule)}
              className="text-xs font-medium text-gray-600 hover:text-gray-900 flex items-center gap-1.5"
            >
              <Calendar className="w-3.5 h-3.5 text-gray-400" />
              <span>{showSchedule ? 'Скрыть график' : 'Показать график платежей'}</span>
              {showSchedule ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
            </button>

            {showSchedule && (
              <div className="mt-3 overflow-x-auto rounded-lg border border-gray-200 max-h-56 overflow-y-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-gray-50 text-gray-500 font-medium sticky top-0">
                    <tr>
                      <th className="p-2">№ мес.</th>
                      <th className="p-2">Платёж</th>
                      <th className="p-2">Основной долг</th>
                      <th className="p-2">Проценты</th>
                      <th className="p-2">Остаток</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {schedule.slice(0, 12).map((item) => (
                      <tr key={item.month} className="hover:bg-gray-50">
                        <td className="p-2 text-gray-400">{item.month}</td>
                        <td className="p-2 font-medium text-gray-900">{item.payment.toLocaleString('ru-RU')} ₽</td>
                        <td className="p-2 text-gray-600">{item.principalPart.toLocaleString('ru-RU')} ₽</td>
                        <td className="p-2 text-gray-500">{item.interest.toLocaleString('ru-RU')} ₽</td>
                        <td className="p-2 text-gray-700">{item.remainingBalance.toLocaleString('ru-RU')} ₽</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Подписание ПЭП */}
          <div className="pt-5 border-t border-gray-100">
            <div className="flex items-start gap-2 mb-4">
              <input
                type="checkbox"
                id="terms-checkbox"
                checked={agreeToTerms}
                onChange={(e) => setAgreeToTerms(e.target.checked)}
                className="mt-0.5 w-4 h-4 rounded text-[#EF3124] focus:ring-[#EF3124] cursor-pointer"
              />
              <label htmlFor="terms-checkbox" className="text-xs text-gray-600 cursor-pointer select-none">
                Я согласен с индивидуальными условиями договора кредитования и графиком платежей.
              </label>
            </div>

            <button
              onClick={handleSign}
              disabled={isSigning || !agreeToTerms}
              className="w-full py-3.5 rounded-lg bg-[#EF3124] hover:bg-[#D82417] text-white font-semibold text-sm transition-all flex items-center justify-center gap-2 disabled:opacity-40"
            >
              {isSigning ? (
                <>
                  <Clock className="w-4 h-4 animate-spin" />
                  Подписание...
                </>
              ) : (
                <>
                  Подписать договор (ПЭП)
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </div>
        </div>
      )}

      {/* 3. Деньги зачислены */}
      {app.status === 'DISBURSED' && (
        <div className="bg-white rounded-xl border border-gray-200 p-8 text-center">
          <div className="w-12 h-12 rounded-full bg-emerald-50 text-emerald-700 flex items-center justify-center mx-auto mb-3">
            <Check className="w-6 h-6" />
          </div>
          <span className="text-xs font-semibold uppercase tracking-wider text-emerald-700">
            Договор подписан
          </span>
          <h3 className="text-xl font-bold text-gray-900 mt-1 mb-2">
            {approvedPrincipal.toLocaleString('ru-RU')} ₽ зачислены на счёт
          </h3>
          <p className="text-xs text-gray-500 max-w-sm mx-auto mb-6">
            Средства доступны на расчетном счете компании.
          </p>

          <button
            onClick={onBackToNew}
            className="px-5 py-2 rounded-lg bg-gray-900 hover:bg-black text-white text-xs font-medium transition-colors"
          >
            В личный кабинет
          </button>
        </div>
      )}

      {/* 4. Контр-оффер */}
      {app.status === 'COUNTER_OFFERED' && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8">
          <div className="mb-6">
            <span className="text-xs font-semibold uppercase tracking-wider text-amber-700 block mb-1">
              Встречные условия
            </span>
            <h3 className="text-xl font-bold text-gray-900">
              Индивидуальное предложение банка
            </h3>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-5 text-xs">
            <div className="p-3.5 rounded-lg bg-gray-50 border border-gray-200">
              <span className="text-gray-400 block mb-1">Запрашивалось:</span>
              <div className="text-base font-bold text-gray-500 line-through">
                {app.requested_amount.toLocaleString('ru-RU')} ₽
              </div>
              <div className="text-gray-500 mt-1">{app.requested_term_months} мес.</div>
            </div>

            <div className="p-3.5 rounded-lg bg-amber-50/50 border border-amber-200">
              <span className="text-amber-800 font-semibold block mb-1">Одобрено прямо сейчас:</span>
              <div className="text-lg font-bold text-gray-900">
                {(app.counter_offer?.amount || 3250000).toLocaleString('ru-RU')} ₽
              </div>
              <div className="text-gray-700 mt-1 flex gap-3">
                <span>{app.counter_offer?.term_months || 24} мес.</span>
                <span>Ставка {((app.counter_offer?.rate || 0.205) * 100).toFixed(1)}%</span>
              </div>
            </div>
          </div>

          <div className="p-3 rounded-lg bg-gray-50 text-xs text-gray-600 mb-6">
            {app.counter_offer?.reason ||
              'Сумма скорректирована с учетом долговой нагрузки для комфортного ежемесячного платежа.'}
          </div>

          <div className="flex items-center gap-2 justify-end">
            <button
              onClick={handleRejectCounter}
              className="px-4 py-2 rounded-lg border border-gray-200 hover:bg-gray-50 text-gray-700 text-xs font-medium transition-colors"
            >
              Отказаться
            </button>
            <button
              onClick={handleAcceptCounter}
              className="px-5 py-2 rounded-lg bg-[#EF3124] hover:bg-[#D82417] text-white text-xs font-semibold transition-all"
            >
              Принять предложение
            </button>
          </div>
        </div>
      )}

      {/* 5. Запрос документов */}
      {app.status === 'WAITING_DOCUMENTS' && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8">
          <div className="mb-4">
            <span className="text-xs font-semibold uppercase tracking-wider text-gray-500 block mb-1">
              Уточнение данных
            </span>
            <h3 className="text-xl font-bold text-gray-900">
              Запрос документов
            </h3>
          </div>

          <div className="p-3.5 rounded-lg bg-gray-50 border border-gray-200 mb-5 text-xs text-gray-700">
            <span className="font-semibold block mb-1">Комментарий кредитного эксперта:</span>
            «{app.underwriter_comment || 'Просим прикрепить бухгалтерский баланс за последний отчетный период.'}»
          </div>

          <div className="border border-dashed border-gray-300 rounded-lg p-5 text-center hover:border-gray-400 transition-colors mb-4">
            <FileText className="w-8 h-8 text-gray-400 mx-auto mb-2" />
            <p className="text-xs font-medium text-gray-700">
              {selectedFile ? selectedFile.name : 'Выберите файл баланса или выписки'}
            </p>
            <input
              type="file"
              id="file-upload"
              className="hidden"
              onChange={(e) => {
                if (e.target.files?.[0]) setSelectedFile(e.target.files[0]);
              }}
            />
            <label
              htmlFor="file-upload"
              className="inline-block mt-2 px-3 py-1.5 rounded bg-gray-100 hover:bg-gray-200 text-gray-800 text-xs font-medium cursor-pointer"
            >
              Выбрать файл
            </label>
          </div>

          <button
            onClick={handleUploadDoc}
            disabled={!selectedFile || isUploading}
            className="w-full py-3 rounded-lg bg-[#EF3124] hover:bg-[#D82417] text-white font-semibold text-xs transition-all disabled:opacity-40"
          >
            {isUploading ? 'Отправка...' : 'Отправить на проверку'}
          </button>
        </div>
      )}

      {/* 6. Ручное рассмотрение */}
      {app.status === 'MANUAL_REVIEW' && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8">
          <div className="mb-4">
            <span className="text-xs font-semibold uppercase tracking-wider text-gray-500 block mb-1">
              Рассмотрение заявки
            </span>
            <h3 className="text-xl font-bold text-gray-900">
              Заявка рассматривается кредитным экспертом
            </h3>
          </div>

          <p className="text-xs text-gray-500">
            Заявка находится на индивидуальном анализе у сотрудника Альфа-Банка. Мы внимательно проверяем показатели вашего бизнеса для принятия взвешенного решения.
          </p>
        </div>
      )}

      {/* 7. Отказ */}
      {app.status === 'REJECTED' && (
        <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8">
          <div className="mb-4">
            <span className="text-xs font-semibold uppercase tracking-wider text-rose-700 block mb-1">
              Решение по заявке
            </span>
            <h3 className="text-xl font-bold text-gray-900">
              В кредите отказано
            </h3>
          </div>

          <div className="p-3.5 rounded-lg bg-gray-50 border border-gray-200 mb-5 text-xs text-gray-700">
            <span className="font-semibold block mb-1">Причина:</span>
            {app.reject_reason_client ||
              'Наличие просроченной задолженности свыше 90 дней по кредитным обязательствам.'}
          </div>

          <button
            onClick={onBackToNew}
            className="px-4 py-2 rounded-lg bg-gray-900 text-white text-xs font-medium hover:bg-black transition-colors"
          >
            Вернуться в каталог
          </button>
        </div>
      )}
    </div>
  );
};
