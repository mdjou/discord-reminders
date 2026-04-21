package db

import (
	"database/sql"
)

type GuildSettings struct {
	GuildID               string
	ChannelID             string
	PingRoleID           string
	ReminderTime         string // HH:MM
	Enabled              bool
	DefaultMainText      string
	DefaultEmbedText     string
	DefaultEmbedImage    string
	DefaultEmbedThumbnail string
}

type CustomMessage struct {
	ID             int64
	GuildID        string
	MainText       string
	EmbedText      string
	EmbedImage     string
	EmbedThumbnail string
	Skip           bool
	Position       int
}

type SentReminder struct {
	ID             int64
	GuildID        string
	ChannelID      string
	SentAt         string
	MainText       string
	EmbedText      string
	EmbedImage     string
	EmbedThumbnail string
	IsCustom       bool
}

func (db *DB) GetGuildSettings(guildID string) (*GuildSettings, error) {
	row := db.QueryRow(`SELECT guild_id, channel_id, ping_role_id, reminder_time, enabled, default_main_text, default_embed_text, default_embed_image, default_embed_thumbnail 
		FROM guild_settings WHERE guild_id = ?`, guildID)

	var s GuildSettings
	err := row.Scan(&s.GuildID, &s.ChannelID, &s.PingRoleID, &s.ReminderTime, &s.Enabled, &s.DefaultMainText, &s.DefaultEmbedText, &s.DefaultEmbedImage, &s.DefaultEmbedThumbnail)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

func (db *DB) UpsertGuildSettings(s *GuildSettings) error {
	_, err := db.Exec(`INSERT INTO guild_settings (guild_id, channel_id, ping_role_id, reminder_time, enabled, default_main_text, default_embed_text, default_embed_image, default_embed_thumbnail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(guild_id) DO UPDATE SET
			channel_id = excluded.channel_id,
			ping_role_id = excluded.ping_role_id,
			reminder_time = excluded.reminder_time,
			enabled = excluded.enabled,
			default_main_text = excluded.default_main_text,
			default_embed_text = excluded.default_embed_text,
			default_embed_image = excluded.default_embed_image,
			default_embed_thumbnail = excluded.default_embed_thumbnail`,
		s.GuildID, s.ChannelID, s.PingRoleID, s.ReminderTime, s.Enabled, s.DefaultMainText, s.DefaultEmbedText, s.DefaultEmbedImage, s.DefaultEmbedThumbnail)
	return err
}

func (db *DB) GetAllGuilds() ([]GuildSettings, error) {
	rows, err := db.Query(`SELECT guild_id, channel_id, ping_role_id, reminder_time, enabled, default_main_text, default_embed_text, default_embed_image, default_embed_thumbnail FROM guild_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guilds []GuildSettings
	for rows.Next() {
		var s GuildSettings
		if err := rows.Scan(&s.GuildID, &s.ChannelID, &s.PingRoleID, &s.ReminderTime, &s.Enabled, &s.DefaultMainText, &s.DefaultEmbedText, &s.DefaultEmbedImage, &s.DefaultEmbedThumbnail); err != nil {
			return nil, err
		}
		guilds = append(guilds, s)
	}
	return guilds, nil
}

func (db *DB) GetNextCustomMessage(guildID string) (*CustomMessage, error) {
	row := db.QueryRow(`SELECT id, guild_id, main_text, embed_text, embed_image, embed_thumbnail, skip, position 
		FROM custom_messages WHERE guild_id = ? ORDER BY position ASC LIMIT 1`, guildID)

	var m CustomMessage
	err := row.Scan(&m.ID, &m.GuildID, &m.MainText, &m.EmbedText, &m.EmbedImage, &m.EmbedThumbnail, &m.Skip, &m.Position)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (db *DB) DeleteCustomMessage(id int64) error {
	_, err := db.Exec(`DELETE FROM custom_messages WHERE id = ?`, id)
	return err
}

func (db *DB) AddCustomMessage(m *CustomMessage) error {
	row := db.QueryRow(`SELECT COALESCE(MAX(position), 0) + 1 FROM custom_messages WHERE guild_id = ?`, m.GuildID)
	var pos int
	if err := row.Scan(&pos); err != nil {
		return err
	}

	_, err := db.Exec(`INSERT INTO custom_messages (guild_id, main_text, embed_text, embed_image, embed_thumbnail, skip, position)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, m.GuildID, m.MainText, m.EmbedText, m.EmbedImage, m.EmbedThumbnail, m.Skip, pos)
	return err
}

func (db *DB) GetQueue(guildID string) ([]CustomMessage, error) {
	rows, err := db.Query(`SELECT id, guild_id, main_text, embed_text, embed_image, embed_thumbnail, skip, position 
		FROM custom_messages WHERE guild_id = ? ORDER BY position ASC`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queue []CustomMessage
	for rows.Next() {
		var m CustomMessage
		if err := rows.Scan(&m.ID, &m.GuildID, &m.MainText, &m.EmbedText, &m.EmbedImage, &m.EmbedThumbnail, &m.Skip, &m.Position); err != nil {
			return nil, err
		}
		queue = append(queue, m)
	}
	return queue, nil
}

func (db *DB) LogSentReminder(r *SentReminder) error {
	_, err := db.Exec(`INSERT INTO sent_reminders (guild_id, channel_id, main_text, embed_text, embed_image, embed_thumbnail, is_custom)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, r.GuildID, r.ChannelID, r.MainText, r.EmbedText, r.EmbedImage, r.EmbedThumbnail, r.IsCustom)
	return err
}
