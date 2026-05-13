package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ne/discreminder/pkg/app/bot"
	"ne/discreminder/pkg/app/core"
	"ne/discreminder/pkg/app/discord"
	"ne/discreminder/pkg/connections"
	"ne/discreminder/pkg/db"
	"ne/discreminder/pkg/events"
	dist "ne/discreminder/pkg/mydiscord"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Load Environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	token := os.Getenv("DISCORD_TOKEN")
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "bot.db"
	}

	// 2. Initialize Database
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 3. Initialize Discord Session
	dg, err := connections.NewDiscordSession(token)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	// 4. Initialize Scheduler and Client
	client := dist.NewClient(dg)
	scheduler := bot.NewScheduler(database, client)

	// 5. Initialize Event Channel and Registry
	eventChan := make(chan events.AppEvent, 100)
	reg := discord.NewCommandRegistry()
	reg.RegisterDefaultProcessors()
	bot.RegisterBotCommands(reg, database, scheduler)

	// 6. Setup Handlers
	reg.SetupProcessor(dg, eventChan)
	connections.SetupDisconnectChannel(dg, eventChan)
	bot.SetupHandlers(dg, database, scheduler)

	// 7. Open Connection
	if err := dg.Open(); err != nil {
		log.Fatalf("Failed to open Discord connection: %v", err)
	}
	defer dg.Close()

	// 8. Start Scheduler
	schedCtx, schedCtxCancel := context.WithCancel(context.Background())
	scheduler.Start(schedCtx)

	// 9. Start Main Loop
	log.Println("Bot is now running. Press CTRL-C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("Shutdown syscall received...")
		schedCtxCancel()
		dg.Close()
		eventChan <- events.AppEvent{Type: events.Shutdown}
	}()

	if err := core.RunLoop(dg, reg, eventChan); err != nil {
		log.Fatalf("Main loop error: %v", err)
	}
}
