package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type filterRule struct {
	Person     string `json:"person"`
	Corp       string `json:"corp"`
	Alliance   string `json:"alliance"`
	System     string `json:"system"`
	PersonID   string `json:"personID"`
	CorpID     string `json:"corpID"`
	AllianceID string `json:"allianceID"`
	SystemID   string `json:"systemID"`
}

type channelControl struct {
	ChannelID string       `json:"channelID"`
	Filters   []filterRule `json:"filters"`
	Enabled   bool         `json:"enabled"`
}

type config struct {
	BotToken string `json:"token"`
	AppID    string `json:"appID"`
}

var (
	conf            config
	zkillRedisqID   string
	enabledChannels int
	channels        map[string]channelControl

	infoLog    *log.Logger
	warningLog *log.Logger
	errorLog   *log.Logger
)

var transport http.RoundTripper = &http.Transport{
	DisableKeepAlives: true,
}

var c = &http.Client{Transport: transport}

func reverse(s string) string {
	chars := []rune(s)
	for i, j := 0, len(chars)-1; i < j; i, j = i+1, j-1 {
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars)
}

func insertNth(s string, n int, x rune) string {
	var buffer bytes.Buffer
	var n1 = n - 1
	var l1 = len(s) - 1
	for i, rune := range s {
		buffer.WriteRune(rune)
		if i%n == n1 && i != l1 {
			buffer.WriteRune(x)
		}
	}
	return buffer.String()
}

func parseISKValue(value float64, dotOffset int) string {
	var retStr string
	var tmp string

	tmp = reverse(strconv.FormatFloat(value, 'f', 2, 64))
	tmp = reverse(insertNth(tmp[dotOffset:], 3, ' '))

	retStr = strconv.FormatFloat(value, 'f', 2, 64)

	return tmp + retStr[len(retStr)-dotOffset:]
}

