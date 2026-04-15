/**
 * ================================================
 * LUA AI AGENT - ФРОНТЕНД ДЛЯ ХАКАТОНА MTS
 * ================================================
 *
 * API эндпоинты:
 * - POST /generate  - генерация кода
 * - POST /feedback  - отправка фидбека/уточнения
 * - GET  /history   - получение истории
 * - GET  /stats     - получение статистики
 *
 * Бекенд запускается на: http://localhost:8080
 */

// ================================================
// 1. КОНФИГУРАЦИЯ И ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ
// ================================================

const API_BASE_URL = 'http://localhost:8080' // Адрес бекенда
let currentSessionId = null // ID текущей сессии
let currentCode = null // Последний сгенерированный код
let isAwaitingClarification = false // Ожидаем уточнение от пользователя
let pendingQuestion = null // Сохраняем вопрос бота

// DOM элементы
const userInput = document.getElementById('userInput')
const sendBtn = document.getElementById('sendBtn')
const chatMessages = document.getElementById('chatMessages')
const historyBtn = document.getElementById('historyBtn')
const timerValue = document.getElementById('timerValue')
const thinkingText = document.getElementById('thinkingText')

// ================================================
// 2. ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ================================================

/**
 * Генерация уникального ID сессии (UUID v4)
 */
function generateSessionId() {
	return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
		const r = (Math.random() * 16) | 0
		const v = c === 'x' ? r : (r & 0x3) | 0x8
		return v.toString(16)
	})
}
function saveSessionId() {
	localStorage.setItem('mws_session_id', currentSessionId)
}

// Загрузка session_id
function loadSessionId() {
	const saved = localStorage.getItem('mws_session_id')
	if (saved) {
		currentSessionId = saved
	} else {
		currentSessionId = generateSessionId()
		localStorage.setItem('mws_session_id', currentSessionId)
	}
}

/**
 * Экранирование HTML символов (безопасность)
 */
function escapeHtml(text) {
	if (!text) return ''
	const div = document.createElement('div')
	div.textContent = text
	return div.innerHTML
}

/**
 * Форматирование кода для отображения с подсветкой
 */
function formatCodeBlock(code, language = 'lua') {
	if (!code) return ''
	return `<pre><code class="language-${language}">${escapeHtml(code)}</code></pre>`
}

/**
 * Обновление текста "о чем думаю"
 */
function updateThinkingText(text) {
	if (thinkingText) {
		thinkingText.textContent = text
	}
}

/**
 * Прокрутка чата вниз
 */
function scrollChatToBottom() {
	if (chatMessages) {
		chatMessages.scrollTop = chatMessages.scrollHeight
	}
}

// ================================================
// 3. УПРАВЛЕНИЕ ТАЙМЕРОМ
// ================================================

let startTime = null
let timerInterval = null

function startTimer() {
	if (timerInterval) stopTimer()
	startTime = Date.now()
	timerInterval = setInterval(() => {
		const elapsed = Date.now() - startTime
		if (timerValue) timerValue.textContent = elapsed
	}, 10)
}

function stopTimer() {
	if (timerInterval) {
		clearInterval(timerInterval)
		timerInterval = null
	}
}

function resetTimer() {
	stopTimer()
	if (timerValue) timerValue.textContent = '0'
}

function setTimerValue(ms) {
	if (timerValue) timerValue.textContent = ms
}

// ================================================
// 4. РАБОТА С СООБЩЕНИЯМИ В ЧАТЕ
// ================================================

/**
 * Добавление сообщения пользователя
 */
function addUserMessage(text) {
	const messageDiv = document.createElement('div')
	messageDiv.className = 'message user'

	const now = new Date()
	const timeStr = now.toLocaleTimeString('ru-RU', {
		hour: '2-digit',
		minute: '2-digit',
	})

	messageDiv.innerHTML = `
        <div class="message-avatar">👤</div>
        <div class="message-content-wrapper">
            <div class="message-content">${escapeHtml(text)}</div>
            <div class="message-time">${timeStr}</div>
        </div>
    `

	chatMessages.appendChild(messageDiv)
	scrollChatToBottom()
	saveChat()
}

/**
 * Добавление сообщения бота
 */
function addBotMessage(text, isCode = false) {
	const messageDiv = document.createElement('div')
	messageDiv.className = 'message bot'

	const now = new Date()
	const timeStr = now.toLocaleTimeString('ru-RU', {
		hour: '2-digit',
		minute: '2-digit',
	})

	let content = ''
	if (isCode) {
		content = formatCodeBlock(text, 'lua')
	} else {
		content = escapeHtml(text).replace(/\n/g, '<br>')
	}

	messageDiv.innerHTML = `
        <div class="message-avatar">🤖</div>
        <div class="message-content-wrapper">
            <div class="message-content">${content}</div>
            <div class="message-time">${timeStr}</div>
        </div>
    `

	chatMessages.appendChild(messageDiv)
	scrollChatToBottom()
	saveChat()
}

