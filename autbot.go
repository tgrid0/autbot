package main

import (
	"fmt"
	"time"
	"net/http"
	"io/ioutil"
	"strings"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/paked/configure"
)

var (
	conf      				= configure.New()
	token     				= conf.String("token", "", "Bot Token")
	auto_kill_post_filter 	= conf.String("auto_kill_post_filter", "", "Name of corporation, alliance, or character to track and post killmails ('---' for no filtering)")
	auto_kill_post_channel 	= conf.String("auto_kill_post_channel", "", "Specific discord channel id to post killmails")
	zkill_redisq_id			= conf.String("zkill_redisq_id", "", "Identifier you may use on zkillboard redisq (any characters and numbers)")
)

func get_rand_gif(tag string) string {
	// using public gihpy key
	resp, err := http.Get("http://api.giphy.com/v1/gifs/random?api_key=dc6zaTOxFJmzC&tag=" + tag)
	if err != nil {
		fmt.Println("Error processing http.Get")
		return "nourl"
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	
	s := string(body[:])
	
	if !strings.Contains(s, "image_original_url") {
		return "nourl"
	}
	
	params := strings.Split(s, ",")
	//remove some backslashes
	orig_url := strings.Replace(strings.Split(params[3], "\"")[3], "\\/", "/", -1)
	
	//fmt.Printf("%s\n", orig_url)
	
	return orig_url
	
}

func get_last_kill(filter string) string {

	resp, err := http.Get("http://redisq.zkillboard.com/listen.php?queueID=" + *zkill_redisq_id)
	if err != nil {
		fmt.Println("Error processing http.Get")
		return "nokill"
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	
	s := string(body[:])
	
	if "---" == filter || strings.Contains(s, filter) {
		x := strings.Split(s, ",")
		killId := strings.Split(x[0], ":")[2]
		killLink := "https://zkillboard.com/kill/" + killId
		return killLink
	}
	
	return "nokill"
}

func main() {
	
	conf.Use(configure.NewFlag())
	conf.Use(configure.NewEnvironment())
	_, err := os.Stat("config.json") 
	if err != nil {
		fmt.Println("Config file does not exist!")
		return
	}
	
	conf.Use(configure.NewJSONFromFile("config.json"))
	
	conf.Parse()

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + *token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register messageCreate as a callback for the messageCreate events.
	dg.AddHandler(messageCreate)

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	
	// Create a goroutine for posting kills to channel
	
    quit := make(chan int)

	if *auto_kill_post_channel != "---" {
		go func() {
				select {
				default:
						fmt.Println("Searching for killmails")
						killLink := get_last_kill(*auto_kill_post_filter)
						if killLink != "nokill" {
							// send message to specific channel
							dg.ChannelMessageSend(*auto_kill_post_channel, killLink) 
							fmt.Println("Killmail sent")
						}
						time.Sleep(10 * time.Millisecond)
				case <- quit:
						fmt.Println("Routine quit")
				}
		}()
	}
	
	// Simple way to keep program running until CTRL-C is pressed.
	<-make(chan struct{})
	
	close(quit) // Stop the goroutine
	
	return
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Print message to stdout.
	fmt.Printf("%20s %20s %20s > %s\n", m.ChannelID, time.Now().Format(time.Stamp), m.Author.Username, m.Content)
	
	if m.Content[:1] == "!" {

		splits := strings.Split(m.Content, " ")
		command := splits[0][1:]

		if command == "get" {
			if len(splits) < 2 {
				s.ChannelMessageSend(m.ChannelID, "```Expected '!get killmail' or '!get gif <something>'```")
				return
			}
			arg := splits[1]
			if arg == "killmail" {
				s.ChannelMessageSend(m.ChannelID, get_last_kill("---"))
			} else if arg == "gif" {
				if len(splits) < 3 {
					s.ChannelMessageSend(m.ChannelID, "```Expected '!get gif <something>'```")
					return
				}
				tag := splits[2]
				gifurl := get_rand_gif(tag)
				if gifurl != "nourl" {
					s.ChannelMessageSend(m.ChannelID, gifurl)
				} else {
					s.ChannelMessageSend(m.ChannelId, "Sorry, no gifs were found by tag '" + tag + "'")
				}
			}
		} else if command == "help" {
			s.ChannelMessageSend(m.ChannelID, "```Commands:\n!get killmail - get some kill from zkillboard\n!get gif <tag> - print gif by tag(cat, owl, etc)```")
		}
	}
}