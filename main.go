package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/tidwall/gjson"
)

// 디스코드로 들어오는 메시지의 시간대는 UTC+0.
// 로컬 시간대로 맞춰서 DB에 넣도록 시간대를 잘 바꾸어 잘 포맷된 형태로 반환한다.
func getTime(discordtime discordgo.Timestamp) string {
	temp, _ := discordtime.Parse()
	nowZoneName, nowZoneOff := time.Now().Zone()
	temp = temp.In(time.FixedZone(nowZoneName, nowZoneOff))
	weekday := temp.Weekday()
	weekdayName := ""
	switch weekday {
	case 0:
		weekdayName = "일"
	case 1:
		weekdayName = "월"
	case 2:
		weekdayName = "화"
	case 3:
		weekdayName = "수"
	case 4:
		weekdayName = "목"
	case 5:
		weekdayName = "금"
	case 6:
		weekdayName = "토"
	}

	field := strings.Fields(temp.Format("2006-01-02 15:04:05"))
	return field[0] + " (" + weekdayName + ") " + field[1]
}

func readFile(path string) string {
	b, e := ioutil.ReadFile(path)
	if e != nil {
		panic(e)
	}
	return string(b)
}

func recordInNOut(sess *discordgo.Session, msg *discordgo.MessageCreate, suffix string) {
	guildID, channelID, authorID, timestamp := msg.GuildID, msg.ChannelID, msg.Author.ID, msg.Timestamp

	e := sess.ChannelMessageDelete(channelID, msg.ID)
	if e != nil {
		log.Println(e)
		return
	}

	member, e := sess.GuildMember(guildID, authorID)
	if e != nil {
		log.Println(e)
		return
	}

	content := "`" + getTime(timestamp) + "` <@!" + member.User.ID + "> `" + suffix + "`"

	_, e = sess.ChannelMessageSend(channelID, content)
	if e != nil {
		log.Println(e)
		return
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	conf := gjson.Parse(readFile("./config.json"))

	discord, e := discordgo.New("Bot " + conf.Get("discord.token").String())
	if e != nil {
		log.Fatalln("error creating Discord session,", e)
		return
	}
	defer discord.Close()

	if e = discord.Open(); e != nil {
		log.Fatalln("error opening connection,", e)
		return
	}

	// Guild에 초대 / 접속했을 때 실행하는 부분
	discord.AddHandler(func(session *discordgo.Session, event *discordgo.GuildCreate) {
		// Guild에 허용된 채널 읽기
		for _, channel := range event.Guild.Channels {
			if channel.Type == discordgo.ChannelTypeGuildText {
				permission, e := session.UserChannelPermissions(discord.State.User.ID, channel.ID)
				if e != nil {
					log.Println(e)
				}

				mustRequired := discordgo.PermissionViewChannel |
					discordgo.PermissionSendMessages |
					discordgo.PermissionManageMessages |
					discordgo.PermissionReadMessageHistory

				if permission&int64(mustRequired) == int64(mustRequired) {
					log.Printf("found channel: [%v] in [%v]\n", channel.Name, event.Guild.Name)
				}
			}
		}
	})

	if e := discord.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{
			{
				Name: "명령: 1=입실, 0=퇴실",
				Type: discordgo.ActivityTypeGame,
			},
		},
	}); e != nil {
		log.Fatalln("error update status complex,", e)
		return
	}

	discord.AddHandler(func(sess *discordgo.Session, msg *discordgo.MessageCreate) {
		if msg.Author.ID == discord.State.User.ID {
			return
		}

		if len(msg.Content) == 0 {
			return
		}

		switch msg.Content {
		case "0":
			recordInNOut(sess, msg, "퇴실")
		case "1":
			recordInNOut(sess, msg, "입실")
		}
	})

	// Ctrl+C를 받아서 프로그램 자체를 종료하는 부분. os 신호를 받는다
	log.Println("bot is now running. Press Ctrl+C to exit.")
	{
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sc
	}
	log.Println("received Ctrl+C, please wait.")
}
