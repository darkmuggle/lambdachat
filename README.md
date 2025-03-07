# LambdaChat Slackbot

A streaming socket Slackbot that responds to user inputs using an OpenAI-compatible endpoint. The bot handles multiple users and retains context until a user says "/reset".

## Features

- **Streaming Responses**: Responses are streamed in real-time to Slack as they are generated
- **Multi-User Support**: Each user has their own conversation history
- **Context Retention**: Conversation context is maintained until explicitly reset
- **Reset Command**: Users can type "/reset" to clear their conversation history
- **Library Implementation**: Can be used as a library in other Go applications

## Requirements

- Go 1.23.4 or later
- A Slack App with the following permissions:
  - `app_mentions:read`
  - `chat:write`
  - `im:history`
  - `im:read`
  - `im:write`
- A Slack App Token (starts with `xapp-`)
- A Slack Bot Token (starts with `xoxb-`)
- An OpenAI-compatible API endpoint (e.g., Lambda Labs API)

## Installation

### Using go get

```bash
go get github.com/lambda/lambdachat-slackbot
```

### Building from Source

Clone the repository and use the provided Makefile:

```bash
git clone https://github.com/lambda/lambdachat-slackbot.git
cd lambdachat-slackbot
make init   # Create necessary directories
make build  # Build all binaries
```

The Makefile provides several useful targets:

- `make build` - Build all binaries
- `make build-slackbot` - Build only the slackbot binary
- `make build-cli` - Build only the CLI binary
- `make clean` - Clean up binaries
- `make test` - Run tests
- `make run-slackbot` - Run the slackbot example
- `make run-direct` - Run the direct example

## Usage

### As a Library in a Slack Bot

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lambda/lambdachat-slackbot/internal/lib"
	"github.com/sirupsen/logrus"
)

func main() {
	// Set up logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logEntry := logrus.NewEntry(logger)

	// Create the SlackBot configuration
	config := lib.Config{
		LambdaChatURL: os.Getenv("LAMBDA_CHAT_URL"),
		APIKey:        os.Getenv("LAMBDA_CHAT_API_KEY"),
		Model:         os.Getenv("LAMBDA_CHAT_MODEL"),
		Persona:       os.Getenv("LAMBDA_CHAT_PERSONA"),
		SlackAppToken: os.Getenv("SLACK_APP_TOKEN"),
		SlackBotToken: os.Getenv("SLACK_BOT_TOKEN"),
		Logger:        logEntry,
	}

	// Create the SlackBot
	bot, err := lib.NewSlackBot(config)
	if err != nil {
		log.Fatalf("Failed to create SlackBot: %v", err)
	}

	// Start the bot
	if err := bot.Run(); err != nil {
		log.Fatalf("SlackBot error: %v", err)
	}
}
```

### As a Command-Line Tool

1. Build the slackbot command:

```bash
go build -o slackbot ./cmd/slackbot
```

2. Run the slackbot:

```bash
./slackbot --api-key=YOUR_API_KEY --app-token=YOUR_SLACK_APP_TOKEN --bot-token=YOUR_SLACK_BOT_TOKEN
```

Or with environment variables:

```bash
export LAMBDA_CHAT_API_KEY=YOUR_API_KEY
export SLACK_APP_TOKEN=YOUR_SLACK_APP_TOKEN
export SLACK_BOT_TOKEN=YOUR_SLACK_BOT_TOKEN
./slackbot
```

## Configuration

The following configuration options are available:

| Option | Environment Variable | Description | Default |
|--------|---------------------|-------------|---------|
| `--api-key` | `LAMBDA_CHAT_API_KEY` | Lambda Chat API Key | (required) |
| `--host` | `LAMBDA_CHAT_URL` | Lambda Chat Host | https://api.lambdalabs.com/v1 |
| `--app-token` | `SLACK_APP_TOKEN` | Slack App Token | (required) |
| `--bot-token` | `SLACK_BOT_TOKEN` | Slack Bot Token | (required) |
| `--model` | `LAMBDA_CHAT_MODEL` | Lambda Chat Model | deepseek-llama3.3-70b |
| `--persona` | `LAMBDA_CHAT_PERSONA` | Lambda Chat Persona | (Bender persona) |
| `--log-level` | `LOG_LEVEL` | Log level (debug, info, warn, error) | info |

### Direct Usage with LambdaChatter

You can also use the LambdaChatter interface directly without the Slack integration:

```go
package main

import (
	"fmt"
	"os"

	"github.com/lambda/lambdachat-slackbot/internal/lambdachat"
	"github.com/sirupsen/logrus"
)

func main() {
	// Set up logging
	logger := logrus.New()
	logEntry := logrus.NewEntry(logger)

	// Create the LambdaChatter
	lc, err := lambdachat.New(
		logEntry,
		os.Getenv("LAMBDA_CHAT_URL"),
		os.Getenv("LAMBDA_CHAT_API_KEY"),
		lambdachat.ModelDeepSeak,
		lambdachat.PersonaBender,
	)
	if err != nil {
		fmt.Printf("Failed to create LambdaChatter: %v\n", err)
		os.Exit(1)
	}

	// Use non-streaming API
	response, err := lc.Chat("user123", "Hello, how are you?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(response)

	// Or use streaming API
	err = lc.ChatStream("user123", "Tell me a story", os.Stdout)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Reset conversation context
	err = lc.Reset("user123")
	if err != nil {
		fmt.Printf("Error resetting conversation: %v\n", err)
	}
}
```

## Interacting with the Slack Bot

- **Direct Messages**: Send a direct message to the bot
- **Mentions**: Mention the bot in a channel (`@YourBot Hello!`)
- **Reset**: Type `/reset` to clear your conversation history

## License

MIT
