package bot

import (
	"log"

	"ne/discreminder/pkg/db"

	"github.com/bwmarrin/discordgo"
)

func SetupHandlers(dg *discordgo.Session, database *db.DB, scheduler *Scheduler) {
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.Ready) {
		handleReady(s, i, database, scheduler)
	})

	dg.AddHandler(func(s *discordgo.Session, g *discordgo.GuildCreate) {
		handleGuildCreate(s, g, database, scheduler)
	})

}

func handleReady(s *discordgo.Session, i *discordgo.Ready, database *db.DB, scheduler *Scheduler) {
	log.Printf("Bot is ready: %s (%s)", i.User.Username, i.User.ID)

	// Sync guilds from the READY payload to ensure we don't miss any joined while offline
	for _, g := range i.Guilds {
		ensureGuildInitialized(g.ID, database, scheduler)
	}
}

func handleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate, database *db.DB, scheduler *Scheduler) {
	log.Printf("Bot joined/synced guild: %s (%s)", g.Name, g.ID)
	ensureGuildInitialized(g.ID, database, scheduler)
}

func ensureGuildInitialized(guildID string, database *db.DB, scheduler *Scheduler) {
	settings, err := database.GetGuildSettings(guildID)
	if err != nil {
		log.Printf("Error checking guild settings for %s: %v", guildID, err)
		return
	}

	if settings == nil {
		log.Printf("Initializing settings for new guild: %s", guildID)
		settings = &db.GuildSettings{
			GuildID:           guildID,
			ReminderTime:      "18:00",
			Enabled:           true,
			DefaultMainText:   "Hey {mention}! This is your daily reminder for {server_name}!",
			DefaultEmbedText:  "Have a great day!",
			DefaultEmbedImage: "",
		}
		if err := database.UpsertGuildSettings(settings); err != nil {
			log.Printf("Error initializing guild settings for %s: %v", guildID, err)
			return
		}
	}

	// Update scheduler cache for the guild (whether new or existing)
	scheduler.UpdateGuild(*settings)
}
