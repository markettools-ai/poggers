package prompt

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type PromptBuilder interface {
	ProcessFromFile(filename string) ([]Message, error)
	Process(prompt string) ([]Message, error)

	SetAnnotation(id string, value interface{})
}
