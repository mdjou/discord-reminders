package core

import (
	"fmt"
	"log"

	"ne/discreminder/pkg/app/discord"
	"ne/discreminder/pkg/events"

	"github.com/bwmarrin/discordgo"
)

// RunLoop starts the main application loop. It processes events sent from background
// worker threads (like Discord handlers) and executes them in the main thread.
func RunLoop(dg *discordgo.Session, reg *discord.CommandRegistry, eventChan <-chan events.AppEvent) error {
	log.Println("Starting application main loop...")

	for {
		event, ok := <-eventChan
		if !ok {
			return fmt.Errorf("event channel closed")
		}

		switch event.Type {
		case events.Disconnect:
			log.Printf("Received disconnect signal..")
			// do nothing?

		case events.Shutdown:
			log.Println("Main loop received shutdown signal. Exiting...")
			return nil

		case events.Command:
			// Actual command processing is performed here, freeing up the Discord goroutine.
			i, ok := event.Data.(*discordgo.InteractionCreate)
			if !ok {
				log.Println("Main loop received malformed command data")
				continue
			}

			var name string
			if i.Type == discordgo.InteractionApplicationCommand {
				name = i.ApplicationCommandData().Name
			} else if i.Type == discordgo.InteractionModalSubmit {
				name = i.ModalSubmitData().CustomID
			}

			log.Printf("Main thread processing interaction: %s (Type: %v)", name, i.Type)
			reg.Handle(dg, i)
		}
	}
}
