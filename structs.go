package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

type Options struct {
	DiscordToken      string
	DiscordStatus     string
	DiscordPrefix     string
	DiscordPurgeTime  int64
	DiscordPlayStatus bool
	YoutubeToken      string
}

type Song struct {
	ChannelID string
	User      string
	ID        string
	VidID     string
	Title     string
	Duration  string
	VideoURL  string
}

type SongSearch struct {
	Id   string
	Name string
}

type VoiceInstance struct {
	voice      *discordgo.VoiceConnection
	session    *discordgo.Session
	encoder    *dca.EncodeSession
	stream     *dca.StreamingSession
	nowPlaying Song
	guildID    string
	speaking   bool
	stop       bool
}

type BadQualitySongNodes struct {
	BadQualitySongNodes []BadQualitySong `json:"songs"`
	BadQualityVids      []BadQualityVids `json:"vids"`
}

type BadQualitySong struct {
	Title    string `json:"title"`
	FormatNo int    `json:"formatNo"`
}

type BadQualityVids struct {
	Author   string `json:"author"`
	FormatNo int    `json:"formatNo"`
}
