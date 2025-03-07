package slackbot

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lambda/lambdachat-slackbot/internal/lambdachat"
	"github.com/lambda/lambdachat-slackbot/internal/webui"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// SlackBotter is the interface for interacting with the Slack API
type SlackBotter interface {
	// Run starts the Slack bot
	Run() error
}

// threadData stores information about an active thread
type threadData struct {
	threadTs    string    // The thread timestamp
	lastUpdated time.Time // When the thread was last active
}

type slackBot struct {
	client         *socketmode.Client
	lambdaChat     lambdachat.LambdaChatter
	l              *logrus.Entry
	messageMu      sync.Mutex
	messageBuffers map[string]*strings.Builder
	webUI          *webui.WebUI
	// Track active threads to handle continued conversation
	threadsMu     sync.RWMutex
	activeThreads map[string]threadData // Maps channel+user -> thread data
	// Thread expiration time
	threadExpiration time.Duration
}

type logger struct {
	l *logrus.Entry
}

func (l logger) Output(i int, s string) error {
	switch i {
	case 1: // Debug
		l.l.Debug(s)
	case 2: // Info
		l.l.Info(s)
	case 3: // Warn
		l.l.Warn(s)
	}
	return nil
}

// New creates a new SlackBotter instance
func New(l *logrus.Entry, appToken, botToken string, lambdaChat lambdachat.LambdaChatter, webUI *webui.WebUI) (SlackBotter, error) {
	ll := logger{
		l: l.WithField("slack-bot", "socketmode"),
	}

	api := slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionLog(ll),
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(ll),
	)

	return &slackBot{
		client:           client,
		lambdaChat:       lambdaChat,
		l:                l,
		messageBuffers:   make(map[string]*strings.Builder),
		webUI:            webUI,
		activeThreads:    make(map[string]threadData),
		threadExpiration: 1 * time.Hour, // Default expiration time: 1 hour
	}, nil
}

// Run starts the Slack bot
func (sb *slackBot) Run() error {
	go sb.handleEvents()
	return sb.client.Run()
}

// handleEvents processes events from the Slack API
func (sb *slackBot) handleEvents() {
	for evt := range sb.client.Events {
		switch evt.Type {
		case socketmode.EventTypeConnecting:
			sb.l.Info("Connecting to Slack with Socket Mode...")
		case socketmode.EventTypeConnectionError:
			sb.l.Info("Connection failed. Retrying later...")
		case socketmode.EventTypeConnected:
			sb.l.Info("Connected to Slack with Socket Mode.")
		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				continue
			}

			sb.l.Infof("Event received: %+v", eventsAPIEvent)

			// Acknowledge the event
			sb.client.Ack(*evt.Request)

			switch eventsAPIEvent.Type {
			case slackevents.CallbackEvent:
				innerEvent := eventsAPIEvent.InnerEvent
				switch ev := innerEvent.Data.(type) {
				case *slackevents.AppMentionEvent:
					// Handle app mention events (when the bot is @mentioned)
					go sb.handleAppMention(ev)
				case *slackevents.MessageEvent:
					// Check if message is in a thread we're tracking
					if ev.ThreadTimeStamp != "" {
						go sb.handleThreadMessage(ev)
					} else if ev.ChannelType == "im" && ev.BotID == "" {
						// Handle direct messages to the bot
						go sb.handleDirectMessage(ev)
					}
				}
			}
		case socketmode.EventTypeInteractive:
			callback, ok := evt.Data.(slack.InteractionCallback)
			if !ok {
				sb.l.Infof("Ignored %+v", evt)
				continue
			}

			sb.l.Infof("Interaction received: %+v", callback)
			sb.client.Ack(*evt.Request, nil)

		case socketmode.EventTypeSlashCommand:
			cmd, ok := evt.Data.(slack.SlashCommand)
			if !ok {
				sb.l.Infof("Ignored %+v", evt)
				continue
			}

			sb.l.Debugf("Slash command received: %+v", cmd)
			sb.client.Ack(*evt.Request, nil)

			// Handle slash commands
			go sb.handleSlashCommand(cmd)

		default:
			fmt.Fprintf(os.Stderr, "Unexpected event type received: %s\n", evt.Type)
		}
	}
}

// trackThread associates a thread with a channel+user combination
func (sb *slackBot) trackThread(channel, user, threadTs string) {
	key := fmt.Sprintf("%s-%s", channel, user)
	sb.threadsMu.Lock()
	sb.activeThreads[key] = threadData{
		threadTs:    threadTs,
		lastUpdated: time.Now(),
	}
	sb.threadsMu.Unlock()
}

