package main

import (
	"fmt"

	"github.com/markettools-ai/poggers/prompt"
)

func main() {
	originalPrompt := `
system:
    Generate 5 items sold in a fantasy shop, owned by a Bunny.
	@OutputSchema
	[
	    {
	        "name": string,
	        "type": "sword" | "potion" | "misc",
	        "price": number + " coins" // should range from 5 to 20 coins
	    }
	]
	@JSONOutput
assistant:
	@BunnyInfo
`

	promptBuilder := prompt.NewPromptBuilder(
		map[string]string{
			"BunnyInfo": "This is the Bunny's information:",
		},
	)

	minifiedPrompt, err := promptBuilder.Process(originalPrompt)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(minifiedPrompt)
	// stringified := fmt.Sprintf("%q", minifiedPrompt)
	// fmt.Println(stringified)

	// fmt.Println("------------------------------------------------")

	// minifiedPromptFromFile, err := promptBuilder.ProcessFromFile("./examples/1_bunny_shop.prompt")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// stringifiedFromFile := fmt.Sprintf("%q", minifiedPromptFromFile)
	// fmt.Println(stringifiedFromFile)
}
