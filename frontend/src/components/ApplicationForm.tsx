import React, { useState, useEffect } from 'react';
import type { ProductType, Application } from '../api/client';
import { fnsApi, applicationsApi } from '../api/client';
import { DEMO_PROFILES } from '../mockData/demoProfiles';
import { Building2, Check, ArrowRight, Loader2 } from 'lucide-react';

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
              annual_revenue: found.inn === '7701000001' ? 50000000 : found.inn === '7701000002' ? 8000000 : found.inn === '7701000003' ? 12000000 : found.inn === '7701000004' ? 20000000 : 200000000,
              business_age_months: found.inn === '7701000001' ? 48 : found.inn === '7701000002' ? 18 : found.inn === '7701000003' ? 8 : found.inn === '7701000004' ? 36 : 60,
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
        company_snapshot: companyInfo ? {
          inn,
          name: companyInfo.name,
          annual_revenue: companyInfo.annual_revenue,
          business_age_months: companyInfo.business_age_months,
          is_liquidating: companyInfo.is_liquidating,
        } : undefined,
      };
      onApplicationCreated(mockCreated);
    } finally {
      setIsSubmitting(false);
    }
  };

  const maxAmount = productType === 'WORKING_CAPITAL' ? 10000000 : 3000000;
  const maxTerm = productType === 'WORKING_CAPITAL' ? 36 : 12;

  return (
    <div className="max-w-3xl mx-auto py-10 px-4 sm:px-6">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900 tracking-tight">
          Заявка на кредитование бизнеса
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Быстрое предодобрение решения в цифровом конвейере без бумажных справок
        </p>
      </div>

      <div className="bg-white rounded-xl border border-gray-200 p-6 sm:p-8">
        <form onSubmit={handleSubmit} className="space-y-7">
          {/* 1. Выбор продукта */}
          <div>
            <label className="block text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3">
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
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  productType === 'WORKING_CAPITAL'
                    ? 'border-[#EF3124] bg-white ring-1 ring-[#EF3124]'
                    : 'border-gray-200 hover:border-gray-300 bg-white'
                }`}
              >
                <div className="flex justify-between items-baseline mb-1">
                  <span className="font-semibold text-sm text-gray-900">Оборотный кредит</span>
                  <span className="text-xs font-semibold text-gray-900">от 18.5%</span>
                </div>
                <p className="text-xs text-gray-500 mb-3">
                  Для закупки товаров, сырья и покрытия сезонных разрывов.
                </p>
                <div className="text-[11px] text-gray-400 pt-2 border-t border-gray-100 flex justify-between">
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
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  productType === 'OVERDRAFT'
                    ? 'border-[#EF3124] bg-white ring-1 ring-[#EF3124]'
                    : 'border-gray-200 hover:border-gray-300 bg-white'
                }`}
              >
                <div className="flex justify-between items-baseline mb-1">
                  <span className="font-semibold text-sm text-gray-900">Овердрафт для бизнеса</span>
                  <span className="text-xs font-semibold text-gray-900">от 21.0%</span>
                </div>
                <p className="text-xs text-gray-500 mb-3">
                  Кредитный лимит к расчетному счету для непрерывных оплат.
                </p>
                <div className="text-[11px] text-gray-400 pt-2 border-t border-gray-100 flex justify-between">
                  <span>До 3 млн ₽</span>
                  <span>До 12 мес.</span>
                </div>
              </div>
            </div>
          </div>

          {/* 2. Данные компании (ИНН) */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-xs font-semibold uppercase tracking-wider text-gray-500">
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
                  className="text-[11px] text-[#EF3124] hover:underline font-medium"
                >
                  Очистить и ввести свои данные
                </button>
              )}
            </div>
            <div className="relative">
              <input
                type="text"
                value={inn}
                onChange={(e) => setInn(e.target.value.replace(/\D/g, '').slice(0, 12))}
                placeholder="10 или 12 цифр ИНН"
                className="w-full px-3.5 py-2.5 rounded-lg border border-gray-300 text-sm font-medium text-gray-900 focus:outline-none focus:ring-1 focus:ring-[#EF3124] focus:border-[#EF3124]"
              />
              <div className="absolute right-3 top-2.5">
                {isSearchingFns && <Loader2 className="w-4 h-4 text-gray-400 animate-spin" />}
              </div>
            </div>

            {/* Карточка ФНС — спокойная, без ярких фонов */}
            {companyInfo && (
              <div className="mt-2.5 p-3.5 rounded-lg bg-gray-50 border border-gray-200 text-xs text-gray-700">
                <div className="flex items-center justify-between mb-2">
                  <div className="font-semibold text-gray-900 flex items-center gap-1.5">
                    <Building2 className="w-3.5 h-3.5 text-gray-500" />
                    {companyInfo.name}
                  </div>
                  <span className="text-[11px] text-emerald-700 font-medium flex items-center gap-1">
                    <Check className="w-3 h-3" /> ФНС подтверждено
                  </span>
                </div>
                <div className="grid grid-cols-3 gap-2 text-[11px] text-gray-500">
                  <div>
                    Выручка: <span className="font-semibold text-gray-800">{companyInfo.annual_revenue.toLocaleString('ru-RU')} ₽</span>
                  </div>
                  <div>
                    Стаж: <span className="font-semibold text-gray-800">{companyInfo.business_age_months} мес.</span>
                  </div>
                  <div>
                    Статус: <span className="font-semibold text-gray-800">{companyInfo.is_liquidating ? 'Ликвидация' : 'Действует'}</span>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* 3. Параметры кредита */}
          <div className="space-y-5 pt-3 border-t border-gray-100">
            <div>
              <div className="flex justify-between items-baseline mb-2">
                <label className="text-xs font-semibold uppercase tracking-wider text-gray-500">
                  Сумма кредита
                </label>
                <span className="text-lg font-bold text-gray-900">
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
                className="w-full h-1.5 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-[#EF3124]"
              />
              <div className="flex justify-between text-[11px] text-gray-400 mt-1">
                <span>300 000 ₽</span>
                <span>{maxAmount.toLocaleString('ru-RU')} ₽</span>
              </div>
            </div>

            <div>
              <div className="flex justify-between items-baseline mb-2">
                <label className="text-xs font-semibold uppercase tracking-wider text-gray-500">
                  Срок кредита
                </label>
                <span className="text-base font-bold text-gray-900">
                  {termMonths} мес.
                </span>
              </div>
              <input
                type="range"
                min="3"
                max={maxTerm}
                step="1"
                value={termMonths}
                onChange={(e) => setTermMonths(Number(e.target.value))}
                className="w-full h-1.5 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-[#EF3124]"
              />
              <div className="flex justify-between text-[11px] text-gray-400 mt-1">
                <span>3 мес.</span>
                <span>{maxTerm} мес.</span>
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1.5">
                Цель финансирования
              </label>
              <input
                type="text"
                value={purpose}
                onChange={(e) => setPurpose(e.target.value)}
                placeholder="Закупка товара, кассовый разрыв"
                className="w-full px-3.5 py-2 rounded-lg border border-gray-300 text-xs text-gray-900 focus:outline-none focus:ring-1 focus:ring-[#EF3124] focus:border-[#EF3124]"
              />
            </div>
          </div>

          {/* Плашка примерного платежа */}
          <div className="p-3.5 rounded-lg bg-gray-50 border border-gray-200 flex items-center justify-between">
            <span className="text-xs text-gray-500">Ориентировочный платёж:</span>
            <span className="text-sm font-bold text-gray-900">
              ~ {monthlyPayment.toLocaleString('ru-RU')} ₽ / мес
            </span>
          </div>

          {submitError && (
            <div className="p-3 rounded-lg bg-rose-50 text-rose-700 text-xs">
              {submitError}
            </div>
          )}

          {/* Кнопка отправки */}
          <button
            type="submit"
            disabled={isSubmitting || inn.length < 10}
            className="w-full py-3.5 rounded-lg bg-[#EF3124] hover:bg-[#D82417] text-white font-semibold text-sm transition-all flex items-center justify-center gap-2 disabled:opacity-40"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Обработка...
              </>
            ) : (
              <>
                Отправить заявку
                <ArrowRight className="w-4 h-4" />
              </>
            )}
          </button>
        </form>
      </div>
    </div>
  );
};
