package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lambda/lambdachat-slackbot/internal/lambdachat"
	"github.com/lambda/lambdachat-slackbot/internal/slackbot"
	"github.com/lambda/lambdachat-slackbot/internal/webui"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var version = "0.0.1~dev"

var rootOptions = struct {
	url       string
	apiKey    string
	appToken  string
	botToken  string
	model     string
	persona   string
	logLevel  string
	webUI     bool
	webUIAddr string
}{}

var ctx, cancel = context.WithCancel(context.Background())

var log = logrus.New().WithContext(ctx).WithField("lambda-chat", "slackbot")

var rootCmd = &cobra.Command{
	Use:     "slackbot",
	Short:   "slackbot - A Slack bot that uses Lambda Chat to respond to messages",
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Set log level
		level, err := logrus.ParseLevel(rootOptions.logLevel)
		if err != nil {
			log.WithError(err).Warnf("Invalid log level %q, using info", rootOptions.logLevel)
			level = logrus.InfoLevel
		}
		logrus.SetLevel(level)

		// Validate required options
		if rootOptions.apiKey == "" {
			log.Fatal("API Key is required")
		}
		if rootOptions.url == "" {
			log.Fatal("Host is required")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Starting Slack Bot")

		// Create the Lambda Chat client
		model := rootOptions.model
		if model == "" {
			model = lambdachat.DefaultModel
		}

		persona := rootOptions.persona
		if persona == "" {
			persona = lambdachat.PersonaBender
		}

		lc, err := lambdachat.New(log, rootOptions.url, rootOptions.apiKey, model, persona)
		if err != nil {
			log.WithError(err).Fatal("Failed to create Lambda Chat client")
		}

		// Initialize WebUI if enabled
		var ui *webui.WebUI
		if rootOptions.webUI {
			ui = webui.New(log.WithField("component", "webui"))

			// Add logrus hook to send logs to UI
			logHook := webui.NewLogrusHook(ui)
			logrus.AddHook(logHook)

			// Start the WebUI server in a goroutine
			go func() {
				log.Infof("Starting WebUI server on %s", rootOptions.webUIAddr)
				if err := ui.Start(rootOptions.webUIAddr); err != nil {
					log.WithError(err).Error("WebUI server error")
				}
			}()
		}

		// Create the Slack bot
		bot, err := slackbot.New(log, rootOptions.appToken, rootOptions.botToken, lc, ui)
		if err != nil {
			log.WithError(err).Fatal("Failed to create Slack bot")
		}

		// Set up signal handling for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Start the bot in a goroutine
		errCh := make(chan error, 1)
		go func() {
			errCh <- bot.Run()
		}()

		// Wait for either an error or a signal
		select {
		case err := <-errCh:
			if err != nil {
				log.WithError(err).Fatal("Slack bot error")
			}
		case sig := <-sigCh:
			log.Infof("Received signal %v, shutting down", sig)
		}

		log.Info("Slack bot stopped")
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootOptions.apiKey, "api-key", "", "", "Lambda Chat API Key")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.url, "host", "", lambdachat.LambdaChatURL, "Lambda Chat Host")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.appToken, "app-token", "", "", "Slack App Token (starts with xapp-)")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.botToken, "bot-token", "", "", "Slack Bot Token (starts with xoxb-)")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.model, "model", "", lambdachat.DefaultModel, "Lambda Chat Model (default: deepseek-llama3.3-70b)")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.persona, "persona", "", lambdachat.PersonaHelpfulAssistant, "Lambda Chat Persona (default: Bender)")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.logLevel, "log-level", "", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVarP(&rootOptions.webUI, "webui", "", true, "Enable WebUI for logging and monitoring")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.webUIAddr, "webui-addr", "", ":8080", "WebUI server address")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatal("Failed to execute command")
	}
	cancel()
}
