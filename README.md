# AutoMuse
Automuse is a discord bot that plays music in a discord voice channel via commands. At the moment, only youtube video or playlist links can be played. The bot is still a WIP and may not work as intended; It's rare but some songs finish before they should.

:point_right: You can add this bot to your server [here](https://discord.com/api/oauth2/authorize?client_id=955836104559460362&permissions=534723950656&scope=bot%20applications.commands)

:point_right: Get/set your bot token [here](https://discord.com/developers/applications/)

:point_right: Follow the official YouTube documentation to get/set your YouTube API key [here](https://developers.google.com/youtube/v3/docs)

# Requirements
- GoLang 1.18
- Your very own discord bot token placed in an environment variable (See Link Above)
     - Env var: BOT_TOKEN
- Your very own YouTube API Key placed in an environment variable (See Link Above)
    - Env var: YT_TOKEN

# How to use
- Typing the play command in any text channel will trigger the bot to join your voice channel, you must be in a voice channel for this to work.
- Playing additional links will place the songs in a queue. 
- The queue will auto-play until done.
- You can skip songs in the queue.
- you can stop the current song and clear the queue
- When no songs are left in the queue, the bot will leave the channel. Play a new song to bring it back in your voice channel.

## Syntax
###### Base Commands to Use the Bot
````
play https://www.youtube.com/watch?v=<VIDEO-ID>          -> Plays/Queues a video
play https://www.youtube.com/playlist?list=<PLAYLIST-ID> -> Plays/Queues a playlist
skip                                                     -> Skips the current Song
stop                                                     -> Stops the current song and clears the queue
queue                                                    -> Shows the current queue in chat
remove #                                                 -> Remove a song from queue at number #
````