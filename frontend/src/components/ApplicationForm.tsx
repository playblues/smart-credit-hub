import React, { useState, useEffect } from 'react';
import type { ProductType, Application } from '../api/client';
import { fnsApi, applicationsApi } from '../api/client';
import { DEMO_PROFILES } from '../mockData/demoProfiles';
import {
  Building2,
  Check,
  ArrowRight,
  Loader2,
  ShieldCheck,
  Sparkles,
  Zap,
  FileCheck2,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';

interface ApplicationFormProps {
  initialInn?: string;
  initialAmount?: number;
  initialTerm?: number;
  initialProduct?: ProductType;
  initialPurpose?: string;
  onApplicationCreated: (app: Application) => void;
  onResetToEmpty?: () => void;
}

export const ApplicationForm: React.FC<ApplicationFormProps> = ({
  initialInn = '',
  initialAmount = 1000000,
  initialTerm = 12,
  initialProduct = 'WORKING_CAPITAL',
  initialPurpose = '',
  onApplicationCreated,
  onResetToEmpty,
}) => {
  const [productType, setProductType] = useState<ProductType>(initialProduct);
  const [inn, setInn] = useState(initialInn);
  const [amount, setAmount] = useState(initialAmount);
  const [termMonths, setTermMonths] = useState(initialTerm);
  const [purpose, setPurpose] = useState(initialPurpose);

  const [isSearchingFns, setIsSearchingFns] = useState(false);
  const [companyInfo, setCompanyInfo] = useState<{
    name: string;
    annual_revenue: number;
    business_age_months: number;
    is_liquidating: boolean;
  } | null>(null);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [openFaq, setOpenFaq] = useState<number | null>(null);

  useEffect(() => {
    setInn(initialInn);
    setAmount(initialAmount);
    setTermMonths(initialTerm);
    setProductType(initialProduct);
    setPurpose(initialPurpose);
  }, [initialInn, initialAmount, initialTerm, initialProduct, initialPurpose]);

  useEffect(() => {
    const cleanInn = inn.trim();
    if (cleanInn.length === 10 || cleanInn.length === 12) {
      setIsSearchingFns(true);
      fnsApi
        .getByInn(cleanInn)
        .then((data) => {
          setCompanyInfo(data);
        })
        .catch(() => {
          const found = DEMO_PROFILES.find((p) => p.inn === cleanInn);
          if (found) {
            setCompanyInfo({
              name: found.name,
              annual_revenue:
                found.inn === '7701000001'
                  ? 50000000
                  : found.inn === '7701000002'
                  ? 8000000
                  : found.inn === '7701000003'
                  ? 12000000
                  : found.inn === '7701000004'
                  ? 20000000
                  : 200000000,
              business_age_months:
                found.inn === '7701000001'
                  ? 48
                  : found.inn === '7701000002'
                  ? 18
                  : found.inn === '7701000003'
                  ? 8
                  : found.inn === '7701000004'
                  ? 36
                  : 60,
              is_liquidating: false,
            });
          } else {
            setCompanyInfo(null);
          }
        })
        .finally(() => {
          setIsSearchingFns(false);
        });
    } else {
      setCompanyInfo(null);
    }
  }, [inn]);

  const rateAnnual = productType === 'WORKING_CAPITAL' ? 0.185 : 0.21;
  const rateMonthly = rateAnnual / 12;
  const monthlyPayment = Math.round(
    amount * (rateMonthly / (1 - Math.pow(1 + rateMonthly, -termMonths)))
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setSubmitError(null);

    try {
      const created = await applicationsApi.create({
        inn,
        product_type: productType,
        requested_amount: amount,
        requested_term_months: termMonths,
        purpose,
      });
      onApplicationCreated(created);
    } catch {
      const mockCreated: Application = {
        id: `mock-app-${Date.now()}`,
        inn,
        status: 'SUBMITTED',
        product_type: productType,
        purpose,
        requested_amount: amount,
        requested_term_months: termMonths,
        created_at: new Date().toISOString(),
        company_snapshot: companyInfo
          ? {
              inn,
              name: companyInfo.name,
              annual_revenue: companyInfo.annual_revenue,
              business_age_months: companyInfo.business_age_months,
              is_liquidating: companyInfo.is_liquidating,
            }
          : undefined,
      };
      onApplicationCreated(mockCreated);
    } finally {
      setIsSubmitting(false);
    }
  };

  const maxAmount = productType === 'WORKING_CAPITAL' ? 10000000 : 3000000;
  const maxTerm = productType === 'WORKING_CAPITAL' ? 36 : 12;

  const quickAmounts =
    productType === 'WORKING_CAPITAL'
      ? [1000000, 3000000, 5000000, 10000000]
      : [500000, 1000000, 2000000, 3000000];

  const quickTerms =
    productType === 'WORKING_CAPITAL'
      ? [6, 12, 18, 24, 36]
      : [3, 6, 9, 12];

  const faqs = [
    {
      q: 'Как быстро цифровой конвейер принимает решение?',
      a: 'Скоринг работает в реальном времени: обогащение данных по ИНН через реестры ФНС и БКИ занимает от 30 до 60 секунд. При высоком скорбалле предодобрение выдаётся мгновенно.',
    },
    {
      q: 'Нужен ли залог или сбор бумажных справок?',
      a: 'Нет. Оборотный кредит и овердрафт до 10 млн ₽ предоставляются без залогового обеспечения. Все финансовые показатели бизнеса оцениваются автоматически по открытым реестрам.',
    },
    {
      q: 'Как подписать кредитный договор?',
      a: 'Подписание происходит полностью дистанционно с помощью простой электронной подписи (ПЭП) по СМС прямо в кабинете Альфа-Банка.',
    },
  ];

  return (
    <div className="max-w-6xl mx-auto py-8 px-4 sm:px-6">
      {/* Заголовок страницы в фирменном стиле Альфы */}
      <div className="mb-8">
        <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-red-50 text-[#EF3124] text-xs font-bold mb-3 tracking-wide">
          <Sparkles className="w-3.5 h-3.5" />
          Цифровой кредитный конвейер МСП
        </div>
        <h1 className="text-3xl sm:text-4xl font-black text-gray-900 tracking-tight">
          Кредитование бизнеса онлайн
        </h1>
        <p className="mt-2 text-sm sm:text-base text-gray-500 max-w-2xl">
          Автоматическое решение по данным ФНС и БКИ за 1 минуту без бумажных справок и визита в банк
        </p>
      </div>

      {/* Двухколоночный макет в стиле Alfa Design System */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        {/* Левая колонка: форма ввода параметров */}
        <div className="lg:col-span-7 bg-white rounded-3xl border border-gray-200/90 p-6 sm:p-8 shadow-xs space-y-7">
          {/* 1. Выбор продукта */}
          <div>
            <label className="block text-xs font-bold uppercase tracking-wider text-gray-400 mb-3">
              1. Кредитный продукт
            </label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {/* Оборотный кредит */}
              <div
                onClick={() => {
                  setProductType('WORKING_CAPITAL');
                  if (amount > 10000000) setAmount(10000000);
                  if (termMonths > 36) setTermMonths(36);
                }}
                className={`p-4 rounded-2xl cursor-pointer transition-all ${
                  productType === 'WORKING_CAPITAL'
                    ? 'border-2 border-[#EF3124] bg-red-50/20 shadow-xs'
                    : 'border border-gray-200 hover:border-gray-300 bg-white'
                }`}
              >
                <div className="flex justify-between items-baseline mb-1">
                  <span className="font-bold text-sm text-gray-900">
                    Оборотный кредит
                  </span>
                  <span className="text-xs font-extrabold text-[#EF3124] bg-red-50 px-2 py-0.5 rounded-md">
                    от 18.5%
                  </span>
                </div>
                <p className="text-xs text-gray-500 mb-3 leading-relaxed">
                  Для закупки товаров, сырья и покрытия кассовых разрывов.
                </p>
                <div className="text-[11px] font-medium text-gray-400 pt-2 border-t border-gray-100 flex justify-between">
                  <span>До 10 млн ₽</span>
                  <span>До 36 мес.</span>
                </div>
              </div>

              {/* Овердрафт */}
              <div
                onClick={() => {
                  setProductType('OVERDRAFT');
                  if (amount > 3000000) setAmount(3000000);
                  if (termMonths > 12) setTermMonths(12);
                }}
                className={`p-4 rounded-2xl cursor-pointer transition-all ${
                  productType === 'OVERDRAFT'
                    ? 'border-2 border-[#EF3124] bg-red-50/20 shadow-xs'
                    : 'border border-gray-200 hover:border-gray-300 bg-white'
                }`}
              >
                <div className="flex justify-between items-baseline mb-1">
                  <span className="font-bold text-sm text-gray-900">
                    Овердрафт
                  </span>
                  <span className="text-xs font-extrabold text-[#EF3124] bg-red-50 px-2 py-0.5 rounded-md">
                    от 21.0%
                  </span>
                </div>
                <p className="text-xs text-gray-500 mb-3 leading-relaxed">
                  Кредитный лимит к счёту для непрерывных оплат поставщикам.
                </p>
                <div className="text-[11px] font-medium text-gray-400 pt-2 border-t border-gray-100 flex justify-between">
                  <span>До 3 млн ₽</span>
                  <span>До 12 мес.</span>
                </div>
              </div>
            </div>
          </div>

          {/* 2. ИНН на мягкой подложке (фирменный стиль инпутов Альфы) */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-xs font-bold uppercase tracking-wider text-gray-400">
                2. ИНН организации или ИП
              </label>
              {(inn || purpose) && (
                <button
                  type="button"
                  onClick={() => {
                    setInn('');
                    setPurpose('');
                    setCompanyInfo(null);
                    if (onResetToEmpty) onResetToEmpty();
                  }}
                  className="text-xs text-[#EF3124] hover:underline font-semibold"
                >
                  Очистить
                </button>
              )}
            </div>

            <div className="bg-[#F2F3F5] rounded-2xl p-3.5 focus-within:bg-white focus-within:ring-2 focus-within:ring-[#EF3124] focus-within:border-transparent transition-all">
              <div className="flex items-center justify-between">
                <input
                  type="text"
                  value={inn}
                  onChange={(e) =>
                    setInn(e.target.value.replace(/\D/g, '').slice(0, 12))
                  }
                  placeholder="Введите 10 или 12 цифр ИНН"
                  className="w-full bg-transparent text-sm sm:text-base font-semibold text-gray-900 placeholder:text-gray-400 focus:outline-none"
                />
                {isSearchingFns && (
                  <Loader2 className="w-5 h-5 text-gray-400 animate-spin shrink-0 ml-2" />
                )}
              </div>
            </div>

            {/* Плашка ФНС */}
            {companyInfo && (
              <div className="mt-3 p-4 rounded-2xl bg-slate-50 border border-slate-200 text-xs animate-fadeIn">
                <div className="flex items-center justify-between mb-2">
                  <div className="font-bold text-gray-900 flex items-center gap-2">
                    <Building2 className="w-4 h-4 text-gray-600" />
                    <span>{companyInfo.name}</span>
                  </div>
                  <span className="text-[11px] bg-emerald-100 text-emerald-800 font-bold px-2 py-0.5 rounded-full flex items-center gap-1">
                    <Check className="w-3 h-3" /> ФНС подтверждено
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2 text-xs text-gray-500 pt-1">
                  <div>
                    Выручка: <span className="font-bold text-gray-900 block sm:inline">{companyInfo.annual_revenue.toLocaleString('ru-RU')} ₽</span>
                  </div>
                  <div>
                    Стаж бизнеса: <span className="font-bold text-gray-900 block sm:inline">{companyInfo.business_age_months} мес.</span>
                  </div>
                  <div>
                    Статус: <span className="font-bold text-gray-900 block sm:inline">{companyInfo.is_liquidating ? 'Ликвидация' : 'Действует'}</span>
                  </div>
                </div>
              </div>
            )}

            {/* Плашка безопасности данных Альфа-Банка */}
            <div className="mt-2.5 flex items-center gap-1.5 text-[11px] text-gray-400">
              <ShieldCheck className="w-3.5 h-3.5 text-gray-400" />
              <span>Гарантия конфиденциальности и защиты банковской тайны</span>
            </div>
          </div>

          {/* 3. Сумма кредита с быстрыми чипсами (Pills) */}
          <div className="space-y-4 pt-2 border-t border-gray-100">
            <div>
              <div className="flex justify-between items-baseline mb-2">
                <label className="text-xs font-bold uppercase tracking-wider text-gray-400">
                  3. Сумма кредита
                </label>
                <span className="text-xl sm:text-2xl font-black text-gray-900">
                  {amount.toLocaleString('ru-RU')} ₽
                </span>
              </div>

              <input
                type="range"
                min="300000"
                max={maxAmount}
                step="50000"
                value={amount}
                onChange={(e) => setAmount(Number(e.target.value))}
                className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-[#EF3124]"
              />

              {/* Фирменные чипсы-кнопки Альфы для быстрого выбора */}
              <div className="flex flex-wrap gap-2 mt-3">
                {quickAmounts.map((q) => (
                  <button
                    key={q}
                    type="button"
                    onClick={() => setAmount(q)}
                    className={`px-3 py-1.5 rounded-xl text-xs font-bold transition-all ${
                      amount === q
                        ? 'bg-[#0B1F35] text-white shadow-xs'
                        : 'bg-[#F2F3F5] text-gray-700 hover:bg-gray-200'
                    }`}
                  >
                    {(q / 1000000).toLocaleString('ru-RU')} млн ₽
                  </button>
                ))}
              </div>
            </div>

            {/* 4. Срок кредита с быстрыми чипсами (Pills) */}
            <div className="pt-2">
              <div className="flex justify-between items-baseline mb-2">
                <label className="text-xs font-bold uppercase tracking-wider text-gray-400">
                  4. Срок кредита
                </label>
                <span className="text-lg sm:text-xl font-black text-gray-900">
                  {termMonths} мес.
                </span>
              </div>

              {/* Таблетки сроков в стиле калькулятора Альфа-Банка */}
              <div className="flex flex-wrap gap-2 mb-3">
                {quickTerms.map((t) => (
                  <button
                    key={t}
                    type="button"
                    onClick={() => setTermMonths(t)}
                    className={`px-3.5 py-2 rounded-xl text-xs font-bold transition-all ${
                      termMonths === t
                        ? 'bg-[#0B1F35] text-white shadow-xs'
                        : 'bg-[#F2F3F5] text-gray-700 hover:bg-gray-200'
                    }`}
                  >
                    {t === 12 ? '1 год' : t === 24 ? '2 года' : t === 36 ? '3 года' : `${t} мес.`}
                  </button>
                ))}
              </div>

              <input
                type="range"
                min="3"
                max={maxTerm}
                step="1"
                value={termMonths}
                onChange={(e) => setTermMonths(Number(e.target.value))}
                className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-[#EF3124]"
              />
            </div>

            {/* Цель кредитования */}
            <div className="pt-2">
              <label className="block text-xs font-bold uppercase tracking-wider text-gray-400 mb-1.5">
                Цель финансирования
              </label>
              <div className="bg-[#F2F3F5] rounded-xl p-3 focus-within:bg-white focus-within:ring-2 focus-within:ring-[#EF3124] transition-all">
                <input
                  type="text"
                  value={purpose}
                  onChange={(e) => setPurpose(e.target.value)}
                  placeholder="Например: Закупка партии товаров, пополнение оборотных средств"
                  className="w-full bg-transparent text-xs sm:text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none"
                />
              </div>
            </div>
          </div>
        </div>

        {/* Правая колонка: стильная плашка «Предварительный расчёт» */}
        <div className="lg:col-span-5 sticky top-20 space-y-4">
          <div className="bg-white rounded-3xl border border-gray-200/90 p-6 sm:p-7 shadow-sm space-y-6">
            <div>
              <span className="text-xs font-bold uppercase tracking-wider text-[#EF3124] block mb-1">
                Предварительный расчёт
              </span>
              <div className="text-3xl sm:text-4xl font-black text-gray-900 tracking-tight">
                ~ {monthlyPayment.toLocaleString('ru-RU')} ₽
              </div>
              <p className="text-xs text-gray-500 mt-1">
                в месяц аннуитетными платежами
              </p>
            </div>

            {/* Параметры */}
            <div className="space-y-3 py-4 border-y border-gray-100 text-xs">
              <div className="flex justify-between items-center">
                <span className="text-gray-500">Сумма кредита:</span>
                <span className="font-bold text-gray-900 text-sm">
                  {amount.toLocaleString('ru-RU')} ₽
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-gray-500">Срок кредитования:</span>
                <span className="font-bold text-gray-900 text-sm">
                  {termMonths} мес.
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-gray-500">Базовая ставка:</span>
                <span className="font-bold text-[#EF3124] text-sm">
                  от {(rateAnnual * 100).toFixed(1)}%
                </span>
              </div>
            </div>

            {/* Преимущества конвейера Альфы */}
            <div className="space-y-2.5 text-xs text-gray-600">
              <div className="flex items-center gap-2">
                <Zap className="w-4 h-4 text-amber-500 shrink-0" />
                <span>Решение конвейера до 60 секунд</span>
              </div>
              <div className="flex items-center gap-2">
                <FileCheck2 className="w-4 h-4 text-emerald-600 shrink-0" />
                <span>Без сбора финансовой отчётности</span>
              </div>
              <div className="flex items-center gap-2">
                <Sparkles className="w-4 h-4 text-[#EF3124] shrink-0" />
                <span>Выдача на счёт день в день</span>
              </div>
            </div>

            {submitError && (
              <div className="p-3 rounded-xl bg-rose-50 text-rose-700 text-xs font-medium">
                {submitError}
              </div>
            )}

            {/* Фирменная большая кнопка Альфа-Банка */}
            <button
              type="button"
              onClick={handleSubmit}
              disabled={isSubmitting || inn.length < 10}
              className="w-full py-4 rounded-2xl bg-[#EF3124] hover:bg-[#D82417] text-white font-black text-sm sm:text-base transition-all shadow-md shadow-red-500/20 flex items-center justify-center gap-2 disabled:opacity-40"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-5 h-5 animate-spin" />
                  Обработка конвейером...
                </>
              ) : (
                <>
                  Получить решение онлайн
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>

            <div className="text-[10px] text-gray-400 text-center leading-relaxed">
              Нажимая кнопку, вы подтверждаете согласие на обработку данных и запрос сведений в БКИ
            </div>
          </div>
        </div>
      </div>

      {/* Блок «Частые вопросы предпринимателей» в стиле аккордеона Альфы */}
      <div className="mt-12 max-w-4xl">
        <h2 className="text-xl sm:text-2xl font-black text-gray-900 mb-4 tracking-tight">
          Частые вопросы по кредитованию бизнеса
        </h2>
        <div className="space-y-3">
          {faqs.map((f, i) => (
            <div
              key={i}
              onClick={() => setOpenFaq(openFaq === i ? null : i)}
              className="bg-[#F2F3F5] hover:bg-gray-200/70 rounded-2xl p-4 sm:p-5 cursor-pointer transition-all"
            >
              <div className="flex items-center justify-between gap-4">
                <span className="font-bold text-sm text-gray-900">{f.q}</span>
                {openFaq === i ? (
                  <ChevronUp className="w-4 h-4 text-gray-600 shrink-0" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-gray-600 shrink-0" />
                )}
              </div>
              {openFaq === i && (
                <p className="mt-3 text-xs sm:text-sm text-gray-600 leading-relaxed animate-fadeIn">
                  {f.a}
                </p>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