// updateThreadTimestamp updates the last activity timestamp for a thread
func (sb *slackBot) updateThreadTimestamp(channel, user string) {
	key := fmt.Sprintf("%s-%s", channel, user)
	sb.threadsMu.Lock()
	if data, exists := sb.activeThreads[key]; exists {
		data.lastUpdated = time.Now()
		sb.activeThreads[key] = data
	}
	sb.threadsMu.Unlock()
}

// isThreadExpired checks if a thread has expired (inactive for more than the expiration time)
func (sb *slackBot) isThreadExpired(lastUpdated time.Time) bool {
	return time.Since(lastUpdated) > sb.threadExpiration
}

// getThreadForUser retrieves the active thread timestamp for a channel+user
func (sb *slackBot) getThreadForUser(channel, user string) string {
	key := fmt.Sprintf("%s-%s", channel, user)
	sb.threadsMu.RLock()
	threadData, exists := sb.activeThreads[key]
	sb.threadsMu.RUnlock()

	if !exists {
		return ""
	}

	// Check if the thread has expired
	if sb.isThreadExpired(threadData.lastUpdated) {
		return ""
	}

	return threadData.threadTs
}

// handleThreadMessage handles messages posted in threads where the bot is active
func (sb *slackBot) handleThreadMessage(ev *slackevents.MessageEvent) {
	// Skip messages from bots or the bot itself
	if ev.BotID != "" || ev.User == "" {
		return
	}

	// Create a unique user ID using the Slack user ID
	userID := fmt.Sprintf("slack-user-%s", ev.User)

	// Log the user input to the web UI
	if sb.webUI != nil {
		sb.webUI.LogUserInput(ev.User, ev.Channel, ev.Text)
	}

	// Check if we need to reload context from thread
	key := fmt.Sprintf("%s-%s", ev.Channel, ev.User)
	sb.threadsMu.RLock()
	threadData, exists := sb.activeThreads[key]
	threadExpired := exists && sb.isThreadExpired(threadData.lastUpdated)
	sb.threadsMu.RUnlock()

	// If the thread exists but has expired, or this is a different thread than the one we're tracking
	if (exists && threadExpired) || (exists && threadData.threadTs != ev.ThreadTimeStamp) {
		// Reload context by resetting the user's context
		_ = sb.lambdaChat.Reset(userID)
		sb.l.Infof("Thread expired or changed, reloading context for user %s in channel %s", ev.User, ev.Channel)

		// Track this thread as the new active thread
		sb.trackThread(ev.Channel, ev.User, ev.ThreadTimeStamp)
	} else if exists {
		// Update the last activity timestamp for this thread
		sb.updateThreadTimestamp(ev.Channel, ev.User)
	} else {
		// This is a new thread for this user, track it
		sb.trackThread(ev.Channel, ev.User, ev.ThreadTimeStamp)
	}

	// Create a message writer that will collect the Slack thread message
	messageWriter := sb.createThreadMessageWriter(ev.Channel, ev.User, ev.ThreadTimeStamp)

	// Process the message and collect the response
	err := sb.lambdaChat.ChatStream(userID, ev.Text, messageWriter)
	if err != nil {
		sb.l.Errorf("Failed to process thread message: %v", err)
		_, _, err = sb.client.Client.PostMessage(
			ev.Channel,
			slack.MsgOptionText(fmt.Sprintf("Error: %v", err), false),
			slack.MsgOptionTS(ev.ThreadTimeStamp),
		)
		if err != nil {
			sb.l.Errorf("Failed to post error message: %v", err)
		}
		return
	}

	// Send the complete response all at once
	if writer, ok := messageWriter.(*multiWriter); ok {
		if err := writer.Flush(); err != nil {
			sb.l.Errorf("Failed to flush thread response: %v", err)
		}
	}
}

