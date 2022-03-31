package main

import (
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Initialize Discord & Setup Youtube
func init() {
	var err error
	botToken = os.Getenv("BOT_TOKEN") // Set your discord bot token as an environment variable.
	youtubeToken = os.Getenv("YT_TOKEN")
	s, err = discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
	v.stop = true // Used to check if the bot is in channel playing music.
}

func main() {
	// Add function handlers to trigger commands from discord chat
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) { log.Println("AutoMuse is Running!") })
	s.AddHandler(executionHandler)

	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}
	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Graceful shutdown")
}

func executionHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Avoid handling the message that the bot creates when replying to a user
	if m.Author.Bot {
		return
	}

	// Setup Channel Information
	guildID := SearchGuild(m.ChannelID)
	channel := false

	// Check if the request was made from a person in the same channel the bot is currently in
	if voiceChannelID == SearchVoiceChannel(m.Author.ID) {
		channel = true
	}

	voiceChannelID = SearchVoiceChannel(m.Author.ID)
	v.guildID = guildID
	v.session = s

	// Commands
	if m.Content != "" {
		if strings.Contains(m.Content, "play") && strings.Contains(m.Content, "youtube") && !strings.Contains(m.Content, "list") {
			go queueSong(m.Content, m, v, voiceChannelID, channel)
		}

		if strings.Contains(m.Content, "play") && strings.Contains(m.Content, "youtube") && strings.Contains(m.Content, "list") {
			go queuePlaylist(m.Content, m, v, voiceChannelID, channel)
		}

		if m.Content == "stop" {
			go stopSong(m.Content, m, v, voiceChannelID)
		}

		if m.Content == "skip" {
			go skipSong(m.Content, m, v, voiceChannelID)
		}

		if m.Content == "queue" {
			go getQueue(m)
		}
	} else {
		return
	}
}
