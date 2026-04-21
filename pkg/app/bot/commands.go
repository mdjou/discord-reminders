package bot

import (
	"fmt"
	"log"
	"strings"

	"ne/discreminder/pkg/app/discord"
	"ne/discreminder/pkg/db"

	"github.com/bwmarrin/discordgo"
)

func RegisterBotCommands(reg *discord.CommandRegistry, database *db.DB, scheduler *Scheduler) {
	reg.Register("config", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("User %s (%s) used /config in guild %s", i.Member.User.Username, i.Member.User.ID, i.GuildID)
		handleConfig(s, i, database, scheduler)
	})
	reg.Register("config-default", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("User %s (%s) used /config-default in guild %s", i.Member.User.Username, i.Member.User.ID, i.GuildID)
		handleConfigDefault(s, i, database)
	})
	reg.Register("config-default-modal", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleConfigDefaultModal(s, i, database, scheduler)
	})
	reg.Register("queue-view", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("User %s (%s) used /queue-view in guild %s", i.Member.User.Username, i.Member.User.ID, i.GuildID)
		handleQueueView(s, i, database)
	})
	reg.Register("queue-add", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("User %s (%s) used /queue-add in guild %s", i.Member.User.Username, i.Member.User.ID, i.GuildID)
		handleQueueAdd(s, i, database)
	})
	reg.Register("queue-add-modal", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		handleQueueAddModal(s, i, database)
	})
	reg.Register("queue-delete", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("User %s (%s) used /queue-delete in guild %s", i.Member.User.Username, i.Member.User.ID, i.GuildID)
		handleQueueDelete(s, i, database)
	})
	reg.Register("show", func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("User %s (%s) used /show in guild %s", i.Member.User.Username, i.Member.User.ID, i.GuildID)
		handleShow(s, i, database, scheduler)
	})
}

func GetCommandSpecs() []*discordgo.ApplicationCommand {
	managePermission := int64(discordgo.PermissionManageGuild)
	dmPermission := false

	return []*discordgo.ApplicationCommand{
		{
			Name:                     "config",
			Description:              "Configure or view reminder settings",
			DefaultMemberPermissions: &managePermission,
			DMPermission:             &dmPermission,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "time",
					Description: "Reminder time (HH:MM UTC)",
				},
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "channel",
					Description: "Channel for reminders",
				},
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "ping-role",
					Description: "Role to ping in reminders",
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "enabled",
					Description: "Enable or disable reminders",
				},
			},
		},
		{
			Name:                     "config-default",
			Description:              "Configure default reminder settings (opens modal)",
			DefaultMemberPermissions: &managePermission,
			DMPermission:             &dmPermission,
		},
		{
			Name:                     "queue-view",
			Description:              "View the custom messages queue",
			DefaultMemberPermissions: &managePermission,
			DMPermission:             &dmPermission,
		},
		{
			Name:                     "queue-add",
			Description:              "Add a message to the queue (opens composition modal)",
			DefaultMemberPermissions: &managePermission,
			DMPermission:             &dmPermission,
		},
		{
			Name:                     "queue-delete",
			Description:              "Delete a message from the queue",
			DefaultMemberPermissions: &managePermission,
			DMPermission:             &dmPermission,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "position",
					Description: "Position in queue (1-based index)",
					Required:    true,
				},
			},
		},
		{
			Name:                     "show",
			Description:              "Show a reminder message in the current channel (for testing)",
			DefaultMemberPermissions: &managePermission,
			DMPermission:             &dmPermission,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "position",
					Description: "Position in queue to test (optional, defaults to server default)",
					Required:    false,
				},
			},
		},
	}
}

func checkPermissions(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB) bool {
	// Admin permission
	if i.Member.Permissions&discordgo.PermissionAdministrator != 0 {
		return true
	}

	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		guild, err = s.Guild(i.GuildID)
	}
	if err == nil && guild.OwnerID == i.Member.User.ID {
		return true
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "You don't have permission to use this command.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	return false
}

