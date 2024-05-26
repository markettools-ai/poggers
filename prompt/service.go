package prompt

type Message struct {
	Author  string
	Content string
}

type PromptBuilder interface {
	ProcessFromFile(filename string) ([]Message, error)
	Process(prompt string) ([]Message, error)

	SetAnnotation(id string, value interface{})
}
