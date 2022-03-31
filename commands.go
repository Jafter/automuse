package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

// Get & Queue all videos in a YouTube Playlist
func queuePlaylist(message string, m *discordgo.MessageCreate, v *VoiceInstance, channelID string, alreadyInChannel bool) {

	// Split the message to get YT link
	commData := strings.Split(message, " ")

	if len(commData) == 2 {

		// Get the PlayList ID
		playlistID := strings.Replace(commData[1], "https://www.youtube.com/playlist?list=", "", -1)

		ytClient := &http.Client{
			Transport: &transport.APIKey{Key: youtubeToken},
		}

		service, err := youtube.New(ytClient)
		if err != nil {
			log.Fatalf("Error creating new YouTube client: %v", err)
		}

		nextPageToken := ""
		for {
			// Retrieve next set of items in the playlist.
			var snippet = []string{"snippet"}
			playlistResponse := playlistItemsList(service, snippet, playlistID, nextPageToken)

			for _, playlistItem := range playlistResponse.Items {
				videoId := playlistItem.Snippet.ResourceId.VideoId
				log.Println("VideoID: " + videoId)
				content := "https://www.youtube.com/watch?v=" + videoId

				// Get Video Data
				video, err := client.GetVideo(content)
				if err != nil {
					log.Println(err)
				} else {

					format := video.Formats.FindByQuality("medium") //TODO: Check if lower quality affects music quality

					// Fill Song Info
					song = Song{
						ChannelID: m.ChannelID,
						User:      m.Author.ID,
						ID:        m.ID,
						VidID:     video.ID,
						Title:     video.Title,
						Duration:  video.Duration.String(),
						VideoURL:  "",
					}

					url, err := client.GetStreamURL(video, format)
					if err != nil {
						log.Println(err)
						return
					} else {
						song.VideoURL = url
						queue = append(queue, song)
					}
				}
			}

			// Set the token to retrieve the next page of results
			// or exit the loop if all results have been retrieved.
			nextPageToken = playlistResponse.NextPageToken
			if nextPageToken == "" {
				break
			}
		}

		if v.nowPlaying == (Song{}) {
			log.Println("Next song was empty - playing songs")
			v.voice, err = s.ChannelVoiceJoin(v.guildID, channelID, false, false)

			if err != nil {
				log.Println("ERROR: Error to join in a voice channel: ", err)
				return
			}

			// Bot joins caller's channel if it's not in it yet.
			if !alreadyInChannel {
				v.voice.Speaking(false)
				s.ChannelMessageSend(m.ChannelID, "**[Muse]** <@"+m.Author.ID+"> - I've joined your channel!")
			}
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Queing Your PlayList... :infinity:")
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Playing ["+queue[0].Title+"] :notes:")
			playQueue(m)
		} else {
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Queued a PlayList... :infinity:")
			getQueue(m)
			log.Println("Next song was not empty - song was queued - Playing: ", v.nowPlaying.Title)
		}
	}
}

// Play Youtube Music in Channel
// Note: User must be in a voice channel for the bot to access said channel
func queueSong(message string, m *discordgo.MessageCreate, v *VoiceInstance, channelId string, alreadyInChannel bool) {

	// Split the message to get YT link
	commData := strings.Split(message, " ")

	if len(commData) == 2 {
		var err error

		v.voice, err = s.ChannelVoiceJoin(v.guildID, channelId, false, false)

		if err != nil {
			log.Println("ERROR: Error to join in a voice channel: ", err)
			return
		}

		// Bot joins caller's channel if it's not in it yet.
		if !alreadyInChannel {
			v.voice.Speaking(false)
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** <@"+m.Author.ID+"> - I've joined your channel!")
		}

		// Get Video Data
		video, err := client.GetVideo(commData[1])
		if err != nil {
			log.Println(err)
		} else {

			format := video.Formats.FindByQuality("medium")

			// Fill Song Info
			song = Song{
				ChannelID: m.ChannelID,
				User:      m.Author.ID,
				ID:        m.ID,
				VidID:     video.ID,
				Title:     video.Title,
				Duration:  video.Duration.String(),
				VideoURL:  "",
			}

			// Message to play or queue a song - v.stop used to see if a song is currently playing.
			if v.stop {
				s.ChannelMessageSend(m.ChannelID, "**[Muse]** Playing ["+song.Title+"] :notes:")
			} else {
				s.ChannelMessageSend(m.ChannelID, "**[Muse]** Queued ["+song.Title+"] :infinity:")
			}

			url, err := client.GetStreamURL(video, format)
			if err != nil {
				log.Println(err)
			} else {
				song.VideoURL = url
				queue = append(queue, song)

				// Check if a song is already playing, if not start playing the queue
				if v.nowPlaying == (Song{}) {
					log.Println("Next song was empty - playing songs")
					playQueue(m)
				} else {
					log.Println("Next song was not empty - song was queued - Playing: ", v.nowPlaying.Title)
				}
			}
		}
	}
}

func stopSong(message string, m *discordgo.MessageCreate, v *VoiceInstance, channelId string) {
	s.ChannelMessageSend(m.ChannelID, "**[Muse]** Stopping ["+v.nowPlaying.Title+"] & Clearing Queue :octagonal_sign:")
	v.stop = true

	queue = []Song{}

	if v.encoder != nil {
		v.encoder.Cleanup()
	}

	v.voice.Disconnect()
}

func skipSong(message string, m *discordgo.MessageCreate, v *VoiceInstance, channelId string) {
	// Check if a song is playing - If no song, skip this and notify
	if v.nowPlaying == (Song{}) {
		s.ChannelMessageSend(m.ChannelID, "**[Muse]** Queue is empty - There's nothing to skip!")
	} else {
		s.ChannelMessageSend(m.ChannelID, "**[Muse]** Skipping ["+v.nowPlaying.Title+"] :loop:")
		v.stop = true
		v.speaking = false

		if v.encoder != nil {
			v.encoder.Cleanup()
		}
		log.Println("In Skip")
		log.Println("Queue Length: ", len(queue))
	}
}

func getQueue(m *discordgo.MessageCreate) {
	queueList := ":musical_note:   QUEUE LIST   :musical_note:\n"
	if v.nowPlaying != (Song{}) {
		queueList = queueList + "Now Playing: " + v.nowPlaying.Title + "  ->  Queued by <@" + v.nowPlaying.User + "> \n"
	}
	for index, element := range queue {
		queueList = queueList + " " + strconv.Itoa(index+1) + ". " + element.Title + "  ->  Queued by <@" + element.User + "> \n"
	}

	s.ChannelMessageSend(m.ChannelID, "**[Muse]** Fetching Queue...")
	s.ChannelMessageSend(m.ChannelID, queueList)
}

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
		v.stop = true

		// Nothing left in queue
		if len(queue) == 0 {
			v.nowPlaying = Song{}
			v.voice.Disconnect()
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Nothing left to play, peace! :v:")
		} else {
			s.ChannelMessageSend(m.ChannelID, "**[Muse]** Next! Now playing ["+queue[0].Title+"] :loop:")
		}
	}
}
