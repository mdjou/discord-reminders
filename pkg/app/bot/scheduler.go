package bot

import (
	"context"
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
	database   *db.DB
	client     *mydiscord.Client
	mu         sync.RWMutex
	guilds     map[string]db.GuildSettings
	updateChan chan struct{}
}

func NewScheduler(database *db.DB, client *mydiscord.Client) *Scheduler {
	return &Scheduler{
		database:   database,
		client:     client,
		guilds:     make(map[string]db.GuildSettings),
		updateChan: make(chan struct{}, 1),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	if err := s.LoadAll(); err != nil {
		log.Printf("Error loading initial guilds: %v", err)
	}

	go func() {
		// Wait until the start of the next minute to align checks
		initialNow := time.Now().UTC()
		nextMinute := initialNow.Add(500 * time.Millisecond).Truncate(time.Minute).Add(time.Minute)
		log.Printf("Scheduler waiting %v until next minute (%v) to align start", time.Until(nextMinute), nextMinute.Format("15:04:05"))
		time.Sleep(time.Until(nextMinute))

		// Initial check for the current minute
		s.checkReminders(nextMinute)
		lastProcessedMinute := nextMinute

		for {
			nextEvent := s.getNextReminderTime(lastProcessedMinute)
			sleepDuration := time.Until(nextEvent)
			log.Printf("Next event: %v; sleep duration: %v", nextEvent, sleepDuration)

			if sleepDuration <= 0 {
				sleepDuration = time.Nanosecond
			}

			timer := time.NewTimer(sleepDuration)
			var firedTime time.Time

		waitLoop:
			for {
				select {
				case firedTime = <-timer.C:
					if !firedTime.IsZero() {
						firedRndTime := firedTime.Round(time.Minute).UTC()
						s.checkReminders(firedRndTime)
						lastProcessedMinute = firedRndTime
					}
					break waitLoop
				case <-s.updateChan:
					// If the timer has more than 1 minute left, interrupt it to recalculate
					if time.Until(nextEvent) > time.Minute {
						timer.Stop()
						break waitLoop
					}
					// If <= 1m left, do nothing and wait for it to fire naturally
				case <-ctx.Done():
					timer.Stop()
					return
				}
			}

		}
	}()
}

func (s *Scheduler) getNextReminderTime(lastProcessed time.Time) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nextTime time.Time
	first := true

	for _, g := range s.guilds {
		parsedTime, err := time.Parse("15:04", g.ReminderTime)
		if err != nil {
			log.Printf("error parsing reminder time for guild %s: %v", g.GuildID, err)
			continue
		}

		t := time.Date(lastProcessed.Year(), lastProcessed.Month(), lastProcessed.Day(), parsedTime.Hour(), parsedTime.Minute(), 0, 0, time.UTC)

		if t.Before(lastProcessed) || t.Equal(lastProcessed) {
			t = t.Add(24 * time.Hour)
		}

		if first || t.Before(nextTime) {
			nextTime = t
			first = false
		}
	}

	if first {
		// No active reminders, sleep for 24 hours
		return lastProcessed.Add(24 * time.Hour).Truncate(time.Minute)
	}

	return nextTime
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
	if g.Enabled && g.ChannelID != "" {
		s.guilds[g.GuildID] = g
		log.Printf("Updated reminder cache for guild %s", g.GuildID)
	} else {
		delete(s.guilds, g.GuildID)
		log.Printf("Removed guild %s from reminder cache (disabled or no channel)", g.GuildID)
	}
	s.mu.Unlock()

	select {
	case s.updateChan <- struct{}{}:
	default:
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
		log.Printf("Successfully sent reminder to guild %s (Channel: %s) at %v", g.GuildID, g.ChannelID, time.Now().UTC())
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
