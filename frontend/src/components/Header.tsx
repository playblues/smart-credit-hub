import React from 'react';
import type { UserSession } from '../api/client';
import { Building2, LogOut, FileText, PlusCircle, Shield } from 'lucide-react';

interface HeaderProps {
  session: UserSession | null;
  activeTab: 'catalog' | 'applications' | 'underwriter';
  onTabChange: (tab: 'catalog' | 'applications' | 'underwriter') => void;
  onOpenLogin: () => void;
  onLogout: () => void;
  applicationsCount?: number;
}

export const Header: React.FC<HeaderProps> = ({
  session,
  activeTab,
  onTabChange,
  onOpenLogin,
  onLogout,
  applicationsCount = 0,
}) => {
  return (
    <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
      <div className="max-w-7xl mx-auto px-3 sm:px-6 h-14 flex items-center justify-between gap-2">
        {/* Логотип Альфа-Банка */}
        <div className="flex items-center gap-2 sm:gap-6 shrink-0">
          <div className="flex items-center gap-2 cursor-pointer" onClick={() => onTabChange('catalog')}>
            <div className="w-7 h-7 rounded-md bg-[#EF3124] flex items-center justify-center text-white font-black text-sm shrink-0">
              А
            </div>
            <div>
              <div className="font-bold text-gray-900 text-sm leading-none">
                Альфа-Банк
              </div>
              <div className="text-[10px] text-gray-400 mt-0.5 hidden xs:block">
                Кредитный конвейер
              </div>
            </div>
          </div>

          {/* Навигация: адаптивная (на мобилке компактные иконки, на десктопе с текстом) */}
          <nav className="flex items-center gap-1 text-xs font-medium">
            <button
              onClick={() => onTabChange('catalog')}
              title="Новая заявка"
              className={`px-2.5 sm:px-3 py-1.5 rounded-md transition-colors flex items-center gap-1.5 ${
                activeTab === 'catalog'
                  ? 'bg-gray-100 text-gray-900 font-semibold'
                  : 'text-gray-500 hover:text-gray-900 hover:bg-gray-50'
              }`}
            >
              <PlusCircle className="w-3.5 h-3.5 text-gray-700" />
              <span className="hidden sm:inline">Новая заявка</span>
            </button>

            {/* Вкладка «Мои заявки» для клиента */}
            {session?.role === 'CLIENT' && (
              <button
                onClick={() => onTabChange('applications')}
                title="Мои заявки"
                className={`px-2.5 sm:px-3 py-1.5 rounded-md transition-colors flex items-center gap-1.5 ${
                  activeTab === 'applications'
                    ? 'bg-gray-100 text-gray-900 font-semibold'
                    : 'text-gray-500 hover:text-gray-900 hover:bg-gray-50'
                }`}
              >
                <FileText className="w-3.5 h-3.5 text-gray-700" />
                <span className="hidden sm:inline">Мои заявки</span>
                {applicationsCount > 0 && (
                  <span className="px-1.5 py-0.2 rounded-full bg-slate-200 text-slate-800 text-[10px] font-bold">
                    {applicationsCount}
                  </span>
                )}
              </button>
            )}

            {/* Вкладка «Кредитный эксперт» для сотрудника банка */}
            {session?.role === 'UNDERWRITER' && (
              <button
                onClick={() => onTabChange('underwriter')}
                title="Кабинет кредитного эксперта"
                className={`px-2.5 sm:px-3 py-1.5 rounded-md transition-colors flex items-center gap-1.5 ${
                  activeTab === 'underwriter'
                    ? 'bg-gray-900 text-white font-semibold'
                    : 'text-gray-600 hover:text-gray-900 hover:bg-gray-50'
                }`}
              >
                <Shield className="w-3.5 h-3.5 text-purple-400 sm:hidden" />
                <span className="hidden sm:inline">Кредитный эксперт</span>
                <span className="sm:hidden font-semibold">Эксперт</span>
              </button>
            )}
          </nav>
        </div>

        {/* Профиль и кнопка выхода/смены аккаунта */}
        <div className="flex items-center gap-2 sm:gap-3 text-xs shrink-0">
          {session ? (
            <div className="flex items-center gap-2 sm:gap-3">
              <div className="text-right max-w-[120px] sm:max-w-[200px] truncate">
                <div className="font-medium text-gray-900 leading-tight truncate">
                  {session.display_name || session.login}
                </div>
                {session.inn ? (
                  <div className="text-[11px] text-gray-400 flex items-center gap-1 justify-end">
                    <Building2 className="w-3 h-3 shrink-0" />
                    <span className="truncate">{session.inn}</span>
                  </div>
                ) : (
                  <div className="text-[10px] text-gray-400 uppercase tracking-wider">
                    Служба банка
                  </div>
                )}
              </div>

              <button
                onClick={onLogout}
                title="Сменить аккаунт / Выйти"
                className="p-1.5 rounded-md text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors flex items-center gap-1 border border-transparent hover:border-gray-200 shrink-0"
              >
                <LogOut className="w-4 h-4" />
                <span className="hidden md:inline text-[11px]">Выйти</span>
              </button>
            </div>
          ) : (
            <button
              onClick={onOpenLogin}
              className="px-3.5 py-1.5 rounded-md border border-gray-300 text-xs font-medium text-gray-700 hover:bg-gray-50 transition-colors"
            >
              Войти
            </button>
          )}
        </div>
      </div>
    </header>
  );
};
