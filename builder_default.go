package poggers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

type promptBuilder struct {
	annotations       map[string]string
	annotationsMutexn sync.RWMutex
	onBeforeProcess   func(name, prompt string) (bool, error)
	onAfterProcess    func(name string, messages []Message) error
}

type PromptBuilderOptions struct {
	Annotations     map[string]string
	OnBeforeProcess func(name, prompt string) (skip bool, err error)
	OnAfterProcess  func(name string, messages []Message) error
}

func NewPromptBuilder(options ...PromptBuilderOptions) PromptBuilder {
	// Default annotations
	internalAnnotations := map[string]string{
		"OutputSchema": OutputSchema,
		"JSONOutput":   JSONOutput,
		"LockedInput":  LockedInput,
	}
	var onBeforeProcess func(name, prompt string) (bool, error)
	var onAfterProcess func(name string, messages []Message) error
	// Override options
	if len(options) > 0 {
		if options[0].Annotations != nil {
			for k, v := range options[0].Annotations {
				internalAnnotations[k] = v
			}
		}
		onBeforeProcess = options[0].OnBeforeProcess
		onAfterProcess = options[0].OnAfterProcess
	}

	return &promptBuilder{internalAnnotations, sync.RWMutex{}, onBeforeProcess, onAfterProcess}
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

func (pB *promptBuilder) ProcessBatchFromDir(directory string) error {
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
	return pB.ProcessBatch(batches)
}

func (pB *promptBuilder) ProcessBatch(batch [][]Prompt) error {
	for _, prompts := range batch {
		wg := sync.WaitGroup{}
		errChan := make(chan error, len(prompts))
		wg.Add(len(prompts))
		for i := 0; i < len(prompts); i++ {
			go func(i int) {
				defer wg.Done()
				// Remove the .prompt suffix
				prompts[i].Name = strings.TrimSuffix(prompts[i].Name, ".prompt")
				// Process the prompt
				_, err := pB.Process(prompts[i].Name, prompts[i].Text)
				if err != nil {
					errChan <- fmt.Errorf("error processing prompt: %w", err)
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
	// Remove the .prompt suffix
	pathParts := strings.Split(filename, "/")
	name := strings.TrimSuffix(pathParts[len(pathParts)-1], ".prompt")
	// Process the file contents
	messages, err := pB.Process(name, text)
	if err != nil {
		return []Message{}, fmt.Errorf("error processing file: %w", err)
	}

	return messages, nil
}

func (pB *promptBuilder) getAnnotation(id string) string {
	pB.annotationsMutexn.RLock()
	annotation := pB.annotations[id]
	pB.annotationsMutexn.RUnlock()
	return annotation
}

func (pB *promptBuilder) processPrompt(prompt string) ([]Message, error) {
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
			result.WriteString(pB.getAnnotation(prompt[start:i]))
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
						result.WriteString(pB.getAnnotation(prompt[start:i]))
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
						result.WriteString(pB.getAnnotation(prompt[start:i]))
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

func (pB *promptBuilder) Process(name, prompt string) ([]Message, error) {
	// Process the onBeforeProcess callback
	if pB.onBeforeProcess != nil {
		skip, err := pB.onBeforeProcess(name, prompt)
		if err != nil {
			return []Message{}, fmt.Errorf("error before processing: %w", err)
		}
		if skip {
			return nil, nil
		}
	}

	// Process the prompt
	results, err := pB.processPrompt(prompt)
	if err != nil {
		return []Message{}, fmt.Errorf("error processing prompt: %w", err)
	}

	// Process the onAfterProcess callback
	if pB.onAfterProcess != nil {
		err := pB.onAfterProcess(name, results)
		if err != nil {
			return []Message{}, fmt.Errorf("error after processing: %w", err)
		}
	}

	return results, nil
}

func (pB *promptBuilder) SetAnnotation(id string, value interface{}) {
	if value == nil {
		pB.annotationsMutexn.Lock()
		delete(pB.annotations, id)
		pB.annotationsMutexn.Unlock()
		return
	}
	valueJSON, err := json.Marshal(value)
	if err != nil {
		pB.annotations[id] = fmt.Sprintf("%v", value)
	} else {
		pB.annotations[id] = string(valueJSON)
	}
}
