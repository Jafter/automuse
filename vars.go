package main

import (
	"github.com/bwmarrin/discordgo"
)

// Bot Parameters
var (
	botToken       string
	youtubeToken   string
	voiceChannelID string
	dg             *discordgo.Session
	s              *discordgo.Session
	v              = new(VoiceInstance)
	client         = Client{Debug: true}
	song           = Song{}
	queue          = []Song{}
)
