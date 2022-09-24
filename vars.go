package main

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
	yt "github.com/kkdai/youtube/v2"
)

// Bot Parameters
var (
	botToken        string
	youtubeToken    string
	s               *discordgo.Session
	v               = new(VoiceInstance)
	opts            = dca.StdEncodeOptions
	client          = yt.Client{Debug: true}
	ctx             = context.Background()
	song            = Song{}
	songSearch      = SongSearch{}
	searchQueue     = []SongSearch{}
	queue           = []Song{}
	searchRequested bool
)
