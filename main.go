package main

import (
	"encoding/json"
	"fmt"

	"github.com/markettools-ai/poggers/prompt"
)

func main() {
	promptBuilder := prompt.NewPromptBuilder(
		map[string]string{
			"PlayerStats": "The player is low HP. They have a sword, but their favorite weapon is a bow. They have 10 coins.",
		},
	)

	messages, err := promptBuilder.ProcessFromFile("./examples/1_rpg_shop.prompt")
	if err != nil {
		fmt.Println(err)
		return
	}
	stringifiedFromFile, _ := json.MarshalIndent(messages, "", "  ")
	fmt.Println(string(stringifiedFromFile))
}
