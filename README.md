
autbot is simple bot for Discord written in Go. Created just as educational project.

## Commands

- `!get killmail` - Gets some killmail from [zkillboard](https://zkillboard.com/)
- `!get gif tag` - Gets random gif from [giphy](https://giphy.com/) by tag. If no gifs found by tag, sends a message in text channel.
- `!help` - Sends a message in text channel with the above commands

Bot also can check zkillboard for specific character, corporation, or alliance killmails and post them automatically.

## Installation

autbot requires [discordgo](https://github.com/bwmarrin/discordgo), [websocket](https://github.com/gorilla/websocket), and [configure](https://github.com/paked/configure) packages, you can install them using these commands:

```bash
go get github.com/gorilla/websocket
go get github.com/bwmarrin/discordgo
go get github.com/paked/configure
```

Then install the bot itself:

```bash
go get github.com/tgrid0/autbot
go install github.com/tgrid0/autbot
```

There is a configuration file, that should be filled with your bot id token, filter string 
with character, corporation or alliance name (or '---' if not needed), specific channel id to autopost kill ('---' to disable autoposting), and string by that you will be remembered on zKillboard killmail listener. Just rename the `config.example.json` to `config.json`, or copy/paste the following.

```json
{
  "token": "",
  "auto_kill_post_filter": "---",
  "auto_kill_post_channel": "---",
  "zkill_redisq_id" : "somemaybeuniqueid"
}
```

Now you can start autbot

```bash
./autbot
```

## [License](LICENSE)

MIT
