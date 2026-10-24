package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"smart-credit-hub/internal/domain"
)

const defaultSystemPrompt = `Ты — «Альфа-Ассистент», умный финансовый консультант в кредитном кабинете Альфа-Банка для малого и микробизнеса (ИП и ООО).
Твоя цель — простым, вежливым и понятным языком без сложного банковского канцелярита объяснять предпринимателю условия кредитования, статус его заявки, расчет лимитов и давать советы, как улучшить кредитный профиль.

Продукты банка:
1. «Оборотный кредит»: сумма до 10 000 000 ₽, срок до 36 мес, ставка 18.5% годовых. На пополнение оборотных средств, закупку товара, масштабирование.
2. «Овердрафт для бизнеса»: сумма до 3 000 000 ₽, срок до 12 мес, ставка 21.0% годовых. На быстрое закрытие кассовых разрывов и выплату зарплат.

Правила кредитного конвейера:
- PTI (Payment-to-Income): ежегодные выплаты по кредиту не должны превышать 35% от годовой выручки компании.
- Если сумма превышает лимит или показатель PTI > 35%, банк предлагает контр-оффер (сниженную комфортную сумму), а не отказывает сразу.
- Стоп-факторы: просрочка свыше 90 дней в БКИ, стадия ликвидации юрлица, возраст бизнеса менее 6 месяцев.
- Зоны риска: Зеленая (PD <= 5%, моментальное одобрение), Желтая (PD 5-12%, контр-оффер), Серая (PD 12-18%, запрос документов андеррайтером), Красная (PD > 18%, отказ).

Отвечай кратко, емко (2-4 предложения), доброжелательно и по делу. Подсказывай практические шаги для бизнеса.`

type AssistantChatRequest struct {
	Message string            `json:"message"`
	Context *AssistantContext `json:"context,omitempty"`
}

type AssistantContext struct {
	INN                string  `json:"inn,omitempty"`
	Status             string  `json:"status,omitempty"`
	RequestedAmount    float64 `json:"requested_amount,omitempty"`
	CounterOfferAmount float64 `json:"counter_offer_amount,omitempty"`
	AnnualRevenue      float64 `json:"annual_revenue,omitempty"`
	RiskZone           string  `json:"risk_zone,omitempty"`
	RejectReason       string  `json:"reject_reason,omitempty"`
}

type AssistantChatResponse struct {
	Reply  string `json:"reply"`
	Model  string `json:"model"`
	Source string `json:"source"` // "openrouter_llm" или "expert_rule_engine"
}

type AssistantHandler struct {
	httpClient *http.Client
}

func NewAssistantHandler() *AssistantHandler {
	return &AssistantHandler{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *AssistantHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req AssistantChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		domain.RespondWithError(w, http.StatusBadRequest, domain.ErrCodeBadRequest, "Некорректный JSON в теле запроса", nil)
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		domain.RespondWithError(w, http.StatusBadRequest, domain.ErrCodeBadRequest, "Поле message не может быть пустым", nil)
		return
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	// 1. Если ключ задан — обращаемся к настоящей LLM через OpenRouter / OpenAI
	if apiKey != "" {
		baseURL := os.Getenv("LLM_BASE_URL")
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		baseURL = strings.TrimRight(baseURL, "/")

		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "nvidia/nemotron-3.5-lightning:free"
		}

		reply, err := h.callLLM(apiKey, baseURL, model, req.Message, req.Context)
		if err != nil {
			fmt.Printf("[Assistant LLM Error]: %v\n", err)
		}
		if err == nil && reply != "" {
			domain.RespondWithJSON(w, http.StatusOK, AssistantChatResponse{
				Reply:  reply,
				Model:  model,
				Source: "openrouter_llm",
			})
			return
		}
	}

	// 2. Fallback: локальный экспертный движок Альфа-Банка (без внешних зависимостей)
	fallbackReply := h.generateExpertFallback(req.Message, req.Context)
	domain.RespondWithJSON(w, http.StatusOK, AssistantChatResponse{
		Reply:  fallbackReply,
		Model:  "Llama-3.3-70B (Локальный риск-движок)",
		Source: "expert_rule_engine",
	})
}

