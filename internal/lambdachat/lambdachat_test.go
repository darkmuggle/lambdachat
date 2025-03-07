package lambdachat

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// MockOpenAIClient is a mock implementation of the OpenAI client for testing
type MockOpenAIClient struct {
	// Add fields as needed for testing
}

func TestReset(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Initialize a conversation for a test user
	userID := "test-user"
	lc.conversations[userID] = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: lc.defaultPersona,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Hello",
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "Hi there!",
		},
	}

	// Reset the conversation
	err := lc.Reset(userID)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Check that the conversation was reset to just the persona
	if len(lc.conversations[userID]) != 1 {
		t.Errorf("Expected conversation length to be 1 after reset, got %d", len(lc.conversations[userID]))
	}

	// Check that the first message is the persona
	firstMsg := lc.conversations[userID][0]
	if firstMsg.Role != openai.ChatMessageRoleSystem {
		t.Errorf("Expected first message role to be 'system', got %s", firstMsg.Role)
	}

	if firstMsg.Content != lc.defaultPersona {
		t.Errorf("Expected first message content to be the persona, got %s", firstMsg.Content)
	}
}

func TestChatResetCommand(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance with a mock client
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Initialize a conversation for a test user
	userID := "test-user"
	lc.conversations[userID] = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: lc.defaultPersona,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Hello",
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "Hi there!",
		},
	}

	// Test the /reset command
	response, err := lc.Chat(userID, "/reset")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Check the response
	if !strings.Contains(response, "reset") {
		t.Errorf("Expected response to contain 'reset', got %s", response)
	}

	// Check that the conversation was reset to just the persona
	if len(lc.conversations[userID]) != 1 {
		t.Errorf("Expected conversation length to be 1 after reset, got %d", len(lc.conversations[userID]))
	}
}

func TestChatStreamResetCommand(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Initialize a conversation for a test user
	userID := "test-user"
	lc.conversations[userID] = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: lc.defaultPersona,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Hello",
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "Hi there!",
		},
	}

	// Create a buffer to capture the output
	var buf bytes.Buffer

	// Test the /reset command
	err := lc.ChatStream(userID, "/reset", &buf)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	// Check the response
	if !strings.Contains(buf.String(), "reset") {
		t.Errorf("Expected response to contain 'reset', got %s", buf.String())
	}

	// Check that the conversation was reset to just the persona
	if len(lc.conversations[userID]) != 1 {
		t.Errorf("Expected conversation length to be 1 after reset, got %d", len(lc.conversations[userID]))
	}
}

func TestPersonaCodingAssistant(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Test setting the coder persona
	userID := "test-user"
	response, err := lc.SetPersona(userID, "coder")
	if err != nil {
		t.Fatalf("SetPersona failed: %v", err)
	}

	// Check the response
	if !strings.Contains(response, "Coding Assistant") {
		t.Errorf("Expected response to contain 'Coding Assistant', got %s", response)
	}

	// Check that the persona was set correctly
	if lc.userPersonas[userID] != PersonaCodingAssistant {
		t.Errorf("Expected persona to be set to PersonaCodingAssistant")
	}
}

func TestModelCommand(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		userModels:     make(map[string]string),
		model:          DefaultModel,
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
		availableModels: []modelInfo{
			{
				ID:          "deepseek-llama3.3-70b",
				DisplayName: "DeepSeek Llama 3.3 70B",
				Aliases:     []string{"deepseek", "deepseek-llama"},
			},
			{
				ID:          "hermes-70b",
				DisplayName: "Hermes 70B",
				Aliases:     []string{"hermes70b", "hermes70"},
			},
			{
				ID:          "qwen-25-coder",
				DisplayName: "Qwen 25 Coder",
				Aliases:     []string{"qwen", "qwen25", "coder"},
				AutoPersona: PersonaCodingAssistant,
			},
		},
	}

	// Test setting a model
	userID := "test-user"
	response, err := lc.Chat(userID, "/model hermes-70b")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Check the response
	if !strings.Contains(response, "Model changed to Hermes 70B") {
		t.Errorf("Expected response to contain 'Model changed to Hermes 70B', got %s", response)
	}

	// Check that the model was set correctly
	if lc.userModels[userID] != "hermes-70b" {
		t.Errorf("Expected model to be set to hermes-70b, got %s", lc.userModels[userID])
	}
}