func handleConfig(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB, scheduler *Scheduler) {
	if !checkPermissions(s, i, database) {
		return
	}

	options := i.ApplicationCommandData().Options
	settings, _ := database.GetGuildSettings(i.GuildID)
	if settings == nil {
		settings = &db.GuildSettings{
			GuildID:         i.GuildID,
			ReminderTime:    "18:00",
			Enabled:         true,
			DefaultMainText: "Hey {mention}! This is your daily reminder for {server_name}!",
		}
	}

	for _, opt := range options {
		switch opt.Name {
		case "time":
			settings.ReminderTime = opt.StringValue()
		case "channel":
			settings.ChannelID = opt.ChannelValue(s).ID
		case "ping-role":
			settings.PingRoleID = opt.RoleValue(s, i.GuildID).ID
		case "enabled":
			settings.Enabled = opt.BoolValue()
		}
	}

	if err := database.UpsertGuildSettings(settings); err != nil {
		log.Printf("Error updating settings for guild %s: %v", i.GuildID, err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Failed to update settings."},
		})
		return
	}

	// Update scheduler cache
	scheduler.UpdateGuild(*settings)

	// Format current config for display
	status := "Enabled"
	if !settings.Enabled {
		status = "Disabled"
	}

	channelMention := "Not set"
	if settings.ChannelID != "" {
		channelMention = fmt.Sprintf("<#%s>", settings.ChannelID)
	}

	pingRoleMention := "None"
	if settings.PingRoleID != "" {
		pingRoleMention = fmt.Sprintf("<@&%s>", settings.PingRoleID)
	}

	embedImage := "None"
	if settings.DefaultEmbedImage != "" {
		embedImage = "Set"
	}

	embedThumbnail := "None"
	if settings.DefaultEmbedThumbnail != "" {
		embedThumbnail = "Set"
	}

	summary := fmt.Sprintf("**Current Configuration:**\n"+
		"• **Status:** %s\n"+
		"• **Time:** %s UTC\n"+
		"• **Channel:** %s\n"+
		"• **Ping Role:** %s\n"+
		"• **Default Image:** %s\n"+
		"• **Default Thumbnail:** %s",
		status, settings.ReminderTime, channelMention, pingRoleMention, embedImage, embedThumbnail)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: summary,
		},
	})
}

func handleQueueView(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB) {
	if !checkPermissions(s, i, database) {
		return
	}

	queue, err := database.GetQueue(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Error fetching queue."},
		})
		return
	}

	if len(queue) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Queue is empty."},
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("**Custom Messages Queue:**\n")
	for i, m := range queue {
		typeStr := "Custom"
		if m.Skip {
			typeStr = "SKIP (Fallbacks to default)"
		}

		preview := formatQueuePreview(m.MainText, m.EmbedText)
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, typeStr, preview))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: sb.String()},
	})
}

func handleConfigDefault(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB) {
	if !checkPermissions(s, i, database) {
		return
	}

	settings, _ := database.GetGuildSettings(i.GuildID)
	defaultMain := ""
	defaultEmbed := ""
	defaultImage := ""
	defaultThumbnail := ""
	if settings != nil {
		defaultMain = settings.DefaultMainText
		defaultEmbed = settings.DefaultEmbedText
		defaultImage = settings.DefaultEmbedImage
		defaultThumbnail = settings.DefaultEmbedThumbnail
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "config-default-modal",
			Title:    "Default Reminder Settings",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "main-text",
							Label:       "Main Text (supports {mention})",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "e.g. Hey {mention}! Time for the daily in {server_name}!",
							Value:       defaultMain,
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "embed-text",
							Label:       "Embed (supports {mention})",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Enter the default embed description here...",
							Value:       defaultEmbed,
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "embed-image",
							Label:       "Embed Image URL",
							Style:       discordgo.TextInputShort,
							Placeholder: "https://...",
							Value:       defaultImage,
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "embed-thumbnail",
							Label:       "Embed Thumbnail URL",
							Style:       discordgo.TextInputShort,
							Placeholder: "https://...",
							Value:       defaultThumbnail,
							Required:    false,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Printf("Error sending config-default modal: %v", err)
	}
}

