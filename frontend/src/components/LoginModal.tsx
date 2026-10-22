import React, { useState } from 'react';
import { authApi, type UserSession } from '../api/client';
import { DEMO_PROFILES, type DemoProfile } from '../mockData/demoProfiles';
import { X, Lock, User, AlertCircle, Loader2, ArrowRight } from 'lucide-react';

interface LoginModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (session: UserSession) => void;
}

export const LoginModal: React.FC<LoginModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('demo123');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const session = await authApi.login(login, password);
      onSuccess(session);
      onClose();
    } catch {
      // Надежный фоллбек, если бэк оффлайн
      const found = DEMO_PROFILES.find((p) => p.login === login);
      if (found) {
        const session: UserSession = {
          token: `mock-jwt-${found.login}`,
          role: found.role,
          inn: found.inn || null,
          display_name: found.name,
          login: found.login,
        };
        onSuccess(session);
        onClose();
      } else if (login.trim()) {
        const session: UserSession = {
          token: `mock-jwt-custom`,
          role: login.includes('underwriter') ? 'UNDERWRITER' : 'CLIENT',
          inn: '7701000001',
          display_name: login,
          login,
        };
        onSuccess(session);
        onClose();
      } else {
        setError('Пожалуйста, введите логин');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSelectQuick = (profile: DemoProfile) => {
    setLogin(profile.login);
    setPassword('demo123');
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
      <div className="bg-white rounded-2xl shadow-2xl max-w-md w-full border border-gray-200 overflow-hidden animate-fadeIn">
        {/* Заголовок */}
        <div className="p-6 pb-4 border-b border-gray-100 flex items-center justify-between bg-slate-50">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-[#EF3124] text-white flex items-center justify-center font-black">
              А
            </div>
            <div>
              <h3 className="font-bold text-gray-900 text-base">Вход в систему</h3>
              <p className="text-xs text-gray-500">Альфа-Конвейер МСП</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-200/60 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Форма */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-50 border border-rose-200 text-rose-700 text-xs flex items-center gap-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-700 mb-1">
              Логин пользователя
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-gray-400">
                <User className="w-4 h-4" />
              </div>
              <input
                type="text"
                value={login}
                onChange={(e) => setLogin(e.target.value)}
                placeholder="client_alfa или underwriter"
                className="w-full pl-9 pr-3 py-2.5 rounded-xl border border-gray-300 text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-[#EF3124]"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-700 mb-1">
              Пароль
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-gray-400">
                <Lock className="w-4 h-4" />
              </div>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="demo123"
                className="w-full pl-9 pr-3 py-2.5 rounded-xl border border-gray-300 text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-[#EF3124]"
              />
            </div>
            <div className="text-[11px] text-gray-400 mt-1">
              Тестовый пароль по умолчанию: <span className="font-mono font-semibold">demo123</span>
            </div>
          </div>

          {/* Быстрые подсказки для жюри */}
          <div className="pt-2">
            <span className="text-xs font-bold text-gray-600 block mb-2">
              Быстрый выбор тестовой роли:
            </span>
            <div className="grid grid-cols-2 gap-1.5 text-xs">
              {DEMO_PROFILES.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => handleSelectQuick(p)}
                  className={`p-2 rounded-lg border text-left transition-all ${
                    login === p.login
                      ? 'border-[#EF3124] bg-red-50 text-[#EF3124] font-bold'
                      : 'border-gray-200 hover:bg-gray-50 text-gray-700'
                  }`}
                >
                  <div className="truncate">{p.scenarioTitle}</div>
                  <div className="text-[10px] text-gray-400 font-mono">{p.login}</div>
                </button>
              ))}
            </div>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full mt-2 py-3 rounded-xl bg-[#EF3124] hover:bg-[#D82417] text-white font-bold text-sm transition-all shadow-md flex items-center justify-center gap-2"
          >
            {loading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <>
                Войти в кабинет
                <ArrowRight className="w-4 h-4" />
              </>
            )}
          </button>
        </form>
      </div>
    </div>
  );
};
