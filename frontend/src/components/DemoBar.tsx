import React, { useState } from 'react';
import { DEMO_PROFILES, type DemoProfile } from '../mockData/demoProfiles';
import { authApi, authStorage, type UserSession } from '../api/client';
import { Loader2, Sparkles, ChevronUp, ChevronDown } from 'lucide-react';

interface DemoBarProps {
  currentSession: UserSession | null;
  onSessionChange: (session: UserSession | null) => void;
  onSelectProfile?: (profile: DemoProfile) => void;
  isDemoMode: boolean;
  onToggleDemoMode: (enabled: boolean) => void;
}

export const DemoBar: React.FC<DemoBarProps> = ({
  currentSession,
  onSessionChange,
  onSelectProfile,
  isDemoMode,
  onToggleDemoMode,
}) => {
  const [loadingLogin, setLoadingLogin] = useState<string | null>(null);

  const handleSelect = async (profile: DemoProfile) => {
    setLoadingLogin(profile.login);
    try {
      const session = await authApi.login(profile.login, 'demo123');
      onSessionChange(session);
      if (onSelectProfile) onSelectProfile(profile);
    } catch {
      const fallbackSession: UserSession = {
        token: `mock-jwt-token-for-${profile.login}`,
        role: profile.role,
        inn: profile.inn || null,
        display_name: profile.name,
        login: profile.login,
      };
      authStorage.set(fallbackSession);
      onSessionChange(fallbackSession);
      if (onSelectProfile) onSelectProfile(profile);
    } finally {
      setLoadingLogin(null);
    }
  };

  return (
    <>
      {/* 1. Верхняя панель быстрого перехода (показывается, когда включена) */}
      {isDemoMode && (
        <div className="bg-[#0B1F35] text-white border-b border-slate-800 text-xs py-1.5 px-4 select-none animate-fadeIn">
          <div className="max-w-7xl mx-auto flex items-center justify-between gap-3 overflow-hidden">
            {/* Кнопки быстрого перехода */}
            <div className="flex items-center gap-1.5 flex-nowrap overflow-x-auto no-scrollbar py-0.5">
              <div className="text-[11px] font-semibold text-slate-400 mr-1 flex items-center gap-1 shrink-0">
                <Sparkles className="w-3 h-3 text-amber-400" />
                <span className="font-medium">Быстрый переход:</span>
              </div>

              {DEMO_PROFILES.map((profile) => {
                const isActive = currentSession?.login === profile.login;
                const isLoading = loadingLogin === profile.login;

                return (
                  <button
                    key={profile.id}
                    onClick={() => handleSelect(profile)}
                    disabled={isLoading}
                    title={profile.description}
                    className={`px-2.5 py-1 rounded-md text-xs font-medium transition-all shrink-0 flex items-center gap-1.5 ${
                      isActive
                        ? 'bg-white text-slate-900 shadow-xs font-semibold'
                        : 'text-slate-300 hover:text-white hover:bg-slate-800/80 bg-slate-900/60 border border-slate-800'
                    }`}
                  >
                    {isLoading ? (
                      <Loader2 className="w-3 h-3 animate-spin text-slate-400" />
                    ) : (
                      <span
                        className={`w-1.5 h-1.5 rounded-full ${
                          profile.role === 'UNDERWRITER'
                            ? 'bg-purple-400'
                            : profile.scenarioBadge === 'Авто-одобрение'
                            ? 'bg-emerald-400'
                            : profile.scenarioBadge === 'Отказ БКИ'
                            ? 'bg-rose-400'
                            : 'bg-amber-400'
                        }`}
                      />
                    )}
                    <span>{profile.scenarioTitle}</span>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* 2. Плавающая кнопка в правом нижнем углу (всегда в одном месте переключает пульт) */}
      <button
        onClick={() => onToggleDemoMode(!isDemoMode)}
        className="fixed bottom-4 right-4 z-50 px-3.5 py-2 rounded-full bg-[#0B1F35] text-white text-xs font-medium shadow-lg hover:bg-black transition-all flex items-center gap-2 border border-slate-700 hover:scale-105 select-none"
        title={isDemoMode ? 'Скрыть панель быстрого перехода' : 'Показать панель быстрого перехода'}
      >
        <Sparkles className="w-3.5 h-3.5 text-amber-400" />
        <span>{isDemoMode ? 'Скрыть быстрый переход' : 'Быстрый переход по аккаунтам'}</span>
        {isDemoMode ? (
          <ChevronUp className="w-3.5 h-3.5 text-slate-400" />
        ) : (
          <ChevronDown className="w-3.5 h-3.5 text-slate-400" />
        )}
      </button>
    </>
  );
};