func initLogs(
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	infoLog = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	warningLog = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	errorLog = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func constructChControl(channelID string) channelControl {
	chControl := channelControl{ChannelID: channelID, Enabled: false, Filters: make([]filterRule, 0)}
	return chControl
}

func main() {

	initLogs(os.Stdout, os.Stdout, os.Stderr)

	_, err := os.Stat("config.json")
	if err != nil {
		errorLog.Println("Config file does not exist!")
		return
	}

	confContent, readErr := ioutil.ReadFile("config.json")
	if readErr != nil {
		errorLog.Println("Error reading settings file!")
	} else {
		_ = json.Unmarshal(confContent, &conf)
		if conf.BotToken == "" {
			errorLog.Println("Bot token is empty, exiting!")
			return
		}
		if conf.AppID == "" {
			rand.Seed(29)
			conf.AppID = "autbot" + strconv.Itoa(rand.Int())
			confToSave, _ := json.Marshal(conf)
			_ = ioutil.WriteFile("config.json", confToSave, 0644)
		}
	}

	enabledChannels = 0
	_, err = os.Stat("settings.json")
	if err == nil {
		settingsContent, readErr := ioutil.ReadFile("settings.json")
		if readErr != nil {
			errorLog.Println("Error reading settings file!")
		} else {
			_ = json.Unmarshal(settingsContent, &channels)
			for _, elem := range channels {
				if elem.Enabled == true {
					enabledChannels++
				}
			}
		}
	} else {
		channels = make(map[string]channelControl)
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + conf.BotToken)
	if err != nil {
		errorLog.Println("error creating Discord session,", err)
		return
	}

	// Register messageCreate as a callback for the messageCreate events.
	dg.AddHandler(messageCreate)

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		errorLog.Println("error opening connection,", err)
		return
	}

	infoLog.Println("Bot is now running.  Press CTRL-C to exit.")

	quit := make(chan int)

	/* routine for posting kills from zkillboard to discord channel
	 */
	go func() {
		for {
			select {
			default:
				if enabledChannels > 0 {
					resp, err := c.Get("http://redisq.zkillboard.com/listen.php?queueID=" + conf.AppID)
					if err == nil {

						body, _ := ioutil.ReadAll(resp.Body)
						resp.Body.Close()

						s := string(body[:])

						data := s[11 : len(s)-1]

						x := strings.Split(s, ",")
						if len(x) < 2 {
							continue
						}
						x = strings.Split(x[0], ":")
						if len(x) < 3 {
							continue
						}
						killID := x[2]
						killLink := "https://zkillboard.com/kill/" + killID + "/"

						var f interface{}
						err := json.Unmarshal([]byte(data), &f)
						if err != nil {
							errorLog.Println(err)
						}

						m := f.(map[string]interface{})

						killMailInfo := m["killmail"].(map[string]interface{})
						victimInfo := killMailInfo["victim"].(map[string]interface{})
						attackersInfo := killMailInfo["attackers"].([]interface{})
						zkbInfo := m["zkb"].(map[string]interface{})

						_, isCharacter := victimInfo["character_id"] //filter structures
						if !isCharacter {
							continue
						}

						var victimSideID string
						var victimSideType string

						_, ok := victimInfo["alliance_id"]
						if ok {
							victimSideID = strconv.FormatFloat(victimInfo["alliance_id"].(float64), 'f', 0, 64)
							victimSideType = "alliances"
						} else {
							victimSideID = strconv.FormatFloat(victimInfo["corporation_id"].(float64), 'f', 0, 64)
							victimSideType = "corporations"
						}

						attckCount := len(attackersInfo)
						var attackerSideID string
						var attackerSideType string
						var attackerID string

						for i := 0; i < attckCount; i++ {
							attackerInfo := attackersInfo[i].(map[string]interface{})
							isKiller := attackerInfo["final_blow"].(bool)
							if isKiller {
								atkID, isChar := attackerInfo["character_id"].(float64)
								if isChar {
									attackerID = strconv.FormatFloat(atkID, 'f', 0, 64)
									_, ok := attackerInfo["alliance_id"]
									if ok {
										attackerSideID = strconv.FormatFloat(attackerInfo["alliance_id"].(float64), 'f', 0, 64)
										attackerSideType = "alliances"
									} else {
										attackerSideID = strconv.FormatFloat(attackerInfo["corporation_id"].(float64), 'f', 0, 64)
										attackerSideType = "corporations"
									}
								} else {
									attackerID = "NPC"
								}

								break
							}
						}

						victimName := getName("characters", strconv.FormatFloat(victimInfo["character_id"].(float64), 'f', 0, 64))
						victimSide := getName(victimSideType, victimSideID)
						victimShipID := strconv.FormatFloat(victimInfo["ship_type_id"].(float64), 'f', 0, 64)
						attackersCount := strconv.Itoa(attckCount)
						var attackerName string
						var attackerSide string
						if attackerID == "NPC" {
							attackerName = attackerID
							attackerSide = ""
						} else {
							attackerName = getName("characters", attackerID)
							attackerSide = "(" + getName(attackerSideType, attackerSideID) + ")"
						}

						totalValue := zkbInfo["totalValue"].(float64)

						embd := &discordgo.MessageEmbed{
							Color:       0xff0000,
							Title:       killLink,
							Description: "Some kill posted!",
							URL:         killLink,
							Thumbnail: &discordgo.MessageEmbedThumbnail{
								URL: "https://imageserver.eveonline.com/Render/" + victimShipID + "_64.png",
							},
						}

						embd.Description = victimName + " (" + victimSide + ") lost " + getShipName(victimShipID) + " to " + attackerName + " " + attackerSide + ", " + attackersCount + " attackers total (" + parseISKValue(totalValue, 3) + " ISK)"

						for chID, chCntrl := range channels {
							if chCntrl.Enabled == true {
								if len(chCntrl.Filters) == 0 {
									_, err := dg.ChannelMessageSendEmbed(chID, embd)
									if err != nil {
										fmt.Println(err)
									}
								} else {
									for _, ruleSet := range chCntrl.Filters {
										if (ruleSet.PersonID == "-" || strings.Contains(s, `"character_id":`+ruleSet.PersonID)) &&
											(ruleSet.CorpID == "-" || strings.Contains(s, `"corporation_id":`+ruleSet.CorpID)) &&
											(ruleSet.AllianceID == "-" || strings.Contains(s, `"alliance_id":`+ruleSet.AllianceID)) &&
											(ruleSet.SystemID == "-") || strings.Contains(s, `"solar_system_id":`+ruleSet.SystemID) {

											checkAttackers := s[strings.Index(s, "attackers"):]

											if (ruleSet.PersonID == "-" || strings.Contains(checkAttackers, `"character_id":`+ruleSet.PersonID)) &&
												(ruleSet.CorpID == "-" || strings.Contains(checkAttackers, `"corporation_id":`+ruleSet.CorpID)) &&
												(ruleSet.AllianceID == "-" || strings.Contains(checkAttackers, `"alliance_id":`+ruleSet.AllianceID)) {
												embd.Color = 0x00ff00
											}
											dg.ChannelMessageSendEmbed(chID, embd)
											break
										}
									}
								}
							}
						}

					} else {
						errorLog.Println("Error processing http.Get")
					}

				} else {
					time.Sleep(5000 * time.Millisecond)
				}
			case <-quit:
				infoLog.Println("Routine quit")
				return
			}
		}
	}()

	// Simple way to keep program running until CTRL-C is pressed.
	<-make(chan struct{})

	close(quit) // Stop the goroutine

	return
}

