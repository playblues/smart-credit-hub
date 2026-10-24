import React, { useState } from 'react';
import { Sparkles, Bot, Send, MessageSquare, ChevronDown, ChevronUp, User } from 'lucide-react';
import type { Application } from '../api/client';
import { apiClient } from '../api/client';

interface AlfaAssistantWidgetProps {
  application?: Application | null;
}

interface ChatMessage {
  sender: 'user' | 'assistant';
  text: string;
}

const FALLBACK_ANSWERS: { keywords: string[]; answer: string }[] = [
  {
    keywords: ['контр-оффер', 'снизили', 'меньше', 'почему меньше', 'уменьшили', 'предложили меньше', '3250000', '3 250 000'],
    answer:
      'Банк скорректировал сумму кредита, чтобы ежемесячные платежи оставались комфортными для бизнеса и не превышали безопасный порог долговой нагрузки (PTI до 35% от годовой выручки). Это защищает компанию от риска кассового разрыва. По мере роста выручки доступный лимит будет автоматически увеличен.',
  },
  {
    keywords: ['отказ', 'почему отказали', 'причина отказа', 'отклонили', 'просрочк'],
    answer:
      'Кредитный конвейер выявил факторы повышенного риска (например, наличие исторических просрочек в БКИ, высокую текущую долговую нагрузку или срок ведения бизнеса менее 6 месяцев). Рекомендуем актуализировать данные в БКИ, закрыть текущие обязательства и подать повторную заявку через 3 месяца.',
  },
  {
    keywords: ['pti', 'нагрузк', 'долговая нагрузка'],
    answer:
      'Показатель PTI (Payment-to-Income) — это отношение ежегодных выплат по кредиту к годовой выручке бизнеса. По стандарту риск-политики Альфа-Банка платежи не должны превышать 35% от выручки, чтобы у компании оставались свободные деньги на операционные расходы.',
  },
  {
    keywords: ['как увеличить', 'больше лимит', 'поднять лимит', 'мало дали'],
    answer:
      'Чтобы увеличить кредитный лимит: 1) Переведите больше оборотов компании на расчетный счет в Альфа-Банке; 2) Закройте неиспользуемые кредитные карты и овердрафты в других банках; 3) Прикрепите бухгалтерский баланс (Форма №1 и №2) — андеррайтер сможет пересмотреть лимит вручную.',
  },
  {
    keywords: ['документ', 'баланс', 'выписк', 'что прикрепить', 'серая зона'],
    answer:
      'В Серой зоне андеррайтеру нужны уточняющие данные. Достаточно загрузить скан бухгалтерского баланса за последний год или выписку по расчетному счету за последние 6 месяцев (в формате PDF, PNG или JPEG).',
  },
  {
    keywords: ['овердрафт', 'оборотный', 'что выбрать', 'разница'],
    answer:
      '«Оборотный кредит» (до 10 млн ₽ до 36 мес под 18.5%) предназначен для крупных закупок товара и масштабирования. «Овердрафт» (до 3 млн ₽ до 12 мес под 21%) идеален для быстрого покрытия кассовых разрывов, выплат налогов и зарплаты.',
  },
];

