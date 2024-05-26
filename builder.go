package poggers

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Prompt struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type PromptBuilder interface {
	ProcessBatchFromDir(directory string, callback func(name string, messages []Message, isLast bool) error) error
	ProcessBatch(batch [][]Prompt, callback func(name string, messages []Message, isLast bool) error) error

	ProcessFromFile(filename string) ([]Message, error)
	Process(prompt string) ([]Message, error)

	SetAnnotation(id string, value interface{})
}