func TestModelQwenSetsPersona(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		userModels:     make(map[string]string),
		model:          DefaultModel,
		defaultPersona: PersonaBender,
		l:              logEntry,
		ctx:            context.Background(),
		availableModels: []modelInfo{
			{
				ID:          "deepseek-llama3.3-70b",
				DisplayName: "DeepSeek Llama 3.3 70B",
				Aliases:     []string{"deepseek", "deepseek-llama"},
			},
			{
				ID:          "qwen-25-coder",
				DisplayName: "Qwen 25 Coder",
				Aliases:     []string{"qwen", "qwen25", "coder"},
				AutoPersona: PersonaCodingAssistant,
			},
		},
	}

	// Test setting Qwen model which should auto-set the persona
	userID := "test-user"
	response, err := lc.SetModel(userID, "qwen")
	if err != nil {
		t.Fatalf("SetModel failed: %v", err)
	}

	// Check the response includes both model and persona info
	if !strings.Contains(response, "Model changed to Qwen 25 Coder") ||
		!strings.Contains(response, "persona automatically set to Coding Assistant") {
		t.Errorf("Expected response to contain model and persona change info, got %s", response)
	}

	// Check that the model was set correctly
	if lc.userModels[userID] != "qwen-25-coder" {
		t.Errorf("Expected model to be set to qwen-25-coder, got %s", lc.userModels[userID])
	}

	// Check that the persona was also set correctly
	if lc.userPersonas[userID] != PersonaCodingAssistant {
		t.Errorf("Expected persona to be set to PersonaCodingAssistant, got %s", lc.userPersonas[userID])
	}
}

func TestChatModelsCommand(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance with a mock client
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Test the /models command
	userID := "test-user"
	response, err := lc.Chat(userID, "/models")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Check that the response contains the expected models
	if !strings.Contains(response, "deepseek-llama3.3-70b") ||
		!strings.Contains(response, "hermes-405b") ||
		!strings.Contains(response, "hermes-70b") ||
		!strings.Contains(response, "qwen-25-coder") {
		t.Errorf("Expected response to contain all models, got %s", response)
	}

	// Ensure conversation wasn't modified/lost by the command
	conversation := lc.getConversation(userID)
	if len(conversation) != 1 {
		t.Errorf("Expected conversation length to still be 1, got %d", len(conversation))
	}
}

func TestChatStreamModelsCommand(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Create a buffer to capture the output
	var buf bytes.Buffer

	// Test the /models command
	userID := "test-user"
	err := lc.ChatStream(userID, "/models", &buf)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	// Check that the response contains the expected models
	response := buf.String()
	if !strings.Contains(response, "deepseek-llama3.3-70b") ||
		!strings.Contains(response, "hermes-405b") ||
		!strings.Contains(response, "hermes-70b") ||
		!strings.Contains(response, "qwen-25-coder") {
		t.Errorf("Expected response to contain all models, got %s", response)
	}

	// Ensure conversation wasn't modified/lost by the command
	conversation := lc.getConversation(userID)
	if len(conversation) != 1 {
		t.Errorf("Expected conversation length to still be 1, got %d", len(conversation))
	}
}

func TestGetConversation(t *testing.T) {
	// Create a new logger for testing
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Discard log output for tests
	logEntry := logrus.NewEntry(logger)

	// Create a new lambdaChat instance
	lc := &lambdaChat{
		conversations:  make(map[string][]openai.ChatCompletionMessage),
		userPersonas:   make(map[string]string),
		defaultPersona: "Test Persona",
		l:              logEntry,
		ctx:            context.Background(),
	}

	// Test getting a conversation for a new user
	userID := "new-user"
	conversation := lc.getConversation(userID)

	// Check that a new conversation was created
	if len(conversation) != 1 {
		t.Errorf("Expected new conversation length to be 1, got %d", len(conversation))
	}

	// Check that the conversation was added to the map
	if _, exists := lc.conversations[userID]; !exists {
		t.Errorf("Expected conversation to be added to the map")
	}

	// Test getting an existing conversation
	existingUserID := "existing-user"
	lc.conversations[existingUserID] = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: lc.defaultPersona,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Hello",
		},
	}

	existingConversation := lc.getConversation(existingUserID)

	// Check that the existing conversation was returned
	if len(existingConversation) != 2 {
		t.Errorf("Expected existing conversation length to be 2, got %d", len(existingConversation))
	}
}
