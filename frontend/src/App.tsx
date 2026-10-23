import { useState, useEffect } from 'react';
import { DemoBar } from './components/DemoBar';
import { Header } from './components/Header';
import { ApplicationForm } from './components/ApplicationForm';
import { ApplicationTracker } from './components/ApplicationTracker';
import { MyApplicationsList } from './components/MyApplicationsList';
import { UnderwriterDesk } from './components/UnderwriterDesk';
import { LoginModal } from './components/LoginModal';
import { DEMO_PROFILES, type DemoProfile } from './mockData/demoProfiles';
import { authStorage, applicationsApi, type UserSession, type Application } from './api/client';

// Стартовый реестр заявок по демо-кейсам
const INITIAL_APPLICATIONS: Application[] = [
  {
    id: 'app-alfa-7701000001',
    inn: '7701000001',
    status: 'AUTO_APPROVED',
    product_type: 'WORKING_CAPITAL',
    requested_amount: 3000000,
    requested_term_months: 12,
    approved_amount: 3000000,
    approved_term_months: 12,
    interest_rate: 0.185,
    created_at: new Date(Date.now() - 1000 * 60 * 120).toISOString(),
    purpose: 'Закупка запчастей для автопарка',
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
    counter_offer: {
      amount: 3250000,
      term_months: 24,
      rate: 0.205,
      pti: 0.249,
      reason: 'Банк оптимизировал размер платежа под оборот компании',
    },
  },
  {
    id: 'app-techno-7701000003',
    inn: '7701000003',
    status: 'MANUAL_REVIEW',
    product_type: 'OVERDRAFT',
    requested_amount: 2500000,
    requested_term_months: 12,
    created_at: new Date(Date.now() - 1000 * 60 * 15).toISOString(),
    purpose: 'Кассовый разрыв IT-проекта',
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
    reject_reason_client: 'Наличие исторической просрочки в БКИ свыше 90 дней',
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
    counter_offer: {
      amount: 10000000,
      term_months: 24,
      rate: 0.185,
      pti: 0.03,
      reason: 'Банк предлагает максимально доступный лимит продукта 10 млн ₽',
    },
  },
];