/**
 * Добавление сообщения с кодом и объяснением
 */
function addGeneratedCodeResponse(data) {
	let message = ''

	// Добавляем код если есть
	if (data.code) {
		currentCode = data.code
		message += `**Сгенерированный код:**\n\n\`\`\`lua\n${data.code}\n\`\`\`\n\n`
	}

	// Добавляем объяснение если есть
	if (data.explanation) {
		message += `**📖 Объяснение:**\n${data.explanation}\n\n`
	}

	// Добавляем план действий если есть
	if (data.plan && data.plan.length > 0) {
		message += `**📋 План действий:**\n`
		data.plan.forEach((step, i) => {
			message += `${i + 1}. ${step}\n`
		})
		message += `\n`
	}

	// Добавляем информацию о времени
	if (data.execution_time_ms) {
		message += `⏱ Время генерации: ${data.execution_time_ms} мс`
	}

	if (message) {
		addBotMessage(message)
	}
}

/**
 * Показ индикатора печати (бот думает)
 */
function showTypingIndicator() {
	const indicator = document.createElement('div')
	indicator.className = 'message bot'
	indicator.id = 'typingIndicator'
	indicator.innerHTML = `
        <div class="message-avatar">🤖</div>
        <div class="message-content-wrapper">
            <div class="message-content">
                <div class="typing-indicator">
                    <span></span><span></span><span></span>
                </div>
            </div>
        </div>
    `
	chatMessages.appendChild(indicator)
	scrollChatToBottom()
	return indicator
}

function removeTypingIndicator() {
	const indicator = document.getElementById('typingIndicator')
	if (indicator) indicator.remove()
}

// ================================================
// 5. API ЗАПРОСЫ К БЕКЕНДУ
// ================================================

/**
 * POST /generate - генерация Lua-кода
 */
async function generateCode(prompt) {
	const response = await fetch(`${API_BASE_URL}/generate`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify({
			prompt: prompt,
			session_id: currentSessionId,
		}),
	})

	if (!response.ok) {
		const error = await response.json()
		throw new Error(error.message || 'Ошибка генерации')
	}

	return await response.json()
}

/**
 * POST /feedback - отправка фидбека или ответ на уточнение
 */
async function sendFeedback(feedback, previousCode) {
	const response = await fetch(`${API_BASE_URL}/feedback`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify({
			session_id: currentSessionId,
			feedback: feedback,
			previous_code: previousCode || '',
		}),
	})

	if (!response.ok) {
		const error = await response.json()
		throw new Error(error.message || 'Ошибка отправки фидбека')
	}

	return await response.json()
}

/**
 * GET /history - получение истории генераций
 */
async function fetchHistory(limit = 20) {
	try {
		const response = await fetch(
			`${API_BASE_URL}/history?limit=${limit}&session_id=${currentSessionId}`,
		)
		if (!response.ok) throw new Error('Ошибка загрузки истории')
		const data = await response.json()
		return data.entries || []
	} catch (error) {
		console.error('Ошибка загрузки истории:', error)
		return []
	}
}

/**
 * GET /stats - получение статистики
 */
async function fetchStats() {
	try {
		const response = await fetch(
			`${API_BASE_URL}/stats?session_id=${currentSessionId}`,
		)
		if (!response.ok) throw new Error('Ошибка загрузки статистики')
		return await response.json()
	} catch (error) {
		console.error('Ошибка загрузки статистики:', error)
		return null
	}
}

// ================================================
// 6. ОСНОВНАЯ ЛОГИКА ОБРАБОТКИ СООБЩЕНИЙ
// ================================================

/**
 * Отправка сообщения пользователя
 */
