package events

import "github.com/bwmarrin/discordgo"

// Type defines the category of an application event.
type Type int

const (
	// Disconnect represents a lost connection to Discord.
	Disconnect Type = iota
	// Command represents an incoming Discord slash command.
	Command
	// Shutdown represents a request to terminate the application gracefully.
	Shutdown
)

// AppEvent is a message sent from background workers (like Discord) to the main loop.
type AppEvent struct {
	Type Type
	Data interface{}
}

// NewCommandEvent creates an event for a Discord interaction.
func NewCommandEvent(i *discordgo.InteractionCreate) AppEvent {
	return AppEvent{Type: Command, Data: i}
}

// NewDisconnectEvent creates an event for a Discord disconnection.
func NewDisconnectEvent(d *discordgo.Disconnect) AppEvent {
	return AppEvent{Type: Disconnect, Data: d}
}