/* get ID by name from ESI
 * category - character, corporation, alliance, system
 * returns string ID or "-"
 */
func getID(category string, name string) string {
	resp, err := c.Get("https://esi.tech.ccp.is/latest/search/?categories=" + category +
		"&datasource=tranquility&language=en-us&search=" + name + "&strict=false")
	if err == nil {
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		s := string(body[:])
		if strings.Contains(s, "error") {
			errorLog.Println(s)
			return "-"
		}
		s = strings.Split(s, "[")[1]
		retid := string(s[:len(s)-2])
		return retid
	}

	errorLog.Println("Error processing http.Get")
	return "-"
}

/* get ship name by id
 * returns string name or "Unknown ship"
 */
func getShipName(id string) string {
	resp, err := c.Get("https://esi.tech.ccp.is/latest/universe/types/" + id +
		"/?datasource=tranquility&language=en-us")
	if err == nil {
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		s := string(body[:])
		if strings.Contains(s, "error") {
			errorLog.Println(s)
			return "Unknown ship"
		}

		var f interface{}
		err := json.Unmarshal([]byte(s), &f)
		if err != nil {
			fmt.Println(err)
		}
		m := f.(map[string]interface{})

		name, ok := m["name"].(string)
		if ok {
			retname := string(name)
			return retname
		}
		return "Unknown ship"
	}

	errorLog.Println("Error processing http.Get")
	return "Unknown ship"
}

/* get name by id
 * category - character, corporation, alliance, solarsystem
 * returns string ID or "-"
 */
func getName(category string, id string) string {
	resp, err := c.Get("https://esi.tech.ccp.is/latest/" + category + "/" + id +
		"/?datasource=tranquility")
	if err == nil {
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		s := string(body[:])
		if strings.Contains(s, "error") {
			errorLog.Println(s)
			return "-"
		}

		var f interface{}
		err := json.Unmarshal([]byte(s), &f)
		if err != nil {
			errorLog.Println(err)
		}
		if f == nil {
			errorLog.Println("Error parsing EVE ESI response!")
			return "-"
		}
		m := f.(map[string]interface{})

		name, ok := m["name"].(string)
		if ok {
			retname := string(name)
			return retname
		}
		return "-"
	}

	errorLog.Println("Error processing http.Get")
	return "-"
}