async function sendMessage() {
	const text = userInput.value.trim()
	if (!text) return

	// Если ожидаем уточнение - отправляем как фидбек
	if (isAwaitingClarification) {
		await sendClarificationFeedback(text)
		return
	}

	// Обычная генерация
	addUserMessage(text)
	userInput.value = ''
	userInput.style.height = 'auto'

	updateThinkingText('Генерирую код... 🤔')
	startTimer()
	showTypingIndicator()

	try {
		const response = await generateCode(text)
		stopTimer()
		removeTypingIndicator()
		setTimerValue(response.execution_time_ms || 0)

		if (response.success) {
			// Проверяем, нужно ли уточнение
			if (response.needs_clarification && response.question) {
				// Бот задает уточняющий вопрос
				isAwaitingClarification = true
				pendingQuestion = response.question
				addBotMessage(
					`❓ **Уточняющий вопрос:**\n\n${response.question}\n\nПожалуйста, уточните ваш запрос.`,
				)
				updateThinkingText('Ожидаю уточнение... 🤔')
			} else {
				// Успешная генерация
				addGeneratedCodeResponse(response)
				updateThinkingText('Готов к работе ✅')
				isAwaitingClarification = false
				pendingQuestion = null
			}
		} else {
			addBotMessage(
				`❌ **Ошибка генерации**\n\nНе удалось сгенерировать код. Попробуйте переформулировать запрос.`,
			)
			updateThinkingText('Ошибка ❌')
		}
	} catch (error) {
		stopTimer()
		removeTypingIndicator()
		addBotMessage(
			`❌ **Ошибка:** ${error.message}\n\nПроверьте, запущен ли бекенд на ${API_BASE_URL}`,
		)
		updateThinkingText('Ошибка соединения ❌')
		console.error('Generate error:', error)
	}
}

/**
 * Отправка уточнения/фидбека
 */
async function sendClarificationFeedback(feedbackText) {
	addUserMessage(feedbackText)
	userInput.value = ''
	userInput.style.height = 'auto'

	updateThinkingText('Исправляю код... 🔧')
	startTimer()
	showTypingIndicator()

	try {
		const response = await sendFeedback(feedbackText, currentCode)
		stopTimer()
		removeTypingIndicator()
		setTimerValue(response.execution_time_ms || 0)

		if (response.success && response.code) {
			currentCode = response.code
			let message = `**✅ Исправленный код:**\n\n\`\`\`lua\n${response.code}\n\`\`\`\n\n`
			if (response.explanation) {
				message += `**📖 Что изменилось:**\n${response.explanation}`
			}
			addBotMessage(message)
			updateThinkingText('Готов к работе ✅')
			isAwaitingClarification = false
			pendingQuestion = null
		} else {
			addBotMessage(
				`❌ Не удалось исправить код. Попробуйте ещё раз или переформулируйте запрос.`,
			)
			updateThinkingText('Ошибка ❌')
		}
	} catch (error) {
		stopTimer()
		removeTypingIndicator()
		addBotMessage(`❌ Ошибка при исправлении: ${error.message}`)
		updateThinkingText('Ошибка ❌')
		console.error('Feedback error:', error)
	}
}

// ================================================
// 7. ИСТОРИЯ И СТАТИСТИКА
// ================================================

/**
 * Показать историю в модальном окне
 */
async function showHistory() {
	const historyData = await fetchHistory()

	// Создаем модальное окно
	const modal = document.createElement('div')
	modal.className = 'history-modal'
	modal.innerHTML = `
        <div class="history-modal-content">
            <div class="history-modal-header">
                <h3>📜 История генераций</h3>
                <button class="history-modal-close">&times;</button>
            </div>
            <div class="history-modal-body">
                ${historyData.length === 0 ? '<p class="history-empty">История пуста</p>' : ''}
                ${historyData
									.map(
										entry => `
                    <div class="history-entry">
                        <div class="history-entry-prompt">📝 ${escapeHtml(entry.prompt)}</div>
                        <div class="history-entry-code"><code>${escapeHtml(entry.code?.substring(0, 100) || '')}...</code></div>
                        <div class="history-entry-meta">
                            <span class="${entry.success ? 'success' : 'error'}">${entry.success ? '✅ Успешно' : '❌ Ошибка'}</span>
                            <span>⏱ ${entry.execution_time_ms || 0} мс</span>
                            <span>📅 ${new Date(entry.created_at).toLocaleString()}</span>
                        </div>
                    </div>
                `,
									)
									.join('')}
            </div>
        </div>
    `

	document.body.appendChild(modal)

	// Закрытие по клику
	modal.querySelector('.history-modal-close').onclick = () => modal.remove()
	modal.onclick = e => {
		if (e.target === modal) modal.remove()
	}
}

/**
 * Показать статистику
 */
async function showStats() {
	const stats = await fetchStats()
	if (!stats) {
		addBotMessage('❌ Не удалось загрузить статистику')
		return
	}

	const statsMessage = `
        **📊 Статистика генераций**

        • **Всего генераций:** ${stats.total_generations || 0}
        • **Успешность:** ${((stats.success_rate || 0) * 100).toFixed(1)}%
        • **Среднее время:** ${stats.avg_execution_time_ms || 0} мс

        ${stats.top_errors?.length ? '**⚠️ Частые ошибки:**\n' + stats.top_errors.map(e => `  • ${e.error}: ${e.count} раз`).join('\n') : ''}
    `

	addBotMessage(statsMessage)
}

