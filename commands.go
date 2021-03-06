package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Get & queue audio in a YouTube video / playlist
func queueSong(m *discordgo.MessageCreate) {
	// Split the message to get YT link
	commData := strings.Split(m.Content, " ")
	queueLenBefore := len(queue)
	if len(commData) == 2 {
		// If playlist.... TODO: Error checking on the link
		if strings.Contains(m.Content, "list") {
			playlistID := strings.Replace(commData[1], "https://www.youtube.com/playlist?list=", "", -1)
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Queueing Your PlayList... :infinity:")
			queuePlaylist(playlistID, m)
		} else {
			// Single video link (not a playlist)
			video, err := client.GetVideo(commData[1])
			if err != nil {
				log.Println(err)
			} else {
				// Get formats with audio channels only
				format := video.Formats.WithAudioChannels()

				// Fill Song Info
				song = fillSongInfo(m.ChannelID, m.Author.ID, m.ID, video.ID, video.Title, video.Duration.String())

				url, err := client.GetStreamURL(video, &format[0])
				if err != nil {
					log.Println(err)
				} else {
					song.VideoURL = url
					queue = append(queue, song)
				}
			}
		}

		// If there's nothing playing and the queue grew
		if v.nowPlaying == (Song{}) && len(queue) >= 1 {

			// Get the channel of the person who made the request
			authorChan := SearchVoiceChannel(m.Author.ID)

			// Join the channel of the person who made the request
			if authorChan != m.ChannelID {
				var err error
				v.voice, err = s.ChannelVoiceJoin(v.guildID, authorChan, false, false)

				if err != nil {
					log.Println("ERROR: Error to join in a voice channel: ", err)
					return
				}

				v.voice.Speaking(false)
				s.ChannelMessageSend(m.ChannelID, "**[Muse]** <@"+m.Author.ID+"> - I've joined your channel!")
			}

			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Playing ["+queue[0].Title+"] :notes:")
			playQueue(m)
		} else {
			// Only display queue if it grew in size...
			if queueLenBefore < len(queue) {
				getQueue(m)
			} else {
				nothingAddedMessage := "**[Muse]** Nothing was added, playlist or song was empty...\n"
				nothingAddedMessage = nothingAddedMessage + "Note:\n"
				nothingAddedMessage = nothingAddedMessage + "- Playlists should have the following url structure: <https://www.youtube.com/playlist?list=><PLAYLIST IDENTIFIER>\n"
				nothingAddedMessage = nothingAddedMessage + "- Videos should have the following url structure: <https://www.youtube.com/watch?v=><VIDEO IDENTIFIER>\n"
				nothingAddedMessage = nothingAddedMessage + "- Youtu.be links or links set at a certain time (t=#s) have not been implemented - sorry!"
				s.ChannelMessageSend(m.ChannelID, nothingAddedMessage)
			}
		}
	}
}

// Stops current song and empties the queue
func stopAll(m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, "**[Muse]** Stopping ["+v.nowPlaying.Title+"] & Clearing Queue :octagonal_sign:")
	v.stop = true

	queue = []Song{}

	if v.encoder != nil {
		v.encoder.Cleanup()
	}

	v.voice.Disconnect()
}

// Skips the current song
func skipSong(m *discordgo.MessageCreate) {
	// Check if a song is playing - If no song, skip this and notify
	var replyMessage string
	if v.nowPlaying == (Song{}) {
		replyMessage = "**[Muse]** Queue is empty - There's nothing to skip!"
	} else {
		replyMessage = fmt.Sprintf("**[Muse]** Skipping [%s] :loop:", v.nowPlaying.Title)
		v.stop = true
		v.speaking = false
		v.encoder.Cleanup()
		log.Println("Skipping " + v.nowPlaying.Title)
		log.Println("Queue Length: ", len(queue)-1)
	}
	s.ChannelMessageSend(m.ChannelID, replyMessage)
}

// Fetches and displays the queue
func getQueue(m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, "**[Muse]** Fetching Queue...")
	queueList := ":musical_note:   QUEUE LIST   :musical_note:\n"

	if v.nowPlaying != (Song{}) {
		queueList = queueList + "Now Playing: " + v.nowPlaying.Title + "  ->  Queued by <@" + v.nowPlaying.User + "> \n"
	}

	for index, element := range queue {
		queueList = queueList + " " + strconv.Itoa(index+1) + ". " + element.Title + "  ->  Queued by <@" + element.User + "> \n"
		if index+1 == 14 {
			s.ChannelMessageSend(m.ChannelID, queueList)
			queueList = ""
		}
	}

	s.ChannelMessageSend(m.ChannelID, queueList)
	log.Println(queueList)
}

// Removes a song from the queue at a specific position
func removeFromQueue(m *discordgo.MessageCreate) {
	// Split the message to get which song to remove from the queue
	commData := strings.Split(m.Content, " ")
	var msgToUser string
	if len(commData) == 2 {
		if queuePos, err := strconv.Atoi(commData[1]); err == nil {
			if queue != nil {
				if 1 <= queuePos && queuePos <= len(queue) {
					queuePos--
					var songTitle = queue[queuePos].Title
					var tmpQueue []Song
					tmpQueue = queue[:queuePos]
					tmpQueue = append(tmpQueue, queue[queuePos+1:]...)
					queue = tmpQueue
					msgToUser = fmt.Sprintf("**[Muse]** Removed %s.", songTitle)
				} else {
					msgToUser = "**[Muse]** The selection was out of range."
				}
			} else {
				msgToUser = "**[Muse]** There is no queue to remove songs from."
			}
		}
		s.ChannelMessageSend(m.ChannelID, msgToUser)
	}
}

// Plays the queue
func playQueue(m *discordgo.MessageCreate) {
	for len(queue) > 0 {
		if len(queue) != 0 {
			v.nowPlaying, queue = queue[0], queue[1:]
		} else {
			v.nowPlaying = Song{}
			return
		}

		v.stop = false
		v.voice.Speaking(true)
		v.DCA(v.nowPlaying.VideoURL)

		// Queue not empty, next song isn't empty (incase nil song in queue)
		if len(queue) != 0 && queue[0].Title != "" {
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Next! Now playing ["+queue[0].Title+"] :loop:")
		}
	}

	// No more songs in the queue, reset the queue + leave channel
	v.stop = true
	v.nowPlaying = Song{}
	s.ChannelMessageSend(m.ChannelID, "**[Muse]** Nothing left to play, peace! :v:")
	v.voice.Disconnect()
}