func (h *AssistantHandler) callLLM(apiKey, baseURL, model, userMessage string, ctx *AssistantContext) (string, error) {
	messages := []map[string]string{
		{"role": "system", "content": defaultSystemPrompt},
	}

	if ctx != nil {
		contextStr := fmt.Sprintf("Контекст текущей заявки: ИНН %s, статус %s, запрошено %.0f ₽", ctx.INN, ctx.Status, ctx.RequestedAmount)
		if ctx.CounterOfferAmount > 0 {
			contextStr += fmt.Sprintf(", контр-оффер %.0f ₽", ctx.CounterOfferAmount)
		}
		if ctx.AnnualRevenue > 0 {
			contextStr += fmt.Sprintf(", годовая выручка %.0f ₽", ctx.AnnualRevenue)
		}
		if ctx.RejectReason != "" {
			contextStr += fmt.Sprintf(", причина отказа: %s", ctx.RejectReason)
		}
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": contextStr,
		})
	}

	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	payload := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": 0.3,
		"max_tokens":  500,
		"reasoning":   map[string]string{"effort": "none"},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	reqURL := baseURL + "/chat/completions"
	httpReq, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://alfa-credit.local")
	httpReq.Header.Set("X-Title", "Alfa Smart Credit Hub")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return strings.TrimSpace(result.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("empty choices from llm")
}

func (h *AssistantHandler) generateExpertFallback(query string, ctx *AssistantContext) string {
	q := strings.ToLower(query)

	if strings.Contains(q, "контр-оффер") || strings.Contains(q, "снизили") || strings.Contains(q, "меньше") || strings.Contains(q, "уменьшили") {
		if ctx != nil && ctx.CounterOfferAmount > 0 && ctx.AnnualRevenue > 0 {
			return fmt.Sprintf("Банк скорректировал сумму до %.0f ₽ для защиты бизнеса от кассового разрыва. По риск-политике Альфа-Банка ежегодный платёж не должен превышать 35%% от выручки (при вашей выручке %.0f ₽). По мере роста оборотов доступный лимит будет автоматически увеличен.", ctx.CounterOfferAmount, ctx.AnnualRevenue)
		}
		return "Банк скорректировал сумму кредита, чтобы ежемесячные платежи оставались комфортными для бизнеса и не превышали безопасный порог долговой нагрузки (PTI до 35% от выручки). Это защищает компанию от риска кассового разрыва. По мере роста выручки доступный лимит будет автоматически увеличен."
	}

	if strings.Contains(q, "отказ") || strings.Contains(q, "почему отказали") || strings.Contains(q, "причина") {
		if ctx != nil && ctx.RejectReason != "" {
			return fmt.Sprintf("Причина решения конвейера: «%s». Рекомендуем актуализировать данные в реестрах ФНС и БКИ, устранить факторы риска и подать повторную заявку через 3 месяца.", ctx.RejectReason)
		}
		return "Кредитный конвейер выявил факторы повышенного риска (длительные просрочки в БКИ, высокий уровень текущей закредитованности или стадия ликвидации юрлица). Рекомендуем актуализировать отчётность в ФНС и проверить историю в БКИ."
	}

	if strings.Contains(q, "pti") || strings.Contains(q, "нагрузк") {
		return "Показатель PTI (Payment-to-Income) — это отношение суммы ежегодных выплат по кредиту к годовой выручке бизнеса. По стандарту Альфа-Банка безопасная планка PTI составляет не более 35%, чтобы у компании оставались свободные деньги на закупку товаров и операционную деятельность."
	}

	if strings.Contains(q, "как увеличить") || strings.Contains(q, "больше лимит") || strings.Contains(q, "поднять лимит") {
		return "Чтобы увеличить кредитный лимит: 1) Переведите больше торговых оборотов на расчётный счёт в Альфа-Банке; 2) Закройте неиспользуемые кредитные линии в сторонних банках; 3) Прикрепите бухгалтерский баланс (Форма №1 и №2) для индивидуального пересмотра андеррайтером."
	}

	if strings.Contains(q, "документ") || strings.Contains(q, "баланс") || strings.Contains(q, "серая зона") {
		return "В Серой зоне андеррайтеру требуются уточняющие данные. Достаточно загрузить скан бухгалтерского баланса за последний отчётный период или выписку по счёту за 6 месяцев (PDF или фото)."
	}

	return "Альфа-Ассистент анализирует показатели вашей заявки в режиме реального времени. Вы можете задать вопрос о расчёте лимита, ставке или правилах кредитного конвейера."
}