// ================================================
// 8. ОБРАБОТЧИКИ СОБЫТИЙ И ИНИЦИАЛИЗАЦИЯ
// ================================================

/**
 * Автоматическое расширение textarea
 */
userInput.addEventListener('input', function () {
	this.style.height = 'auto'
	this.style.height = Math.min(this.scrollHeight, 120) + 'px'
})

/**
 * Отправка по Enter (Shift+Enter - новая строка)
 */
userInput.addEventListener('keydown', e => {
	if (e.key === 'Enter' && !e.shiftKey) {
		e.preventDefault()
		sendMessage()
	}
})

/**
 * Кнопка отправки
 */
sendBtn.addEventListener('click', sendMessage)

/**
 * Кнопка истории
 */
if (historyBtn) {
	historyBtn.addEventListener('click', showHistory)
}

/**
 * Инициализация сессии
 */
function init() {
	loadSessionId() // ← загружаем session_id
	loadChat() // ← загружаем чат

	console.log('Session ID:', currentSessionId)
	updateThinkingText('Готов к работе ✅')

	// Добавляем стили для модального окна истории (оставляем как было)
	const modalStyles = document.createElement('style')
	modalStyles.textContent = `
        .history-modal {
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.5);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 2000;
        }
        .history-modal-content {
            background: white;
            border-radius: 16px;
            width: 90%;
            max-width: 700px;
            max-height: 80%;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        .history-modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 20px;
            border-bottom: 1px solid #e9ecef;
        }
        .history-modal-close {
            background: none;
            border: none;
            font-size: 24px;
            cursor: pointer;
        }
        .history-modal-body {
            flex: 1;
            overflow-y: auto;
            padding: 16px;
        }
        .history-entry {
            background: #f8f9fa;
            border-radius: 12px;
            padding: 12px;
            margin-bottom: 12px;
        }
        .history-entry-prompt {
            font-weight: 600;
            margin-bottom: 8px;
        }
        .history-entry-code {
            font-size: 12px;
            background: #1e1e1e;
            color: #d4d4d4;
            padding: 8px;
            border-radius: 8px;
            overflow-x: auto;
        }
        .history-entry-meta {
            display: flex;
            gap: 12px;
            margin-top: 8px;
            font-size: 11px;
            color: #6c757d;
        }
        .history-empty {
            text-align: center;
            color: #6c757d;
            padding: 40px;
        }
        .success { color: #28a745; }
        .error { color: #dc3545; }
    `
	document.head.appendChild(modalStyles)
}

// ========== ФУНКЦИИ ДЛЯ МЫСЛЕЙ МОДЕЛИ ==========

// Добавление мысли в блок
function addThought(message, type = 'processing') {
	const thoughtsList = document.getElementById('thoughtsList')
	if (!thoughtsList) return

	const thoughtDiv = document.createElement('div')
	thoughtDiv.className = `thought-message ${type}`

	const now = new Date()
	const timeStr = now.toLocaleTimeString('ru-RU', {
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
	})

	thoughtDiv.innerHTML = `
        <div>${message}</div>
        <div class="thought-time">${timeStr}</div>
    `

	thoughtsList.appendChild(thoughtDiv)

	// Автоскролл вниз
	thoughtDiv.scrollIntoView({ behavior: 'smooth', block: 'end' })

	// Ограничиваем количество мыслей (оставляем последние 30)
	while (thoughtsList.children.length > 30) {
		thoughtsList.removeChild(thoughtsList.firstChild)
	}
}

// Очистка мыслей (опционально)
function clearThoughts() {
	const thoughtsList = document.getElementById('thoughtsList')
	if (thoughtsList) {
		thoughtsList.innerHTML = ''
		addThought('Очистка истории мыслей', 'system')
	}
}

// ========== ОБНОВЛЕННАЯ ФУНКЦИЯ GENERATE ==========
async function generateCode(prompt) {
	// Добавляем мысль о начале генерации
	addThought(
		`Получен запрос: "${prompt.substring(0, 50)}${prompt.length > 50 ? '...' : ''}"`,
		'processing',
	)
	addThought('Анализирую задачу...', 'processing')

	const response = await fetch(`${API_BASE_URL}/generate`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify({
			prompt: prompt,
			session_id: currentSessionId,
		}),
	})

	addThought('⚙️ Отправка запроса на сервер...', 'processing')

	if (!response.ok) {
		addThought(`Ошибка сервера: ${response.status}`, 'error')
		throw new Error(`HTTP ${response.status}`)
	}

	addThought('🔄 Получен ответ, обрабатываю...', 'processing')

	const data = await response.json()

	if (data.success) {
		addThought('Код успешно сгенерирован!', 'success')
		if (data.execution_time_ms) {
			addThought(`⏱ Время выполнения: ${data.execution_time_ms} мс`, 'system')
		}
	} else {
		addThought('Генерация не удалась', 'error')
	}

	return data
}