export const AlfaAssistantWidget: React.FC<AlfaAssistantWidgetProps> = ({ application }) => {
  const [isOpen, setIsOpen] = useState(true);
  const [modelName, setModelName] = useState('Llama 3.3 70B (OpenRouter / Риск-модель)');
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      sender: 'assistant',
      text: 'Здравствуйте! Я Альфа-Ассистент — ваш персональный ИИ-консультант. Помогу разобраться в условиях кредита, расчёте лимита PTI и решении конвейера. О чем хотите узнать?',
    },
  ]);
  const [input, setInput] = useState('');
  const [isTyping, setIsTyping] = useState(false);

  const getAssistantResponse = (query: string): string => {
    const qLower = query.toLowerCase();

    for (const item of FALLBACK_ANSWERS) {
      if (item.keywords.some((k) => qLower.includes(k))) {
        return item.answer;
      }
    }

    if (application?.status === 'COUNTER_OFFERED') {
      return `По вашей заявке предложен контр-оффер на сумму ${application.counter_offer?.amount?.toLocaleString('ru-RU')} ₽ со ставкой ${((application.counter_offer?.rate || 0.185) * 100).toFixed(1)}%. Причина: соблюдение норматива долговой нагрузки PTI ≤ 35% для вашей выручки (${application.company_snapshot?.annual_revenue?.toLocaleString('ru-RU')} ₽). Вы можете принять предложение в 1 клик или запросить пересмотр.`;
    }

    if (application?.status === 'REJECTED') {
      return `Заявка отклонена конвейером по причине: "${application.reject_reason_client || 'Факторы риска БКИ/ФНС'}". Рекомендуем проверить кредитную историю компании в БКИ и актуализировать данные в реестре ФНС.`;
    }

    return 'Альфа-Банк оценивает заявку автоматически по открытым реестрам ФНС и БКИ в реальном времени. Если у вас есть вопросы по ставке, сроку или расчету лимита — выберите вопрос выше или напишите подробнее!';
  };

  const handleSend = async (textToSend?: string) => {
    const text = (textToSend || input).trim();
    if (!text) return;

    const userMsg: ChatMessage = { sender: 'user', text };
    setMessages((prev) => [...prev, userMsg]);
    if (!textToSend) setInput('');
    setIsTyping(true);

    try {
      const res = await apiClient.post<{ reply: string; model: string; source: string }>(
        '/assistant/chat',
        {
          message: text,
          context: {
            inn: application?.inn,
            status: application?.status,
            requested_amount: application?.requested_amount,
            counter_offer_amount: application?.counter_offer?.amount,
            annual_revenue: application?.company_snapshot?.annual_revenue,
            risk_zone: application?.scoring_result?.risk_zone,
            reject_reason: application?.reject_reason_client,
          },
        }
      );
      if (res.data?.model) {
        setModelName(res.data.model);
      }
      setMessages((prev) => [...prev, { sender: 'assistant', text: res.data.reply }]);
    } catch {
      // Офлайн фоллбек, если сервер недоступен
      const reply = getAssistantResponse(text);
      setMessages((prev) => [...prev, { sender: 'assistant', text: reply }]);
    } finally {
      setIsTyping(false);
    }
  };

  const quickPrompts = [
    'Почему предложили контр-оффер?',
    'Что такое показатель PTI (35%)?',
    'Как увеличить кредитный лимит?',
    'Что прикрепить в Серой зоне?',
  ];

  return (
    <div className="bg-gradient-to-br from-slate-900 via-[#0B1F35] to-slate-900 rounded-3xl p-5 sm:p-6 text-white shadow-xl border border-slate-800 animate-fadeIn mt-6">
      {/* Шапка ассистента */}
      <div
        className="flex items-center justify-between cursor-pointer select-none"
        onClick={() => setIsOpen(!isOpen)}
      >
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-[#EF3124] to-red-500 flex items-center justify-center text-white shadow-md shadow-red-500/30">
            <Sparkles className="w-5 h-5 text-amber-200" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-extrabold text-sm sm:text-base text-white">
                Альфа-Ассистент
              </h3>
              <span className="text-[10px] bg-red-500/20 text-red-300 border border-red-500/30 font-bold px-2 py-0.5 rounded-full">
                {modelName}
              </span>
            </div>
            <p className="text-xs text-slate-400">
              Персональный риск-консультант кредитного конвейера
            </p>
          </div>
        </div>

        <button className="text-slate-400 hover:text-white p-1 rounded-lg">
          {isOpen ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
        </button>
      </div>

      {/* Тело виджета */}
      {isOpen && (
        <div className="mt-4 pt-4 border-t border-slate-800/80 space-y-4">
          {/* Быстрые чипсы-вопросы */}
          <div className="flex flex-wrap gap-1.5">
            {quickPrompts.map((q, idx) => (
              <button
                key={idx}
                type="button"
                onClick={() => handleSend(q)}
                className="px-3 py-1.5 rounded-xl bg-slate-800/90 hover:bg-slate-700/90 text-[11px] font-medium text-slate-200 border border-slate-700/60 transition-all hover:border-slate-500 flex items-center gap-1.5"
              >
                <MessageSquare className="w-3 h-3 text-[#EF3124]" />
                {q}
              </button>
            ))}
          </div>

          {/* Окно сообщений */}
          <div className="bg-slate-950/60 rounded-2xl p-4 max-h-72 overflow-y-auto space-y-3 border border-slate-800/50">
            {messages.map((m, idx) => (
              <div
                key={idx}
                className={`flex gap-2.5 ${
                  m.sender === 'user' ? 'justify-end' : 'justify-start'
                }`}
              >
                {m.sender === 'assistant' && (
                  <div className="w-6 h-6 rounded-lg bg-[#EF3124] text-white flex items-center justify-center shrink-0 text-xs font-bold mt-0.5">
                    А
                  </div>
                )}
                <div
                  className={`p-3 rounded-2xl text-xs sm:text-[13px] leading-relaxed max-w-[85%] ${
                    m.sender === 'user'
                      ? 'bg-[#EF3124] text-white font-medium rounded-tr-xs'
                      : 'bg-slate-800/90 text-slate-100 border border-slate-700/60 rounded-tl-xs shadow-xs'
                  }`}
                >
                  {m.text}
                </div>
                {m.sender === 'user' && (
                  <div className="w-6 h-6 rounded-lg bg-slate-700 text-slate-300 flex items-center justify-center shrink-0 mt-0.5">
                    <User className="w-3.5 h-3.5" />
                  </div>
                )}
              </div>
            ))}

            {isTyping && (
              <div className="flex items-center gap-2 text-xs text-slate-400 italic py-1">
                <Bot className="w-4 h-4 text-amber-400 animate-pulse" />
                <span>Альфа-Ассистент анализирует заявку...</span>
              </div>
            )}
          </div>

          {/* Поле ввода */}
          <form
            onSubmit={(e) => {
              e.preventDefault();
              handleSend();
            }}
            className="flex items-center gap-2"
          >
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="Спросите у ассистента о решении, ставке или лимитах..."
              className="flex-1 px-4 py-2.5 rounded-xl bg-slate-800/90 border border-slate-700 text-xs sm:text-sm text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-[#EF3124]"
            />
            <button
              type="submit"
              disabled={!input.trim()}
              className="px-4 py-2.5 rounded-xl bg-[#EF3124] hover:bg-[#D82417] text-white text-xs font-bold transition-all shadow-md shadow-red-500/20 disabled:opacity-40 flex items-center gap-1.5"
            >
              <span>Спросить</span>
              <Send className="w-3.5 h-3.5" />
            </button>
          </form>
        </div>
      )}
    </div>
  );
};
