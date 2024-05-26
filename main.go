package main

import (
	"fmt"

	"github.com/markettools-ai/poggers/prompt"
)

func main() {
	originalPrompt := `
Generate 5 items sold in a fantasy shop, owned by a Bunny.
[
    {
        "name": string,
        "type": "sword" | "potion" | "misc",
        "price": number + " coins" // should range from 5 to 20 coins
    }
]
`

	minifiedPrompt := prompt.FromString(originalPrompt, prompt.AddOnJSONOutput)
	stringified := fmt.Sprintf("%q", minifiedPrompt)
	fmt.Println(stringified)

	fmt.Println("------------------------------------------------")

	minifiedPromptFromFile, err := prompt.FromFile("./examples/1_bunny_shop.prompt", prompt.AddOnJSONOutput)
	if err != nil {
		fmt.Println(err)
		return
	}
	stringifiedFromFile := fmt.Sprintf("%q", minifiedPromptFromFile)
	fmt.Println(stringifiedFromFile)
}