// ========== ОБНОВЛЕННАЯ ФУНКЦИЯ FEEDBACK ==========
async function sendFeedback(feedback, previousCode) {
	addThought(
		`Получен фидбек: "${feedback.substring(0, 50)}${feedback.length > 50 ? '...' : ''}"`,
		'processing',
	)
	addThought('Исправляю код на основе обратной связи...', 'processing')

	const response = await fetch(`${API_BASE_URL}/feedback`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
		},
		body: JSON.stringify({
			session_id: currentSessionId,
			feedback: feedback,
			previous_code: previousCode || '',
		}),
	})

	if (!response.ok) {
		addThought(`Ошибка при исправлении: ${response.status}`, 'error')
		throw new Error(`HTTP ${response.status}`)
	}

	const data = await response.json()

	if (data.success) {
		addThought('Код успешно исправлен!', 'success')
	} else {
		addThought('Не удалось исправить код', 'error')
	}

	return data
}

// ========== ДОБАВЛЯЕМ МЫСЛИ В SENDMESSAGE ==========
// Обнови существующую функцию sendMessage, добавив мысли:

async function sendMessage() {
	const text = userInput.value.trim()
	if (!text) return

	// Добавляем мысль о начале обработки
	addThought(
		` Пользователь: "${text.substring(0, 40)}${text.length > 40 ? '...' : ''}"`,
		'system',
	)

	// Если ожидаем уточнение
	if (isAwaitingClarification) {
		addThought('Отправляю уточнение...', 'processing')
		await sendClarificationFeedback(text)
		return
	}

	// Обычная генерация
	addUserMessage(text)
	userInput.value = ''
	userInput.style.height = 'auto'

	updateThinkingText('Генерирую код... ')
	startTimer()
	showTypingIndicator()

	addThought('Запуск процесса генерации...', 'processing')

	try {
		const response = await generateCode(text)
		stopTimer()
		removeTypingIndicator()
		setTimerValue(response.execution_time_ms || 0)

		if (response.success) {
			if (response.needs_clarification && response.question) {
				isAwaitingClarification = true
				pendingQuestion = response.question
				addBotMessage(
					` **Уточняющий вопрос:**\n\n${response.question}\n\nПожалуйста, уточните ваш запрос.`,
				)
				addThought(` Задан уточняющий вопрос: "${response.question}"`, 'ai')
				updateThinkingText('Ожидаю уточнение... ')
			} else {
				addGeneratedCodeResponse(response)
				addThought('💡 Код отправлен пользователю', 'success')
				updateThinkingText('Готов к работе ')
				isAwaitingClarification = false
				pendingQuestion = null
			}
		} else {
			addThought(' Ошибка в ответе сервера', 'error')
			addBotMessage(
				` **Ошибка генерации**\n\nНе удалось сгенерировать код. Попробуйте переформулировать запрос.`,
			)
			updateThinkingText('Ошибка ')
		}
	} catch (error) {
		stopTimer()
		removeTypingIndicator()
		addThought(` Критическая ошибка: ${error.message}`, 'error')
		addBotMessage(
			` **Ошибка:** ${error.message}\n\nПроверьте, запущен ли бекенд на ${API_BASE_URL}`,
		)
		updateThinkingText('Ошибка соединения ')
		console.error('Generate error:', error)
	}
}

function saveChat() {
	const messages = document.getElementById('chatMessages').innerHTML
	localStorage.setItem('chat', messages)
	localStorage.setItem('currentCode', currentCode || '')
	localStorage.setItem('isAwaitingClarification', isAwaitingClarification)
	localStorage.setItem('pendingQuestion', pendingQuestion || '')
}

// Восстанавливаем чат при загрузке
function loadChat() {
	const saved = localStorage.getItem('chat')
	if (saved) {
		document.getElementById('chatMessages').innerHTML = saved
		currentCode = localStorage.getItem('currentCode') || null
		isAwaitingClarification =
			localStorage.getItem('isAwaitingClarification') === 'true'
		pendingQuestion = localStorage.getItem('pendingQuestion') || null
		scrollChatToBottom()
	}
}

// Запуск инициализации
init()
