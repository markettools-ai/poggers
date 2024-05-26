package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type promptBuilder struct {
	annotations map[string]string
}

func NewPromptBuilder(annotations ...map[string]string) PromptBuilder {
	internalAnnotations := make(map[string]string)
	if len(annotations) > 0 {
		for k, v := range annotations[0] {
			internalAnnotations[k] = v
		}
	}

	internalAnnotations["OutputSchema"] = OutputSchema
	internalAnnotations["JSONOutput"] = JSONOutput

	return &promptBuilder{internalAnnotations}
}

func (pB *promptBuilder) ProcessFromFile(filename string) ([]Message, error) {
	// Check if the file has a .prompt extension
	if !strings.HasSuffix(filename, ".prompt") {
		return []Message{}, fmt.Errorf("file must have a .prompt extension")
	}

	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return []Message{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read and print the file contents
	fullText := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fullText += scanner.Text() + "\n"
	}

	if err := scanner.Err(); err != nil {
		return []Message{}, fmt.Errorf("error reading file: %w", err)
	}

	return pB.Process(fullText)
}

func (pB *promptBuilder) Process(prompt string) ([]Message, error) {
	prompt = "\n" + prompt + "\n"
	var result strings.Builder
	messages := []Message{}
	var stack int
	var label string
	var isTabulated bool
	for i := 0; i < len(prompt); {
		next := func(write bool) {
			if write {
				result.WriteByte(prompt[i])
			}
			i++
		}

		// New line
		if prompt[i] == '\n' {
			isTabulated = false
			next(false)
			if i == len(prompt) {
				continue
			}
			// Tabulation
			if prompt[i] == '\t' || i+4 < len(prompt) && prompt[i:i+4] == "    " {
				isTabulated = true
				// Check for label
				if label == "" {
					return []Message{}, fmt.Errorf("found tabulated text without a label")
				}
				// Skip tabulation
				for prompt[i] == ' ' || prompt[i] == '\t' {
					next(false)
				}
			}
			continue
		}
		// Labels
		if !isTabulated {
			// Reset label
			if label != "" {
				messages = append(messages, Message{Role: label, Content: result.String()})
				result.Reset()
				label = ""
			}
			start := i
			// Get label
			foundLabel := false
			for (prompt[i] >= 'a' && prompt[i] <= 'z') ||
				(prompt[i] >= 'A' && prompt[i] <= 'Z') ||
				(prompt[i] >= '0' && prompt[i] <= '9') ||
				prompt[i] == '_' || prompt[i] == '-' {
				foundLabel = true
				next(false)
			}
			if prompt[i] != ':' {
				return []Message{}, fmt.Errorf("expected a label, found %q", prompt[start:i])
			}
			if !foundLabel {
				return []Message{}, fmt.Errorf("expected a label, found nothing")
			}
			label = prompt[start:i]
			// Skip the colon
			next(false)
			// Skip spaces
			for prompt[i] == ' ' {
				next(false)
			}
			continue
		}

		// Escape characters
		if prompt[i] == '\\' {
			next(false)
			next(true)
			continue
		}
		// Annotations
		if prompt[i] == '@' {
			next(false)
			start := i
			for prompt[i] != ' ' && prompt[i] != '\n' {
				next(false)
			}
			id := prompt[start:i]
			if value, ok := pB.annotations[id]; ok {
				// Add space before annotation if needed
				if result.Len() > 0 && result.String()[result.Len()-1] != ' ' {
					result.WriteByte(' ')
				}
				result.WriteString(value)
			}
			continue
		}
		// Brackets
		switch prompt[i] {
		case '{', '[':
			stack++
			next(true)
			continue
		case '}', ']':
			stack--
			next(true)
			continue
		}

		// Objects
		if stack > 0 {
			// Strings
			if prompt[i] == '"' {
				next(true)
				for prompt[i] != '"' {
					// Escape characters
					if prompt[i] == '\\' {
						next(false)
						next(true)
						continue
					}
					// Annotations
					if prompt[i] == '@' {
						next(false)
						start := i
						for prompt[i] != ' ' && prompt[i] != '\n' {
							next(false)
						}
						id := prompt[start:i]
						if value, ok := pB.annotations[id]; ok {
							// Add space before annotation if needed
							if result.Len() > 0 && result.String()[result.Len()-1] != ' ' {
								result.WriteByte(' ')
							}
							result.WriteString(value)
						}
						continue
					}
					next(true)
				}
				next(true)
				continue
			}
			// Comments
			if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
				next(true)
				next(true)
				for prompt[i] != '\n' {
					// Escape characters
					if prompt[i] == '\\' {
						next(false)
						next(true)
						continue
					}
					// Annotations
					if prompt[i] == '@' {
						next(false)
						start := i
						for prompt[i] != ' ' && prompt[i] != '\n' {
							next(false)
						}
						id := prompt[start:i]
						if value, ok := pB.annotations[id]; ok {
							// Add space before annotation if needed
							if result.Len() > 0 && result.String()[result.Len()-1] != ' ' {
								result.WriteByte(' ')
							}
							result.WriteString(value)
						}
						continue
					}
					next(true)
				}
				next(true)
				continue
			}
			// Spaces
			foundSpace := false
			for prompt[i] == ' ' || prompt[i] == '\n' || prompt[i] == '\t' {
				next(false)
				foundSpace = true
			}
			if foundSpace {
				continue
			}
		}

		// Other characters
		next(true)
	}
	// Last label
	if label != "" {
		messages = append(messages, Message{Role: label, Content: result.String()})
	}

	return messages, nil
}

func (pB *promptBuilder) SetAnnotation(id string, value interface{}) {
	if value == nil {
		delete(pB.annotations, id)
		return
	}
	pB.annotations[id] = fmt.Sprintf("%v", value)
}