func handleConfigDefaultModal(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB, scheduler *Scheduler) {
	if !checkPermissions(s, i, database) {
		return
	}

	data := i.ModalSubmitData()
	settings, _ := database.GetGuildSettings(i.GuildID)
	if settings == nil {
		settings = &db.GuildSettings{GuildID: i.GuildID, ReminderTime: "18:00", Enabled: true}
	}

	for _, row := range data.Components {
		for _, comp := range row.(*discordgo.ActionsRow).Components {
			input := comp.(*discordgo.TextInput)
			switch input.CustomID {
			case "main-text":
				settings.DefaultMainText = input.Value
			case "embed-text":
				settings.DefaultEmbedText = input.Value
			case "embed-image":
				settings.DefaultEmbedImage = input.Value
			case "embed-thumbnail":
				settings.DefaultEmbedThumbnail = input.Value
			}
		}
	}

	if err := database.UpsertGuildSettings(settings); err != nil {
		log.Printf("Error saving default settings: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to save settings.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	scheduler.UpdateGuild(*settings)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Default settings updated successfully!",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func handleQueueAdd(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB) {
	if !checkPermissions(s, i, database) {
		return
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "queue-add-modal",
			Title:    "Compose Custom Reminder",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "main-text",
							Label:       "Main Text (Optional, supports {mention})",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Enter the main message text here...",
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "embed-text",
							Label:       "Embed Text (Optional, supports {mention})",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Enter the embed description here...",
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "embed-image",
							Label:       "Embed Image URL (Optional)",
							Style:       discordgo.TextInputShort,
							Placeholder: "https://...",
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "embed-thumbnail",
							Label:       "Embed Thumbnail URL (Optional)",
							Style:       discordgo.TextInputShort,
							Placeholder: "https://...",
							Required:    false,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "skip",
							Label:       "Skip this reminder? (Type 'Y' to skip)",
							Style:       discordgo.TextInputShort,
							Placeholder: "N",
							Required:    false,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Printf("Error sending modal: %v", err)
	}
}

func handleQueueAddModal(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB) {
	if !checkPermissions(s, i, database) {
		return
	}

	data := i.ModalSubmitData()
	msg := db.CustomMessage{GuildID: i.GuildID}

	for _, row := range data.Components {
		for _, comp := range row.(*discordgo.ActionsRow).Components {
			input := comp.(*discordgo.TextInput)
			switch input.CustomID {
			case "main-text":
				msg.MainText = input.Value
			case "embed-text":
				msg.EmbedText = input.Value
			case "embed-image":
				msg.EmbedImage = input.Value
			case "embed-thumbnail":
				msg.EmbedThumbnail = input.Value
			case "skip":
				val := strings.ToUpper(strings.TrimSpace(input.Value))
				if val == "YES" || val == "Y" || val == "TRUE" || val == "1" {
					msg.Skip = true
				}
			}
		}
	}

	if err := database.AddCustomMessage(&msg); err != nil {
		log.Printf("Error adding custom message to queue: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to add to queue.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Added to queue successfully!",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func handleQueueDelete(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB) {
	if !checkPermissions(s, i, database) {
		return
	}

	pos := i.ApplicationCommandData().Options[0].IntValue()
	queue, _ := database.GetQueue(i.GuildID)
	if pos < 1 || int(pos) > len(queue) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Invalid position."},
		})
		return
	}

	msgToDelete := queue[pos-1]
	if err := database.DeleteCustomMessage(msgToDelete.ID); err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Failed to delete message."},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: "Deleted from queue!"},
	})
}

func handleShow(s *discordgo.Session, i *discordgo.InteractionCreate, database *db.DB, scheduler *Scheduler) {
	if !checkPermissions(s, i, database) {
		return
	}

	settings, _ := database.GetGuildSettings(i.GuildID)
	if settings == nil {
		settings = &db.GuildSettings{
			GuildID:         i.GuildID,
			DefaultMainText: "Hey {mention}! This is your daily reminder for {server_name}!",
			Enabled:         true,
		}
	}

	options := i.ApplicationCommandData().Options
	var position int64
	hasPosition := false
	for _, opt := range options {
		if opt.Name == "position" {
			position = opt.IntValue()
			hasPosition = true
		}
	}

	var mText, eText, iUrl, tUrl string
	var isCustom bool

	if hasPosition {
		queue, err := database.GetQueue(i.GuildID)
		if err != nil || position < 1 || int(position) > len(queue) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{Content: "Invalid queue position."},
			})
			return
		}
		msg := queue[position-1]
		if msg.Skip {
			mText, eText, iUrl, tUrl = settings.DefaultMainText, settings.DefaultEmbedText, settings.DefaultEmbedImage, settings.DefaultEmbedThumbnail
			isCustom = false
		} else {
			mText, eText, iUrl, tUrl = msg.MainText, msg.EmbedText, msg.EmbedImage, msg.EmbedThumbnail
			isCustom = true
		}
	} else {
		// Use default as requested
		mText, eText, iUrl, tUrl = settings.DefaultMainText, settings.DefaultEmbedText, settings.DefaultEmbedImage, settings.DefaultEmbedThumbnail
		isCustom = false
	}

	mainText, embedText, embedImage, embedThumbnail, _, err := scheduler.ReplacePlaceholders(*settings, mText, eText, iUrl, tUrl, isCustom)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Error preparing reminder."},
		})
		return
	}

	var embed *discordgo.MessageEmbed
	if embedText != "" || embedImage != "" || embedThumbnail != "" {
		embed = &discordgo.MessageEmbed{
			Description: embedText,
		}
		if embedImage != "" {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: embedImage,
			}
		}
		if embedThumbnail != "" {
			embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
				URL: embedThumbnail,
			}
		}
	}

	respData := &discordgo.InteractionResponseData{
		Content: mainText,
	}
	if embed != nil {
		respData.Embeds = []*discordgo.MessageEmbed{embed}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: respData,
	})
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func formatQueuePreview(main, embed string) string {
	main = strings.ReplaceAll(main, "\n", "⏎")
	embed = strings.ReplaceAll(embed, "\n", "⏎")

	if main != "" && embed != "" {
		return fmt.Sprintf("%s | %s", truncate(main, 20), truncate(embed, 20))
	}
	if main != "" {
		return fmt.Sprintf("%s | ", truncate(main, 40))
	}
	if embed != "" {
		return fmt.Sprintf(" | %s", truncate(embed, 40))
	}
	return " | "
}
