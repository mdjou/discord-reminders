package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"ne/discreminder/pkg/db"
	"ne/discreminder/pkg/mydiscord"

	"github.com/bwmarrin/discordgo"
)

type Scheduler struct {
	database *db.DB
	client   *mydiscord.Client
	mu       sync.RWMutex
	guilds   map[string]db.GuildSettings
}

func NewScheduler(database *db.DB, client *mydiscord.Client) *Scheduler {
	return &Scheduler{
		database: database,
		client:   client,
		guilds:   make(map[string]db.GuildSettings),
	}
}

func (s *Scheduler) Start() {
	if err := s.LoadAll(); err != nil {
		log.Printf("Error loading initial guilds: %v", err)
	}

	go func() {
		// Wait until the start of the next minute to align checks
		now := time.Now()
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		log.Printf("Scheduler waiting %v until next minute (%v) to align start", time.Until(nextMinute), nextMinute.Format("15:04:05"))
		time.Sleep(time.Until(nextMinute))

		// Initial check for the current minute
		s.checkReminders(nextMinute)

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for t := range ticker.C {
			s.checkReminders(t)
		}
	}()
}

func (s *Scheduler) LoadAll() error {
	guilds, err := s.database.GetAllGuilds()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range guilds {
		if g.Enabled && g.ChannelID != "" {
			s.guilds[g.GuildID] = g
		} else {
			delete(s.guilds, g.GuildID)
		}
	}
	log.Printf("Loaded %d active reminders into memory", len(s.guilds))
	return nil
}

func (s *Scheduler) UpdateGuild(g db.GuildSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g.Enabled && g.ChannelID != "" {
		s.guilds[g.GuildID] = g
		log.Printf("Updated reminder cache for guild %s", g.GuildID)
	} else {
		delete(s.guilds, g.GuildID)
		log.Printf("Removed guild %s from reminder cache (disabled or no channel)", g.GuildID)
	}
}

func (s *Scheduler) checkReminders(t time.Time) {
	t = t.Round(time.Minute).UTC()
	currentTime := t.Format("15:04")

	s.mu.RLock()
	// Create a snapshot to avoid holding the lock during HTTP requests
	var activeGuilds []db.GuildSettings
	for _, g := range s.guilds {
		if g.ReminderTime == currentTime {
			activeGuilds = append(activeGuilds, g)
		}
	}
	s.mu.RUnlock()

	for _, g := range activeGuilds {
		s.sendReminderForGuild(g)
	}
}

func (s *Scheduler) sendReminderForGuild(g db.GuildSettings) {
	log.Printf("Sending reminder for guild %s", g.GuildID)

	mainText, embedText, embedImage, embedThumbnail, isCustom, err := s.PrepareReminder(g, true)
	if err != nil {
		log.Printf("Error preparing reminder for guild %s: %v", g.GuildID, err)
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

	if err := s.client.SendReminder(g.ChannelID, mainText, embed); err != nil {
		log.Printf("Error sending reminder to channel %s for guild %s: %v", g.ChannelID, g.GuildID, err)
	} else {
		log.Printf("Successfully sent reminder to guild %s (Channel: %s)", g.GuildID, g.ChannelID)
		// Log to DB
		err := s.database.LogSentReminder(&db.SentReminder{
			GuildID:        g.GuildID,
			ChannelID:      g.ChannelID,
			MainText:       mainText,
			EmbedText:      embedText,
			EmbedImage:     embedImage,
			EmbedThumbnail: embedThumbnail,
			IsCustom:       isCustom,
		})
		if err != nil {
			log.Printf("Error logging sent reminder for guild %s: %v", g.GuildID, err)
		}
	}
}

func (s *Scheduler) PrepareReminder(g db.GuildSettings, consume bool) (mainText, embedText, embedImage, embedThumbnail string, isCustom bool, err error) {
	customMsg, err := s.database.GetNextCustomMessage(g.GuildID)
	if err != nil {
		return "", "", "", "", false, err
	}

	isSkip := false

	if customMsg != nil {
		if customMsg.Skip {
			isSkip = true
		} else {
			mainText = customMsg.MainText
			embedText = customMsg.EmbedText
			embedImage = customMsg.EmbedImage
			embedThumbnail = customMsg.EmbedThumbnail
			isCustom = true
		}
		// Always remove the custom message from queue after processing (even if it was a skip)
		if consume {
			if err := s.database.DeleteCustomMessage(customMsg.ID); err != nil {
				log.Printf("Error deleting custom message %d for guild %s: %v", customMsg.ID, g.GuildID, err)
			}
		}
	}

	if isSkip || (customMsg == nil) {
		// Use default
		mainText = g.DefaultMainText
		embedText = g.DefaultEmbedText
		embedImage = g.DefaultEmbedImage
		embedThumbnail = g.DefaultEmbedThumbnail
		isCustom = false
	}

	return s.ReplacePlaceholders(g, mainText, embedText, embedImage, embedThumbnail, isCustom)
}

func (s *Scheduler) ReplacePlaceholders(g db.GuildSettings, mainText, embedText, embedImage, embedThumbnail string, isCustom bool) (string, string, string, string, bool, error) {
	// Placeholders
	guild, err := s.client.Session.Guild(g.GuildID)
	guildName := "Server"
	if err == nil {
		guildName = guild.Name
	}

	roleMention := ""
	if g.PingRoleID != "" {
		roleMention = fmt.Sprintf("<@&%s>", g.PingRoleID)
	}

	mainText = replacePlaceholders(mainText, guildName, roleMention)
	embedText = replacePlaceholders(embedText, guildName, roleMention)

	return mainText, embedText, embedImage, embedThumbnail, isCustom, nil
}

func replacePlaceholders(text, guildName, roleMention string) string {
	text = strings.ReplaceAll(text, "{server_name}", guildName)
	text = strings.ReplaceAll(text, "{mention}", roleMention)
	return text
}
