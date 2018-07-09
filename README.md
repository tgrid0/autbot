
autbot is simple bot for Discord written in Go. Created just as educational project.

## Commands

- `!kmpost init` - initialize killmail posting for this channel
- `!kmpost enable` - initialize and enable immediately (be ready for killmail spam)
- `!kmpost disable` - disable posting for this channel
- `!kmpost filter list` - list filters for channel
- `!kmpost filter add <person> <corporation> <alliance> <system>` - add filter to channel, only killmails that contain one this names will be posted (atleast one name needs to be filled)
- `!kmpost filter rem <index>` - remove filter by index
- `!kmpost help` - print the above commands

Bot also can check zkillboard for specific character, corporation, or alliance killmails and post them automatically.

## Installation

autbot requires [discordgo](https://github.com/bwmarrin/discordgo) and [websocket](https://github.com/gorilla/websocket) packages, you can install them using these commands:

```bash
go get github.com/gorilla/websocket
go get github.com/bwmarrin/discordgo
```

Then install the bot itself:

```bash
go get github.com/tgrid0/autbot
go install github.com/tgrid0/autbot
```

There is a configuration file, that needs to be filled with your bot id token. Just rename the `config.example.json` to `config.json`, or copy/paste the following.

```json
{
  "token": "your bot token"
}
```

Now you can start autbot

```bash
./autbot
```

## [License](LICENSE)

MIT
