<p align="center">
  <img src="https://github.com/markettools-ai/poggers/assets/20731019/d560920e-d8ff-4180-846c-b603a5ba35ee" width="100">
  <h1 align="center">Welcome to poggers!</h1>
</p>

**poggers** is a Golang library and an IDL that assists in creating detailed, multi-level, optimized, and human-readable LLM prompts.

> Make sure to download poggers' [VSCode Extension](https://marketplace.visualstudio.com/items?itemName=markettools-ai.poggers-prompt)!

## Features
- **poggers' IDL**: Write your prompts in a human-readable way and separate them by files. Use annotations, labels, comments, objects, etc. ([Syntax highlighting included!](https://marketplace.visualstudio.com/items?itemName=markettools-ai.poggers-prompt))
- **Request Optimization**: **poggers** will optimize your prompts to remove as many unnecessary tokens as possible.
- **Complete Execution Control**: Run independent prompts concurrently, mix them, add their result as a parameter to another prompt, etc.
- **Model Adaptability**: Although **poggers** is being designed with focus on OpenAI's API for GPT models, it can be easily adapted to any other AI.

## Usage
`grandma.prompt`
```json
system:
  Generate a short story about a grandma that has become a @GrandmasFate to @GrandmasMission.
```
`main.go`
```go
package main

import (
  "fmt"
  "github.com/markettools-ai/poggers"
)

func main() {
  // Create a new prompt builder
  promptBuilder := poggers.NewPromptBuilder(
    // Define annotations on initialization
    map[string]string{
      "GrandmasFate": "superhero",
    },
  )
  // Annotations can also be set individually
  promptBuilder.SetAnnotation("GrandmasMission", "save the world")

  // Process the prompt file
  msgs, err := poggers.ProcessFromFile("./grandma.prompt")
  if err != nil {
    panic(err)
  }

  // msgs is a slice of optimized messages that can be sent to the AI
}
```

## Examples
For complete examples on how **poggers** works, check out [poggers-quest](https://github.com/markettools-ai/poggers-quest).