// handleAppMention handles app mention events
func (sb *slackBot) handleAppMention(ev *slackevents.AppMentionEvent) {
	// Extract the message text without the bot mention
	text := strings.TrimSpace(strings.TrimPrefix(ev.Text, fmt.Sprintf("<@%s>", ev.BotID)))

	// Create a unique user ID using the Slack user ID
	userID := fmt.Sprintf("slack-user-%s", ev.User)

	// Log the user input to the web UI
	if sb.webUI != nil {
		sb.webUI.LogUserInput(ev.User, ev.Channel, text)
	}

	// Determine thread timestamp to use:
	var threadTs string

	// If the mention is already in a thread, use that thread
	if ev.ThreadTimeStamp != "" {
		threadTs = ev.ThreadTimeStamp

		// Check if thread is expired
		key := fmt.Sprintf("%s-%s", ev.Channel, ev.User)
		sb.threadsMu.RLock()
		threadData, exists := sb.activeThreads[key]
		threadExpired := exists && sb.isThreadExpired(threadData.lastUpdated)
		sb.threadsMu.RUnlock()

		if (exists && threadExpired) || (exists && threadData.threadTs != ev.ThreadTimeStamp) {
			// Reload context by resetting the user's context
			_ = sb.lambdaChat.Reset(userID)
			sb.l.Infof("Thread expired or changed, reloading context for user %s in channel %s", ev.User, ev.Channel)
		}

		// Track or update this thread
		sb.trackThread(ev.Channel, ev.User, threadTs)
	} else {
		// If not in a thread, use the message timestamp as the parent for a new thread
		// This makes the bot reply directly to the mention message
		threadTs = ev.TimeStamp

		// Track this new thread
		sb.trackThread(ev.Channel, ev.User, threadTs)
	}

	// Create a message writer that will collect the Slack thread message
	messageWriter := sb.createThreadMessageWriter(ev.Channel, ev.User, threadTs)

	// Process the message and collect the response
	str, err := sb.lambdaChat.Chat(userID, text)
	if err != nil {
		sb.l.Errorf("Failed to process message: %v", err)
		_, _, err = sb.client.Client.PostMessage(
			ev.Channel,
			slack.MsgOptionText(fmt.Sprintf("Error: %v", err), false),
			slack.MsgOptionTS(threadTs),
		)
		if err != nil {
			sb.l.Errorf("Failed to post error message: %v", err)
		}
		return
	}

	// Write the complete response
	messageWriter.Write([]byte(str))

	// Send the complete response all at once
	if writer, ok := messageWriter.(*multiWriter); ok {
		if err := writer.Flush(); err != nil {
			sb.l.Errorf("Failed to flush app mention response: %v", err)
		}
	}
}

// handleDirectMessage handles direct messages to the bot
func (sb *slackBot) handleDirectMessage(ev *slackevents.MessageEvent) {
	// Create a unique user ID using the Slack user ID
	userID := fmt.Sprintf("slack-user-%s", ev.User)

	// Log the user input to the web UI
	if sb.webUI != nil {
		sb.webUI.LogUserInput(ev.User, ev.Channel, ev.Text)
	}

	// Determine thread timestamp to use:
	var threadTs string

	// Check if we have an ongoing thread with this user in this channel
	existingThreadTs := sb.getThreadForUser(ev.Channel, ev.User)

	if existingThreadTs != "" {
		// We have an active, non-expired thread
		threadTs = existingThreadTs
		sb.updateThreadTimestamp(ev.Channel, ev.User)
	} else {
		// Check if there's an expired thread
		key := fmt.Sprintf("%s-%s", ev.Channel, ev.User)
		sb.threadsMu.RLock()
		threadData, exists := sb.activeThreads[key]
		threadExpired := exists && sb.isThreadExpired(threadData.lastUpdated)
		sb.threadsMu.RUnlock()

		if exists && threadExpired {
			// Reset context for expired thread
			_ = sb.lambdaChat.Reset(userID)
			sb.l.Infof("Thread expired, reloading context for user %s in channel %s", ev.User, ev.Channel)

			// Use the existing thread but update its timestamp
			threadTs = threadData.threadTs
			sb.trackThread(ev.Channel, ev.User, threadTs)
		} else {
			// No thread exists or it's a different thread, create a new one
			threadTs = ev.TimeStamp
			sb.trackThread(ev.Channel, ev.User, threadTs)
		}
	}

	// Create a message writer that will collect the Slack thread message
	messageWriter := sb.createThreadMessageWriter(ev.Channel, ev.User, threadTs)

	// Process the message and collect the response
	err := sb.lambdaChat.ChatStream(userID, ev.Text, messageWriter)
	if err != nil {
		sb.l.Errorf("Failed to process message: %v", err)
		if _, _, err = sb.client.Client.PostMessage(
			ev.Channel,
			slack.MsgOptionText(fmt.Sprintf("Error: %v", err), false),
			slack.MsgOptionTS(threadTs),
		); err != nil {
			sb.l.Errorf("Failed to post error message: %v", err)
		}
		return
	}

	// Send the complete response all at once
	if writer, ok := messageWriter.(*multiWriter); ok {
		if err := writer.Flush(); err != nil {
			sb.l.Errorf("Failed to flush direct message response: %v", err)
		}
	}
}