export function App() {
  const [session, setSession] = useState<UserSession | null>(() => {
    const saved = authStorage.get();
    if (saved) return saved;
    const defaultProfile = DEMO_PROFILES[0];
    return {
      token: `mock-jwt-${defaultProfile.login}`,
      role: defaultProfile.role,
      inn: defaultProfile.inn,
      display_name: defaultProfile.name,
      login: defaultProfile.login,
    };
  });

  const [activeTab, setActiveTab] = useState<'catalog' | 'applications' | 'underwriter'>('catalog');
  const [activeProfile, setActiveProfile] = useState<DemoProfile | null>(DEMO_PROFILES[0]);
  const [currentApp, setCurrentApp] = useState<Application | null>(null);
  const [isLoginModalOpen, setIsLoginModalOpen] = useState(false);
  const [isDemoMode, setIsDemoMode] = useState(true);

  // Реестр поданных заявок
  const [allApplications, setAllApplications] = useState<Application[]>(INITIAL_APPLICATIONS);

  // Автоматическая загрузка истории заявок клиента с бэкенда
  useEffect(() => {
    if (session?.token && session.role === 'CLIENT') {
      applicationsApi
        .list()
        .then((res) => {
          if (res && res.items && res.items.length > 0) {
            setAllApplications((prev) => {
              // Объединяем полученные с бэкенда заявки
              const existingIds = new Set(res.items.map((i) => i.id));
              const rest = prev.filter((p) => !existingIds.has(p.id));
              return [...res.items, ...rest];
            });
          }
        })
        .catch(() => {
          // Если бэкенд оффлайн, остаемся на локальном реестре
        });
    }
  }, [session?.token, session?.role]);

  // Заявки текущего авторизованного клиента по ИНН
  const myApplications = allApplications.filter(
    (app) => session?.inn && app.inn === session.inn
  );

  const handleProfileSelect = (profile: DemoProfile) => {
    setActiveProfile(profile);
    if (profile.role === 'UNDERWRITER') {
      setActiveTab('underwriter');
    } else {
      setActiveTab('catalog');
      setCurrentApp(null);
    }
  };

  const handleNewApplicationClick = () => {
    setActiveProfile(null);
    setCurrentApp(null);
    setActiveTab('catalog');
  };

  const handleApplicationCreated = (app: Application) => {
    setAllApplications((prev) => [app, ...prev]);
    setCurrentApp(app);
  };

  const handleLogout = () => {
    authStorage.clear();
    setSession(null);
    setActiveProfile(null);
    setCurrentApp(null);
    setActiveTab('catalog');
    setIsLoginModalOpen(false);
  };

  return (
    <div className="min-h-screen flex flex-col bg-[#F4F6F9]">
      {/* 1. Верхний пульт переключения сценариев для жюри (сворачиваемый) */}
      <DemoBar
        currentSession={session}
        onSessionChange={(newSession) => {
          setSession(newSession);
          if (newSession?.role === 'UNDERWRITER') {
            setActiveTab('underwriter');
          }
        }}
        onSelectProfile={handleProfileSelect}
        isDemoMode={isDemoMode}
        onToggleDemoMode={setIsDemoMode}
      />

      {/* 2. Официальная шапка Альфа-Банка */}
      <Header
        session={session}
        activeTab={activeTab}
        onTabChange={(tab) => {
          if (tab === 'catalog') {
            handleNewApplicationClick();
          } else {
            setActiveTab(tab);
            setCurrentApp(null);
          }
        }}
        onOpenLogin={() => setIsLoginModalOpen(true)}
        onLogout={handleLogout}
        applicationsCount={myApplications.length}
      />

      {/* 3. Модальное окно входа */}
      <LoginModal
        isOpen={isLoginModalOpen}
        onClose={() => setIsLoginModalOpen(false)}
        onSuccess={(newSession) => {
          setSession(newSession);
          if (newSession.role === 'UNDERWRITER') {
            setActiveTab('underwriter');
          } else {
            setActiveTab('catalog');
          }
        }}
      />

      {/* 4. Основная рабочая область */}
      <main className="flex-1 pb-16">
        {activeTab === 'underwriter' && session?.role === 'UNDERWRITER' ? (
          <UnderwriterDesk
            currentApp={currentApp}
            onSelectApplication={(app) => setCurrentApp(app)}
            onDecisionApplied={(updated) => {
              setCurrentApp(updated);
              setAllApplications((prev) =>
                prev.map((a) => (a.id === updated.id ? updated : a))
              );
            }}
          />
        ) : activeTab === 'applications' ? (
          <MyApplicationsList
            applications={myApplications}
            companyName={session?.display_name}
            inn={session?.inn}
            onOpenApplication={(app) => {
              setCurrentApp(app);
              setActiveTab('catalog'); // открывает экран трекера
            }}
            onNewApplication={handleNewApplicationClick}
          />
        ) : currentApp ? (
          <ApplicationTracker
            applicationId={currentApp.id}
            initialApplication={currentApp}
            onBackToNew={handleNewApplicationClick}
          />
        ) : (
          <ApplicationForm
            key={activeProfile ? activeProfile.id : 'clean-new-form'}
            initialInn={activeProfile ? activeProfile.inn : ''}
            initialAmount={activeProfile?.defaultAmount || 1000000}
            initialTerm={activeProfile?.defaultTerm || 12}
            initialProduct={activeProfile?.defaultProduct || 'WORKING_CAPITAL'}
            initialPurpose={activeProfile?.purpose || ''}
            onApplicationCreated={handleApplicationCreated}
            onResetToEmpty={() => setActiveProfile(null)}
          />
        )}
      </main>

      {/* 5. Футер */}
      <footer className="border-t border-gray-200 bg-white py-6 text-center text-xs text-gray-500">
        <div className="max-w-7xl mx-auto px-4 flex flex-col sm:flex-row items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="font-bold text-gray-800">Альфа-Будущее</span>
            <span>•</span>
            <span>Хакатон «Умный кредитный кабинет предпринимателя»</span>
          </div>
          <div className="text-gray-400">
            Enterprise Cloud-Native Prototype • Docker Ready
          </div>
        </div>
      </footer>
    </div>
  );
}

export default App;