/* handle messages from users
 */
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if len(m.Content) < 2 {
		return
	}

	if m.Content[:1] == "!" {

		splits := strings.Split(m.Content, " ")

		command := splits[0][1:]

		if command == "kmpost" {
			if len(splits) < 2 {
				s.ChannelMessageSend(m.ChannelID, "```Expected atleast two words!```")
				return
			}
			arg := splits[1]
			if arg == "init" {
				chControl, hasKey := channels[m.ChannelID]
				if hasKey == false {
					chControl = constructChControl(m.ChannelID)
					channels[m.ChannelID] = chControl
					s.ChannelMessageSend(m.ChannelID, "```Channel initialized!```")
				} else {
					s.ChannelMessageSend(m.ChannelID, "```Channel already initialized!```")
				}
			} else if arg == "enable" {
				chControl, hasKey := channels[m.ChannelID]
				if hasKey == false {
					chControl = constructChControl(m.ChannelID)
				}

				if chControl.Enabled == true {
					s.ChannelMessageSend(m.ChannelID, "```Posting already enabled!```")
					return
				}
				s.ChannelMessageSend(m.ChannelID, "```Enabling posting on this channel!```")
				enabledChannels++
				chControl.Enabled = true
				channels[m.ChannelID] = chControl
				return
			} else if arg == "disable" {
				chControl, hasKey := channels[m.ChannelID]
				if hasKey == false {
					s.ChannelMessageSend(m.ChannelID, "```Posting not enabled!```")
					return
				}
				if chControl.Enabled == false {
					s.ChannelMessageSend(m.ChannelID, "```Posting not enabled!```")
					return
				}
				s.ChannelMessageSend(m.ChannelID, "```Disabling posting.```")
				enabledChannels--
				chControl.Enabled = false
				channels[m.ChannelID] = chControl
				return
			} else if arg == "filter" {
				if len(splits) < 3 {
					s.ChannelMessageSend(m.ChannelID, "```Expected atleast three words for filter configuration!```")
					return
				}
				filterArg := splits[2]
				if filterArg == "list" {
					// show added filters
					chControl, hasKey := channels[m.ChannelID]
					if hasKey == false {
						s.ChannelMessageSend(m.ChannelID, "```No info for this channel!```")
						return
					}
					if len(chControl.Filters) == 0 {
						s.ChannelMessageSend(m.ChannelID, "```No filters for this channel!```")
						return
					}
					msg := "```Filters for this channel:\n"
					for i := 0; i < len(chControl.Filters); i++ {
						msg += strconv.Itoa(i+1) + ": <" + chControl.Filters[i].Person + "> <" + chControl.Filters[i].Corp +
							"> <" + chControl.Filters[i].Alliance + "> <" + chControl.Filters[i].System + ">\n"
					}
					msg += "```"
					s.ChannelMessageSend(m.ChannelID, msg)
				} else if filterArg == "add" {
					chControl, hasKey := channels[m.ChannelID]
					if hasKey == false {
						s.ChannelMessageSend(m.ChannelID, "```The channel is not configured, type '!kmpost init' to initialize!```")
						return
					}
					// add filter with format 'person corp alliance system', '-' for ignore param
					params := strings.Split(m.Content, ">")
					if len(params)-1 < 4 {
						s.ChannelMessageSend(m.ChannelID, "```Please specify filter rule in format: <person> <corporation> <alliance> <system>, leaving <-> for parameter ignore!```")
						return
					}
					ruleSet := make([]string, 4)
					ruleIds := make([]string, 4)
					categories := make([]string, 4)
					categories[0] = "character"
					categories[1] = "corporation"
					categories[2] = "alliance"
					categories[3] = "solarsystem"
					for i := 0; i < 4; i++ {
						part := strings.Split(params[i], "<")
						ruleSet[i] = part[1]
						if part[1] != "-" && len(part[1]) < 3 {
							s.ChannelMessageSend(m.ChannelID, "```Parameter "+part[1]+" is too short, aborting fitler creation!```")
							return
						}
						if part[1] != "-" {
							part[1] = strings.Replace(part[1], " ", "%20", -1)
							ruleIds[i] = getID(categories[i], part[1])
						} else {
							ruleIds[i] = "-"
						}
					}
					chControl.Filters = append(chControl.Filters,
						filterRule{Person: ruleSet[0], Corp: ruleSet[1], Alliance: ruleSet[2], System: ruleSet[3], PersonID: ruleIds[0], CorpID: ruleIds[1], AllianceID: ruleIds[2], SystemID: ruleIds[3]})
					channels[m.ChannelID] = chControl
					s.ChannelMessageSend(m.ChannelID, "```Filter created.```")
				} else if filterArg == "rem" {
					chControl, hasKey := channels[m.ChannelID]
					if hasKey == false {
						s.ChannelMessageSend(m.ChannelID, "```No info for this channel!```")
						return
					}
					if len(splits) < 4 {
						s.ChannelMessageSend(m.ChannelID, "```Error!\nCommand syntax: !kmpost filter rem <index>```")
						return
					}
					indexArg := splits[3]
					i, convErr := strconv.Atoi(indexArg)
					if convErr != nil {
						s.ChannelMessageSend(m.ChannelID, "```Error!\nCommand syntax: !kmpost filter rem <index>```")
						return
					}
					filters := chControl.Filters
					if i > len(filters) || i < 1 {
						s.ChannelMessageSend(m.ChannelID, "```Error!\nNo filter on that index!```")
						return
					}
					i = i - 1
					filters = append(filters[:i], filters[i+1:]...)
					chControl.Filters = filters
					channels[m.ChannelID] = chControl
				}
			} else if arg == "status" {
				chControl, hasKey := channels[m.ChannelID]
				if hasKey == false {
					s.ChannelMessageSend(m.ChannelID, "```No info for this channel!```")
					return
				}
				if chControl.Enabled == true {
					s.ChannelMessageSend(m.ChannelID, "```Status: posting enabled.```")
				} else {
					s.ChannelMessageSend(m.ChannelID, "```Status: posting disabled.```")
				}
				return
			} else if arg == "commit" {
				channelsToSave, _ := json.Marshal(channels)
				saveErr := ioutil.WriteFile("settings.json", channelsToSave, 0644)
				if saveErr != nil {
					s.ChannelMessageSend(m.ChannelID, "```Error while saving configuration!```")
				}
				s.ChannelMessageSend(m.ChannelID, "```Configuration saved successfuly.```")
			} else if arg == "help" {
				msg := "```- '!kmpost init' - initialize killmail posting for this channel\n"
				msg += "- '!kmpost enable' - initialize and enable immediately\n"
				msg += "- '!kmpost disable' - disable posting for this channel\n"
				msg += "- '!kmpost filter list' - list filters for channel\n"
				msg += "- '!kmpost filter add <person> <corporation> <alliance> <system>' - add filter to channel\n"
				msg += "- '!kmpost filter rem <index>' - remove filter by index\n"
				msg += "- '!kmpost help' - print this message```"
				s.ChannelMessageSend(m.ChannelID, msg)
			}
		}
	}
}
