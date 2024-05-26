package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var ErrNoPromptExtension = fmt.Errorf("file does not have a .prompt extension")

type AddOn string

const (
	AddOnJSONOutput AddOn = "The output should be only a valid, raw, minified JSON object with no additional data."
)

// Read the file with .prompt extension and minifies it
func FromFile(filename string, addons ...AddOn) (string, error) {
	// Check if the file has a .prompt extension
	if !strings.HasSuffix(filename, ".prompt") {
		return "", ErrNoPromptExtension
	}

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

	return string(FromString(fullText, addons...)), nil
}

type MinifiedPrompt string

// Remove unnecessary spaces inside JSON-like objects while keeping comments and spaces outside JSON-like objects.
func FromString(prompt string, addons ...AddOn) MinifiedPrompt {
	prompt = strings.TrimSpace(prompt)
	var result strings.Builder
	bracketCount := 0
	for i := 0; i < len(prompt); {
		next := func(write bool) {
			if write {
				result.WriteByte(prompt[i])
			}
			i++
		}

		// Brackets
		switch prompt[i] {
		case '{', '[':
			bracketCount++
		case '}', ']':
			bracketCount--
		}

		if bracketCount > 0 {
			// Strings
			if prompt[i] == '"' {
				next(true)
				for prompt[i] != '"' {
					next(true)
				}
				next(true)
			}

			// Comments
			if prompt[i] == '/' && i+1 < len(prompt) && prompt[i+1] == '/' {
				next(true)
				next(true)
				for prompt[i] != '\n' {
					next(true)
				}
				next(true)
			}

			// Spaces
			if prompt[i] == ' ' || prompt[i] == '\n' || prompt[i] == '\t' {
				next(false)
				continue
			}
		}
		// Other characters
		next(true)
	}

	// Add-ons
	for _, addon := range addons {
		result.WriteString("\n")
		result.WriteString(string(addon))
	}

	return MinifiedPrompt(result.String())
}
