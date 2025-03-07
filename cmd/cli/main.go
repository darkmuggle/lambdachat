package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lambda/lambdachat-slackbot/internal/lambdachat"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var version = "0.0.1~dev"

var rootOptions = struct {
	url    string
	apiKey string
}{}

var ctx, cancel = context.WithCancel(context.Background())

var log = logrus.New().WithContext(ctx).WithField("lambda-chat", "cli")

var rootCmd = &cobra.Command{
	Use:     "action",
	Short:   "action - command for interacting wtih Lambda Chat and Github",
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if rootOptions.apiKey == "" {
			log.Fatal("API Key is required")
		}
		if rootOptions.url == "" {
			log.Fatal("Host is required")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Starting Interactive Chat")

		lc, err := lambdachat.New(log, rootOptions.url, rootOptions.apiKey, lambdachat.DefaultModel, lambdachat.PersonaBender)
		if err != nil {
			log.WithError(err).Fatal("Failed to create Lambda Chat")
		}

		// Use a fixed user ID for CLI interactions
		const cliUserID = "cli-user"

		for {
			fmt.Print("Assistant query: ")
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)

			out, err := lc.Chat(cliUserID, text)
			if err != nil {
				log.WithError(err).Fatal("Failed to chat")
			}
			fmt.Println(out)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootOptions.apiKey, "api-key", "", "secret_vscode1_17e616d542514a3d8c73e1353da71e9b.XXJi0nsdps4gu2zDGr59I2r5HSRwFyvB", "Lambda Chat API Key")
	rootCmd.PersistentFlags().StringVarP(&rootOptions.url, "host", "", "https://api.lambdalabs.com/v1", "Lambda Chat Host")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.WithError(err).Fatal("Failed to execute command")
	}
	cancel()
}
