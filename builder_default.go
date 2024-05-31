package poggers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type promptBuilder struct {
	annotations       map[string]string
	annotationsMutexn sync.RWMutex
	onBeforeProcess   func(name string, constants map[string]string) (bool, error)
	onAfterProcess    func(name string, messages []Message) error
}

type PromptBuilderOptions struct {
	Annotations     map[string]string
	OnBeforeProcess func(name string, constants map[string]string) (skip bool, err error)
	OnAfterProcess  func(name string, messages []Message) error
}

func NewPromptBuilder(options ...PromptBuilderOptions) PromptBuilder {
	// Default annotations
	internalAnnotations := map[string]string{
		"OutputSchema": OutputSchema,
		"JSONOutput":   JSONOutput,
		"LockedInput":  LockedInput,
	}
	var onBeforeProcess func(name string, constants map[string]string) (bool, error)
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
	constants := map[string]string{}
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
			// Comment
			if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
				// Skip the comment
				for prompt[i] != '\n' {
					next(false)
				}
				continue
			}
			// Check for tabulation
			if prompt[i] == '\t' || prompt[i] == ' ' {
				isTabulated = true
				for prompt[i] == '\t' || prompt[i] == ' ' {
					next(false)
				}
				// Check for new line
				if prompt[i] == '\n' {
					continue
				}
				// Comment
				if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
					// Skip the comment
					for prompt[i] != '\n' {
						next(false)
					}
					continue
				}
				// Check for label
				if label == "" {
					return []Message{}, fmt.Errorf("found tabulated text without a label")
				}
			}
			continue
		}
		// Labels or Constants
		if !isTabulated {
			// Reset label
			if label != "" {
				messages = append(messages, Message{Role: label, Content: result.String()})
				result.Reset()
				label = ""
			}
			start := i
			// Check for word
			if prompt[i] >= 'a' && prompt[i] <= 'z' ||
				prompt[i] >= 'A' && prompt[i] <= 'Z' ||
				prompt[i] >= '0' && prompt[i] <= '9' ||
				prompt[i] == '_' || prompt[i] == '-' {
				// Start getting the word
				for (prompt[i] >= 'a' && prompt[i] <= 'z') ||
					(prompt[i] >= 'A' && prompt[i] <= 'Z') ||
					(prompt[i] >= '0' && prompt[i] <= '9') ||
					prompt[i] == '_' || prompt[i] == '-' {
					next(false)
				}
				// Check for colon
				if prompt[i] == ':' {
					label = prompt[start:i]
					// Skip the colon
					next(false)
					// Skip spaces
					for prompt[i] == '\t' || prompt[i] == ' ' {
						next(false)
					}
					// Comment
					if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
						// Skip the comment
						for prompt[i] != '\n' {
							next(false)
						}
						continue
					}
					// End the label
					if prompt[i] == '\n' {
						continue
					} else {
						return []Message{}, fmt.Errorf("expected new line, found %q", prompt[i])
					}
				}
				// Skip spaces
				for prompt[i] == '\t' || prompt[i] == ' ' {
					next(false)
				}
				// Check for equals sign
				if prompt[i] == '=' {
					// Skip the equals sign
					next(false)
					// Skip spaces
					for prompt[i] == '\t' || prompt[i] == ' ' {
						next(false)
					}
					// Start getting the value
					valueStart := i
					// Skip until new line or comment
					for prompt[i] != '\n' || (prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/') {
						next(false)
					}
					constants[prompt[start:i]] = prompt[valueStart:i]
					// Comment
					if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
						// Skip the comment
						for prompt[i] != '\n' {
							next(false)
						}
						continue
					}
					continue
				} else {
					return []Message{}, fmt.Errorf("expected colon or equals sign, found %q", prompt[i])
				}
			} else {
				return []Message{}, fmt.Errorf("expected label or constant, found nothing")
			}
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
					next(true)
				}
				next(true)
				continue
			}
			// AI Comments
			if prompt[i] == '#' {
				next(true)
				for prompt[i] != '\n' {
					next(true)
				}
				next(true)
				continue
			}
			// Comment
			if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
				// Skip the comment
				for prompt[i] != '\n' {
					next(false)
				}
				continue
			}
			// Spaces
			if prompt[i] == ' ' || prompt[i] == '\n' || prompt[i] == '\t' {
				next(false)
				for prompt[i] == ' ' || prompt[i] == '\n' || prompt[i] == '\t' {
					next(false)
				}
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
	// Process the prompt
	results, err := pB.processPrompt(prompt)
	if err != nil {
		return []Message{}, fmt.Errorf("error processing prompt: %w", err)
	}

	// Get constants
	constants := map[string]string{}
	constantsMatches := regexp.MustCompile(`(?m)^[A-Za-z_\-][A-Za-z_\-0-9]*\s*=\s*.*$`).FindAllString(prompt, -1)
	for _, match := range constantsMatches {
		parts := strings.Split(match, "=")
		constants[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	// Call onBeforeProcess callback
	if pB.onBeforeProcess != nil {
		skip, err := pB.onBeforeProcess(name, constants)
		if err != nil {
			return []Message{}, fmt.Errorf("error before processing: %w", err)
		}
		if skip {
			return nil, nil
		}
	}

	// Replace annotations
	annotations := regexp.MustCompile(`@[A-Za-z0-9_-]+`)
	for i, message := range results {
		results[i].Content = annotations.ReplaceAllStringFunc(message.Content, func(annotation string) string {
			return pB.getAnnotation(annotation[1:])
		})
	}

	// Call onAfterProcess callback
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
