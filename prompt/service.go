package prompt

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var ErrNoPromptExtension = fmt.Errorf("file does not have a .prompt extension")

type AddOn string

var (
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
	// Regular expression to match JSON-like objects, handling nested structures
	reJSON := regexp.MustCompile(`(\{[^{}]*\}|\$begin:math:display\$[^$]*\$end:math:display\$)`)

	// Function to minify JSON-like objects
	minifyJSON := func(json string) string {
		var result strings.Builder
		inString := false
		inComment := false

		for i := 0; i < len(json); i++ {
			char := json[i]

			if char == '"' {
				inString = !inString
			}

			if !inString && char == '/' && i+1 < len(json) && json[i+1] == '/' {
				inComment = true
			}

			if inComment && char == '\n' {
				inComment = false
				result.WriteByte(char)
				continue
			}

			if !inString && !inComment && (char == ' ' || char == '\t' || char == '\n') {
				continue
			}

			result.WriteByte(char)
		}
		return result.String()
	}

	// Split prompt into parts: text and JSON-like objects
	parts := reJSON.Split(prompt, -1)
	matches := reJSON.FindAllString(prompt, -1)

	// Reconstruct the prompt, minifying JSON-like objects
	var result strings.Builder
	for i, part := range parts {
		result.WriteString(strings.TrimSpace(part))
		if i < len(matches) {
			if result.Len() > 0 && !strings.HasSuffix(result.String(), "[") && !strings.HasSuffix(result.String(), "{") {
				result.WriteByte(' ')
			}
			result.WriteString(minifyJSON(matches[i]))
		}
	}

	// Add-ons
	for _, addon := range addons {
		result.WriteString("\n")
		result.WriteString(string(addon))
	}

	return MinifiedPrompt(result.String())
}