// handleSlashCommand handles slash commands
func (sb *slackBot) handleSlashCommand(cmd slack.SlashCommand) {
	// Create a unique user ID using the Slack user ID
	userID := fmt.Sprintf("slack-user-%s", cmd.UserID)

	lm := fmt.Sprintf("User %s event", userID)
	defer func() {
		if sb.webUI != nil {
			sb.webUI.Log(lm)
		}
	}()

	// Handle different commands
	switch strings.ToLower(cmd.Command) {
	case "/reset":
		_ = sb.lambdaChat.Reset(userID)
		sb.threadsMu.Lock()
		delete(sb.activeThreads, fmt.Sprintf("%s-%s", cmd.ChannelID, cmd.UserID))
		sb.threadsMu.Unlock()

		// Send a confirmation message
		if _, _, msgErr := sb.client.Client.PostMessage(
			cmd.ChannelID,
			slack.MsgOptionText("*Conversation has been reset.*", false),
			slack.MsgOptionPostEphemeral(cmd.UserID),
		); msgErr != nil {
			_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText("*Conversation has been reset.*", false))
			sb.l.Errorf("Failed to post reset confirmation: %v", msgErr)
		}

		lm = fmt.Sprintf("User %s reset conversation", cmd.UserID)

	case "/persona":
		if cmd.Text == "" {
			// No persona name provided, so list available personas
			personas := sb.lambdaChat.GetAvailablePersonas()
			message := "*Available personas:*\n• " + strings.Join(personas, "\n• ")

			if _, _, msgErr := sb.client.Client.PostMessage(
				cmd.ChannelID,
				slack.MsgOptionText(message, false),
				slack.MsgOptionPostEphemeral(cmd.UserID),
			); msgErr != nil {
				_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(message, false))
				sb.l.Errorf("Failed to post personas list: %v", msgErr)
			}

			lm = fmt.Sprintf("User %s requested available personas", cmd.UserID)
			return
		}

		response, err := sb.lambdaChat.SetPersona(userID, cmd.Text)
		if err != nil {
			sb.l.Errorf("Failed to set persona: %v", err)
			errorMsg := fmt.Sprintf("Error setting persona: %v\nAvailable personas: %s",
				err, strings.Join(sb.lambdaChat.GetAvailablePersonas(), ", "))

			if _, _, msgErr := sb.client.Client.PostMessage(
				cmd.ChannelID,
				slack.MsgOptionText(errorMsg, false),
				slack.MsgOptionPostEphemeral(cmd.UserID),
			); msgErr != nil {
				_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(errorMsg, false))
				sb.l.Errorf("Failed to post error message: %v", msgErr)
			}
			return
		}

		var msgErr error
		if _, _, msgErr = sb.client.Client.PostMessage(
			cmd.ChannelID,
			slack.MsgOptionText(response, false),
			slack.MsgOptionPostEphemeral(cmd.UserID),
		); msgErr != nil {
			_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(response, false))
			sb.l.Errorf("Failed to post persona change confirmation: %v", msgErr)
		}

		lm = fmt.Sprintf("User %s changed persona to %s", cmd.UserID, cmd.Text)

	case "/personas":
		personas := sb.lambdaChat.GetAvailablePersonas()
		message := "*Available personas:*\n• " + strings.Join(personas, "\n• ")

		if _, _, msgErr := sb.client.Client.PostMessage(
			cmd.ChannelID,
			slack.MsgOptionText(message, false),
			slack.MsgOptionPostEphemeral(cmd.UserID),
		); msgErr != nil {
			_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(message, false))
			sb.l.Errorf("Failed to post personas list: %v", msgErr)
		}
		lm = fmt.Sprintf("User %s requested available personas", cmd.UserID)

	case "/models":
		// List available models
		models := sb.lambdaChat.GetAvailableModels()
		message := "*Available models:*\n• " + strings.Join(models, "\n• ")

		if _, _, msgErr := sb.client.Client.PostMessage(
			cmd.ChannelID,
			slack.MsgOptionText(message, false),
			slack.MsgOptionPostEphemeral(cmd.UserID),
		); msgErr != nil {
			_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(message, false))
			sb.l.Errorf("Failed to post models list: %v", msgErr)
		}

	case "/model":
		if cmd.Text == "" {
			models := sb.lambdaChat.GetAvailableModels()
			message := "*Available models:*\n• " + strings.Join(models, "\n• ")

			if _, _, msgErr := sb.client.Client.PostMessage(
				cmd.ChannelID,
				slack.MsgOptionText(message, false),
				slack.MsgOptionPostEphemeral(cmd.UserID),
			); msgErr != nil {
				_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(message, false))
				sb.l.Errorf("Failed to post models list: %v", msgErr)
			}

			lm = fmt.Sprintf("User %s requested available models", cmd.UserID)
			return
		}

		stripped := strings.Replace(cmd.Text, "*", "", -1)
		response, err := sb.lambdaChat.SetModel(userID, stripped)

		if err != nil {
			sb.l.Errorf("Failed to set model: %v", err)
			errorMsg := fmt.Sprintf("Error setting model: %v\nAvailable Models: \n• %s",
				err, strings.Join(sb.lambdaChat.GetAvailableModels(), "\n• "))

			if _, _, msgErr := sb.client.Client.PostMessage(
				cmd.ChannelID,
				slack.MsgOptionText(errorMsg, false),
				slack.MsgOptionPostEphemeral(cmd.UserID),
			); msgErr != nil {
				_, _, msgErr = sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(errorMsg, false))
				sb.l.Errorf("Failed to post models list: %v", msgErr)
			}
			return
		}

		if _, _, msgErr := sb.client.Client.PostMessage(cmd.ChannelID, slack.MsgOptionText(response, false)); msgErr != nil {
			sb.l.Errorf("Failed to post model change confirmation: %v", msgErr)
		}

		lm = fmt.Sprintf("User %s changed model to %s", cmd.UserID, cmd.Text)
	}
}

