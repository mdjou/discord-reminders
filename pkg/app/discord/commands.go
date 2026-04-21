package discord

import (
	"log"

	"ne/discreminder/pkg/events"

	"github.com/bwmarrin/discordgo"
)

// CommandHandler defines the function signature for processing a Discord slash command.
type CommandHandler func(s *discordgo.Session, i *discordgo.InteractionCreate)

// CommandRegistry manages the mapping between command names and their respective processors.
type CommandRegistry struct {
	handlers map[string]CommandHandler
}

// NewCommandRegistry initializes a new CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		handlers: make(map[string]CommandHandler),
	}
}

// Register adds a new command processor to the registry.
func (r *CommandRegistry) Register(name string, handler CommandHandler) {
	r.handlers[name] = handler
}

// SetupProcessor attaches the command processing logic to the Discord session.
// Instead of processing immediately, it sends events to the provided channel to free up
// the Discord thread as quickly as possible.
func (r *CommandRegistry) SetupProcessor(dg *discordgo.Session, eventChan chan<- events.AppEvent) {
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Handle both slash commands and modal submissions.
		if i.Type != discordgo.InteractionApplicationCommand && i.Type != discordgo.InteractionModalSubmit {
			return
		}

		// Enqueue the interaction for the main loop to process.
		eventChan <- events.NewCommandEvent(i)
	})
}

// Handle dispatches an interaction to its registered processor.
// This should be called from the main application loop.
func (r *CommandRegistry) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var name string
	if i.Type == discordgo.InteractionApplicationCommand {
		name = i.ApplicationCommandData().Name
	} else if i.Type == discordgo.InteractionModalSubmit {
		name = i.ModalSubmitData().CustomID
	}

	if handler, ok := r.handlers[name]; ok {
		handler(s, i)
	} else {
		log.Printf("No processor registered for interaction: %s (Type: %v)", name, i.Type)
	}
}

// BasicProcessors returns a set of common processors, like a ping command.
// This can be used as a starting point or for testing.
func (r *CommandRegistry) RegisterDefaultProcessors() {
	r.Register("ping", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong!",
			},
		})
		if err != nil {
			log.Printf("Error responding to ping: %v", err)
		}
	})
}
