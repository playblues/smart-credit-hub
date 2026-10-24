export interface DemoProfile {
  id: string;
  login: string;
  name: string;
  inn: string;
  scenarioTitle: string;
  scenarioBadge: string;
  badgeVariant: 'green' | 'yellow' | 'grey' | 'red' | 'blue' | 'purple';
  description: string;
  role: 'CLIENT' | 'UNDERWRITER';
  defaultAmount?: number;
  defaultTerm?: number;
  defaultProduct?: 'WORKING_CAPITAL' | 'OVERDRAFT';
  purpose?: string;
}

export const DEMO_PROFILES: DemoProfile[] = [
  {
    id: 'case-1',
    login: 'client_alfa',
    name: 'ООО «Альфа Логистика»',
    inn: '7701000001',
    scenarioTitle: 'Альфа Логистика',
    scenarioBadge: 'Авто-одобрение',
    badgeVariant: 'green',
    description: 'Зрелый бизнес (4 года), выручка 50 млн ₽. Быстрое одобрение без залога.',
    role: 'CLIENT',
    defaultAmount: 3000000,
    defaultTerm: 12,
    defaultProduct: 'WORKING_CAPITAL',
    purpose: 'Закупка запчастей для автопарка',
  },
  {
    id: 'case-2',
    login: 'client_smirnov',
    name: 'ИП Смирнов А.П.',
    inn: '7701000002',
    scenarioTitle: 'ИП Смирнов',
    scenarioBadge: 'Контр-оффер',
    badgeVariant: 'yellow',
    description: 'Запрос 5 млн при обороте 8 млн. Банк предлагает комфортные 3.25 млн ₽.',
    role: 'CLIENT',
    defaultAmount: 5000000,
    defaultTerm: 24,
    defaultProduct: 'WORKING_CAPITAL',
    purpose: 'Расширение торговой точки',
  },
  {
    id: 'case-3',
    login: 'client_technostart',
    name: 'ООО «ТехноСтарт»',
    inn: '7701000003',
    scenarioTitle: 'ТехноСтарт',
    scenarioBadge: 'Серая зона',
    badgeVariant: 'grey',
    description: 'Молодой IT-бизнес (8 мес). Требуется проверка андеррайтером и баланс.',
    role: 'CLIENT',
    defaultAmount: 2500000,
    defaultTerm: 12,
    defaultProduct: 'OVERDRAFT',
    purpose: 'Покрытие кассового разрыва',
  },
  {
    id: 'case-4',
    login: 'client_negativ',
    name: 'ООО «Негатив Трейд»',
    inn: '7701000004',
    scenarioTitle: 'Негатив Трейд',
    scenarioBadge: 'Отказ БКИ',
    badgeVariant: 'red',
    description: 'Просрочка 115 дней в БКИ. Срабатывает стоп-фактор с понятным объяснением.',
    role: 'CLIENT',
    defaultAmount: 4000000,
    defaultTerm: 12,
    defaultProduct: 'WORKING_CAPITAL',
    purpose: 'Пополнение оборотных средств',
  },
  {
    id: 'case-5',
    login: 'client_megastroy',
    name: 'ООО «Мегастрой»',
    inn: '7701000005',
    scenarioTitle: 'Мегастрой',
    scenarioBadge: 'Лимит продукта',
    badgeVariant: 'blue',
    description: 'Запрос 15 млн при потолке 10 млн. Предложение максимального лимита.',
    role: 'CLIENT',
    defaultAmount: 15000000,
    defaultTerm: 24,
    defaultProduct: 'WORKING_CAPITAL',
    purpose: 'Закупка строительного оборудования',
  },
  {
    id: 'case-underwriter',
    login: 'underwriter',
    name: 'Кредитный эксперт Альфа-Банка',
    inn: '',
    scenarioTitle: 'Кабинет андеррайтера',
    scenarioBadge: 'Кредитный эксперт',
    badgeVariant: 'purple',
    description: 'Очередь заявок, проверка факторов риска, запрос документов и принятие решений.',
    role: 'UNDERWRITER',
  },
];
