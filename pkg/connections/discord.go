package connections

import (
	"fmt"

	"ne/discreminder/pkg/events"

	"github.com/bwmarrin/discordgo"
)

// NewDiscordSession creates a non-started Discord session from the provided token.
// It configures the session to be single-threaded (SyncEvents = true) to prevent
// race conditions in event handlers.
func NewDiscordSession(token string) (*discordgo.Session, error) {
	if token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Ensure internal event handling is single-threaded to prevent race conditions.
	dg.SyncEvents = true

	// dg.Identify.Intents = dg.Identify.Intents | discordgo.IntentGuildMembers | discordgo.IntentMessageContent

	return dg, nil
}

// SetupDisconnectChannel configures the session to send disconnect events to the provided channel.
func SetupDisconnectChannel(dg *discordgo.Session, eventChan chan<- events.AppEvent) {
	dg.AddHandler(func(s *discordgo.Session, d *discordgo.Disconnect) {
		eventChan <- events.NewDisconnectEvent(d)
	})
}
