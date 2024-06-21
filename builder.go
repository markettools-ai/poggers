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
	Process(url string) error

	ProcessBatchFromDir(directory string) error
	ProcessBatch(batch [][]Prompt) error

	ProcessFromFile(filename string) ([]Message, error)
	ProcessRaw(name, prompt string) ([]Message, error)

	SetAnnotation(id string, value interface{})
}
