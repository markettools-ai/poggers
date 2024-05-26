package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	promptBuilder := NewPromptBuilder()

	err := promptBuilder.
		ProcessBatchFromDir(
			"./examples/batch",
			func(name string, messages []Message) error {
				switch name {
				case "0_party_theme":
					promptBuilder.SetAnnotation("PartyTheme", "Under the Sea")
				case "0_guests":
					value, _ := json.Marshal([]map[string]interface{}{
						{
							"firstName": "John",
							"hobbies":   []string{"swimming", "reading", "jogging"},
							"age":       20,
						},
						{
							"firstName": "Jane",
							"hobbies":   []string{"painting", "acting", "singing"},
							"age":       25,
						},
					})
					promptBuilder.SetAnnotation("GuestList", string(value))
				}
				stringified, _ := json.MarshalIndent(messages, "", "  ")
				fmt.Println(string(stringified))
				return nil
			},
		)

	if err != nil {
		fmt.Println(err)
		return
	}
}
