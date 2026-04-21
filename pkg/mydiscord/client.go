package mydiscord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type Client struct {
	Session *discordgo.Session
}

func NewClient(session *discordgo.Session) *Client {
	return &Client{Session: session}
}

func (c *Client) SendReminder(channelID string, content string, embed *discordgo.MessageEmbed) error {
	if channelID == "" {
		return fmt.Errorf("channel ID is empty")
	}

	params := &discordgo.MessageSend{
		Content: content,
	}
	if embed != nil {
		params.Embeds = []*discordgo.MessageEmbed{embed}
	}

	_, err := c.Session.ChannelMessageSendComplex(channelID, params)
	return err
}

func (c *Client) CreateGuildCommand(guildID string, cmd *discordgo.ApplicationCommand) error {
	_, err := c.Session.ApplicationCommandCreate(c.Session.State.User.ID, guildID, cmd)
	return err
}
