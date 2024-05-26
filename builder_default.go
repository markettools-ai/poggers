package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
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

	// Default annotations
	internalAnnotations["OutputSchema"] = OutputSchema
	internalAnnotations["JSONOutput"] = JSONOutput

	return &promptBuilder{internalAnnotations}
}

func readFile(filename string) (string, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read and print the file contents
	fullText := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fullText += scanner.Text() + "\n"
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return fullText, nil
}

func (pB *promptBuilder) ProcessBatchFromDir(directory string, callback func(name string, messages []Message) error) error {
	// Files are grouped by prefix
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("error reading directory: %w", err)
	}
	batches := [][]Prompt{}
	for _, file := range files {
		// Check if the file is a valid batch member
		if file.IsDir() {
			continue
		}
		prefix, err := strconv.Atoi(strings.Split(file.Name(), "_")[0])
		if err != nil {
			continue
		}
		text, err := readFile(directory + "/" + file.Name())
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}
		// Expand the batch slice if needed
		if len(batches) <= prefix {
			batches = append(batches, []Prompt{})
		}
		// Add the file to the batch
		batches[prefix] = append(batches[prefix], Prompt{Name: file.Name(), Text: text})
	}

	// Process batches
	return pB.ProcessBatch(batches, callback)
}

func (pB *promptBuilder) ProcessBatch(batch [][]Prompt, callback func(name string, messages []Message) error) error {
	for _, prompts := range batch {
		wg := sync.WaitGroup{}
		errChan := make(chan error, len(prompts))
		wg.Add(len(prompts))
		for i := 0; i < len(prompts); i++ {
			go func(i int) {
				defer wg.Done()
				// Process the prompt
				messages, err := pB.Process(prompts[i].Text)
				if err != nil {
					errChan <- fmt.Errorf("error processing prompt: %w", err)
					return
				}
				// Remove the .prompt suffix
				prompts[i].Name = strings.TrimSuffix(prompts[i].Name, ".prompt")
				// Process the messages
				err = callback(prompts[i].Name, messages)
				if err != nil {
					errChan <- fmt.Errorf("error processing callback: %w", err)
					return
				}
			}(i)
		}
		wg.Wait()
		close(errChan)
		for err := range errChan {
			return fmt.Errorf("error processing batch: %w", err)
		}
	}
	return nil
}

func (pB *promptBuilder) ProcessFromFile(filename string) ([]Message, error) {
	// Read file
	text, err := readFile(filename)
	if err != nil {
		return []Message{}, fmt.Errorf("error reading file: %w", err)
	}
	// Process the file contents
	messages, err := pB.Process(text)
	if err != nil {
		return []Message{}, fmt.Errorf("error processing file: %w", err)
	}

	return messages, nil
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
			for prompt[i] >= 'a' && prompt[i] <= 'z' ||
				prompt[i] >= 'A' && prompt[i] <= 'Z' ||
				prompt[i] >= '0' && prompt[i] <= '9' ||
				prompt[i] == '_' || prompt[i] == '-' {
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
						for prompt[i] >= 'a' && prompt[i] <= 'z' ||
							prompt[i] >= 'A' && prompt[i] <= 'Z' ||
							prompt[i] >= '0' && prompt[i] <= '9' ||
							prompt[i] == '_' || prompt[i] == '-' {
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
						for prompt[i] >= 'a' && prompt[i] <= 'z' ||
							prompt[i] >= 'A' && prompt[i] <= 'Z' ||
							prompt[i] >= '0' && prompt[i] <= '9' ||
							prompt[i] == '_' || prompt[i] == '-' {
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
