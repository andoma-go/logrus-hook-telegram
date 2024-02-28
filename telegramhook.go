package telegramhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/andoma-go/logrus"
)

// TelegramHook to send logs via the Telegram API.
type TelegramHook struct {
	client    *http.Client
	mu        sync.RWMutex
	appName   string
	authToken string
	chatId    string
	threadId  string
	level     logrus.Level
	async     bool
}

// Option defines a method for additional configuration when instantiating TelegramHook
type Option func(*TelegramHook)

// Async sets logging to telegram as asynchronous
func WithAsync(async bool) Option {
	return func(h *TelegramHook) {
		h.SetAsync(async)
	}
}

// Timeout sets http call timeout for telegram client
func WithTimeout(timeout time.Duration) Option {
	return func(h *TelegramHook) {
		if timeout > 0 {
			h.client.Timeout = timeout
		}
	}
}

// WithLevel set level
func WithLevel(level logrus.Level) Option {
	return func(h *TelegramHook) {
		h.SetLevel(level)
	}
}

// New creates a new instance of a hook targeting the Telegram API.
func NewTelegramHook(appName, authToken, chatId, threadId string, options ...Option) (*TelegramHook, error) {
	client := &http.Client{}
	return NewTelegramHookWithClient(appName, authToken, chatId, threadId, client, options...)
}

// NewTelegramHookWithClient creates a new instance of a hook targeting the Telegram API with custom http.Client.
func NewTelegramHookWithClient(appName, authToken, chatId, threadId string, client *http.Client, options ...Option) (*TelegramHook, error) {
	h := TelegramHook{
		client:    client,
		appName:   appName,
		authToken: authToken,
		chatId:    chatId,
		threadId:  threadId,
		level:     logrus.ErrorLevel,
		async:     false,
	}

	for _, opt := range options {
		opt(&h)
	}

	// Verify the API token is valid and correct before continuing
	if err := h.verifyToken(); err != nil {
		return nil, err
	}

	return &h, nil
}

// apiRequest encapsulates the request structure we are sending to the Telegram API.
type apiRequest struct {
	ChatId    string `json:"chat_id"`
	ThreadId  string `json:"message_thread_id,omitempty"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// apiResponse encapsulates the response structure received from the Telegram API.
type apiResponse struct {
	Ok        bool         `json:"ok"`
	ErrorCode *int         `json:"error_code,omitempty"`
	Desc      *string      `json:"description,omitempty"`
	Result    *interface{} `json:"result,omitempty"`
}

// verifyToken issues a test request to the Telegram API to ensure the provided token is correct and valid.
func (h *TelegramHook) verifyToken() error {
	endpoint, _ := url.JoinPath(h.ApiEndpoint(), "getMe")

	res, err := h.client.Get(endpoint)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	apiRes := apiResponse{}
	if err := json.NewDecoder(res.Body).Decode(&apiRes); err != nil {
		return err
	}

	if !apiRes.Ok {
		// Received an error from the Telegram API
		msg := "Received error response from Telegram API"

		if apiRes.ErrorCode != nil {
			msg = fmt.Sprintf("%s (error code %d)", msg, *apiRes.ErrorCode)
		}

		if apiRes.Desc != nil {
			msg = fmt.Sprintf("%s: %s", msg, *apiRes.Desc)
		}

		j, _ := json.MarshalIndent(apiRes, "", "\t")
		return fmt.Errorf("%s\n%s", msg, j)
	}

	return nil
}

// sendMessage issues the provided message to the Telegram API.
func (h *TelegramHook) sendMessage(msg string) error {
	apiReq := apiRequest{
		ChatId:    h.ChatId(),
		ThreadId:  h.ThreadId(),
		Text:      msg,
		ParseMode: "HTML",
	}
	b, err := json.Marshal(apiReq)
	if err != nil {
		return err
	}

	endpoint, _ := url.JoinPath(h.ApiEndpoint(), "sendMessage")

	res, err := h.client.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error when issuing request to Telegram API, %v", err)
		return err
	}
	defer res.Body.Close()

	apiRes := apiResponse{}
	if err := json.NewDecoder(res.Body).Decode(&apiRes); err != nil {
		return err
	}

	if !apiRes.Ok {
		// Received an error from the Telegram API
		msg := "Received error response from Telegram API"

		if apiRes.ErrorCode != nil {
			msg = fmt.Sprintf("%s (error code %d)", msg, *apiRes.ErrorCode)
		}

		if apiRes.Desc != nil {
			msg = fmt.Sprintf("%s: %s", msg, *apiRes.Desc)
		}

		return fmt.Errorf(msg)
	}

	return nil
}

// createMessage crafts an HTML-formatted message to send to the Telegram API.
func (h *TelegramHook) createMessage(entry *logrus.Entry) string {
	var msg string

	switch entry.Level {
	case logrus.PanicLevel:
		msg = "<b>PANIC</b>"
	case logrus.FatalLevel:
		msg = "<b>FATAL</b>"
	case logrus.ErrorLevel:
		msg = "<b>ERROR</b>"
	case logrus.WarnLevel:
		msg = "<b>WARNING</b>"
	case logrus.InfoLevel:
		msg = "<b>INFO</b>"
	case logrus.DebugLevel:
		msg = "<b>DEBUG</b>"
	}

	msg = strings.Join([]string{msg, h.AppName()}, "@")
	msg = strings.Join([]string{msg, entry.Message}, " - ")

	if len(entry.Data) > 0 {
		msg = strings.Join([]string{msg, "<pre>"}, "\n")
		for k, v := range entry.Data {
			msg = strings.Join([]string{msg, html.EscapeString(fmt.Sprintf("\t%s: %+v", k, v))}, "\n")
		}
		msg = strings.Join([]string{msg, "</pre>"}, "\n")
	}

	return msg
}

// Levels returns the log levels that the hook should be enabled for.
func (h *TelegramHook) Levels() []logrus.Level {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return logrus.AllLevels[:h.level+1]
}

// Fire emits a log message to the Telegram API.
func (h *TelegramHook) Fire(entry *logrus.Entry) error {
	msg := h.createMessage(entry)

	if h.Async() {
		go h.sendMessage(msg)
		return nil
	}

	if err := h.sendMessage(msg); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send message, %v", err)
		return err
	}

	return nil
}

// ApiEndpoint
func (h *TelegramHook) ApiEndpoint() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return fmt.Sprintf("https://api.telegram.org/bot%s", h.authToken)
}

// AppName
func (h *TelegramHook) AppName() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.appName
}

func (h *TelegramHook) SetAppName(appName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.appName = appName
}

// AuthToken
func (h *TelegramHook) AuthToken() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.authToken
}

func (h *TelegramHook) SetAuthToken(authToken string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.authToken = authToken
}

// ChatId
func (h *TelegramHook) ChatId() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.chatId
}

func (h *TelegramHook) SetChatId(chatId string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.chatId = chatId
}

// ThreadId
func (h *TelegramHook) ThreadId() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.threadId
}

func (h *TelegramHook) SetThreadId(threadId string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.threadId = threadId
}

// Level
func (h *TelegramHook) Level() logrus.Level {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.level
}

func (h *TelegramHook) SetLevel(level logrus.Level) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.level = level
}

// Async
func (h *TelegramHook) Async() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.async
}

func (h *TelegramHook) SetAsync(async bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.async = async
}