// createThreadMessageWriter creates a writer that will update a Slack message in a thread
func (sb *slackBot) createThreadMessageWriter(channel, user, threadTs string) io.Writer {
	// Create a unique key for this message
	key := fmt.Sprintf("%s-%s", channel, user)

	// Create a new buffer for this message
	sb.messageMu.Lock()
	sb.messageBuffers[key] = &strings.Builder{}
	sb.messageMu.Unlock()

	// Return a writer that will update the Slack message as the response is generated
	return &multiWriter{
		slackWriter: &threadMessageWriter{
			bot:      sb,
			channel:  channel,
			key:      key,
			threadTs: threadTs,
		},
		webUI:   sb.webUI,
		user:    user,
		channel: channel,
		content: new(strings.Builder),
	}
}

// threadMessageWriter is a writer that collects the entire response and then posts it in a thread
type threadMessageWriter struct {
	bot      *slackBot
	channel  string
	key      string
	threadTs string
}

// Write implements the io.Writer interface
func (w *threadMessageWriter) Write(p []byte) (n int, err error) {
	// Add the new content to the buffer
	w.bot.messageMu.Lock()
	buffer, ok := w.bot.messageBuffers[w.key]
	if !ok {
		w.bot.messageMu.Unlock()
		return 0, fmt.Errorf("buffer not found for key %s", w.key)
	}

	n, err = buffer.Write(p)
	w.bot.messageMu.Unlock()
	return n, err
}

// Flush writes the collected content to Slack in a thread
func (w *threadMessageWriter) Flush() error {
	w.bot.messageMu.Lock()
	buffer, ok := w.bot.messageBuffers[w.key]
	if !ok {
		w.bot.messageMu.Unlock()
		return fmt.Errorf("buffer not found for key %s", w.key)
	}
	content := buffer.String()
	w.bot.messageMu.Unlock()

	// Send the complete message to Slack in the thread
	_, _, _, err := w.bot.client.Client.SendMessage(
		w.channel,
		slack.MsgOptionText(content, false),
		slack.MsgOptionTS(w.threadTs), // Reply in the thread
	)
	if err != nil {
		w.bot.l.Errorf("Failed to send complete thread message: %v", err)
	}
	return err
}

// multiWriter is a writer that collects content for both Slack and the web UI
type multiWriter struct {
	slackWriter interface {
		io.Writer
		Flush() error
	}
	webUI   *webui.WebUI
	user    string
	channel string
	content *strings.Builder
}

// Write implements the io.Writer interface
func (w *multiWriter) Write(p []byte) (n int, err error) {
	// Write to the slack buffer
	n, err = w.slackWriter.Write(p)
	if err != nil {
		return n, err
	}

	// Also store content for web UI
	if w.webUI != nil {
		w.content.Write(p)
	}

	return n, nil
}

// Flush sends the collected content to Slack and logs to the web UI
func (w *multiWriter) Flush() error {
	// Send the complete message to Slack
	err := w.slackWriter.Flush()

	// Log the complete response to the web UI
	if w.webUI != nil && w.content.Len() > 0 {
		w.webUI.LogResponse(w.user, w.channel, w.content.String())
	}

	return err
}
