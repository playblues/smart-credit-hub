import React from 'react';
import type { Application } from '../api/client';
import { STATUS_RU } from '../utils/formatters';
import { FileText, Clock, PlusCircle, ChevronRight } from 'lucide-react';

interface MyApplicationsListProps {
  applications: Application[];
  onOpenApplication: (app: Application) => void;
  onNewApplication: () => void;
  companyName?: string;
  inn?: string | null;
}

export const MyApplicationsList: React.FC<MyApplicationsListProps> = ({
  applications,
  onOpenApplication,
  onNewApplication,
  companyName,
  inn,
}) => {
  return (
    <div className="max-w-4xl mx-auto py-10 px-4 sm:px-6">
      <div className="flex flex-wrap items-center justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 tracking-tight">
            История кредитных заявок
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            {companyName || 'Ваша организация'} {inn ? `(ИНН ${inn})` : ''}
          </p>
        </div>

        <button
          onClick={onNewApplication}
          className="px-4 py-2 rounded-lg bg-[#EF3124] hover:bg-[#D82417] text-white text-xs font-semibold transition-all flex items-center gap-1.5 shadow-xs"
        >
          <PlusCircle className="w-4 h-4" />
          Подать новую заявку
        </button>
      </div>

      {applications.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center">
          <FileText className="w-10 h-10 text-gray-300 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-gray-800 mb-1">
            У вас пока нет поданных заявок
          </h3>
          <p className="text-xs text-gray-500 max-w-sm mx-auto mb-6">
            Оформите заявку на оборотный кредит или овердрафт за пару минут с моментальным решением цифрового конвейера.
          </p>
          <button
            onClick={onNewApplication}
            className="px-4 py-2 rounded-lg bg-gray-900 hover:bg-black text-white text-xs font-medium transition-colors"
          >
            Оформить первую заявку
          </button>
        </div>
      ) : (
        <div className="space-y-3">
          {applications.map((app) => {
            const statusConfig = STATUS_RU[app.status];
            return (
              <div
                key={app.id}
                onClick={() => onOpenApplication(app)}
                className="group bg-white rounded-xl border border-gray-200 hover:border-gray-300 p-5 transition-all cursor-pointer hover:shadow-xs flex items-center justify-between gap-4"
              >
                <div className="space-y-1.5 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-gray-400">
                      #{app.id.slice(-6)}
                    </span>
                    <span className="text-xs font-medium text-gray-700">
                      {app.product_type === 'WORKING_CAPITAL' ? 'Оборотный кредит' : 'Овердрафт для бизнеса'}
                    </span>
                    <span
                      className={`px-2 py-0.5 rounded text-[11px] font-semibold border ${statusConfig.badgeClass}`}
                    >
                      {statusConfig.label}
                    </span>
                  </div>

                  <div className="flex items-baseline gap-2">
                    <span className="text-xl font-bold text-gray-900">
                      {(app.approved_amount || app.requested_amount).toLocaleString('ru-RU')} ₽
                    </span>
                    <span className="text-xs text-gray-500">
                      на {app.approved_term_months || app.requested_term_months} мес.
                    </span>
                    {app.interest_rate && (
                      <span className="text-xs text-gray-500">
                        под {(app.interest_rate * 100).toFixed(1)}%
                      </span>
                    )}
                  </div>

                  <div className="text-[11px] text-gray-400 flex items-center gap-2">
                    <Clock className="w-3 h-3" />
                    <span>Подана: {new Date(app.created_at || Date.now()).toLocaleDateString('ru-RU')}</span>
                    {app.purpose && <span>• {app.purpose}</span>}
                  </div>
                </div>

                <ChevronRight className="w-5 h-5 text-gray-300 group-hover:text-gray-600 group-hover:translate-x-0.5 transition-all shrink-0" />
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
