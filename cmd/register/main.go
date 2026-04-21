package main

import (
	"log"
	"os"

	"ne/discreminder/pkg/app/bot"
	"ne/discreminder/pkg/connections"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}

	dg, err := connections.NewDiscordSession(token)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	if err := dg.Open(); err != nil {
		log.Fatalf("Failed to open Discord connection: %v", err)
	}
	defer dg.Close()

	log.Println("Registering slash commands globally...")
	specs := bot.GetCommandSpecs()

	// Create global commands
	_, err = dg.ApplicationCommandBulkOverwrite(dg.State.User.ID, "", specs)
	if err != nil {
		log.Fatalf("Failed to bulk overwrite commands: %v", err)
	}

	log.Println("Successfully registered commands. Note: Global commands can take up to an hour to propagate.")
}
