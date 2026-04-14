package dto

type OllamaRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages,omitempty"`
	Prompt   string                 `json:"prompt,omitempty"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options"`
}

type OllamaResponse struct {
	Message  Message `json:"message,omitempty"`
	Response string  `json:"response,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
