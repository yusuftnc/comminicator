package protocol

import (
	"agent/model"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var subFonriRegexes = map[string]*model.SubRegex{}

/*
NOVA
1E5A0046"ADM-CID"1709L0#63121[#63121|1401 01 001][IKeypad]_01:07:45,02-21-2024
*/
func init() {
	addFonriRegex("mainRegex", `(?<Length>.*)"(?<MessageType>SIA-DCS|ADM-CID|NULL)"(?<Sequence>[0-9]{4})R?(?<Receiver>[A-Fa-f0-9]{1,6})?L(?<Line>[A-Fa-f0-9]{1,6})[#]?(?<CustomerNumber>[A-Fa-f0-9]{3,16})?[\[](?<Data>.*)$`, true)
	addFonriRegex("ADM-CID", `(?P<CustomerNumber>.*)\|(?P<EventType>[a-zA-Z1-9]{1})(?P<Event>\d{3})\s?(?P<Partition>\d{2})\s?(?P<Zone>\d{3})\]\[(?P<ZoneName>[^\]]*)\].*$`, true)
	addFonriRegex("SIA-DCS", `[#(?<CustomerNumber>\d+)[F]*\|Nri(?<Partition>\d)\/(?<Events>.+?)\]`, true)
	addFonriRegex("NULL", `[\]][_]?(?<TimeStamp>[0-9:,-_\\s]*)?$`, true)
}

func addFonriRegex(name, regexText string, isActive bool) {
	compiledRegex, err := regexp.Compile(regexText)
	if err != nil {
		log.Fatalf("Regex derlenirken hata oluştu: %v", err)
	}
	subFonriRegexes[name] = &model.SubRegex{
		Name:          name,
		RegexText:     regexText,
		IsActive:      isActive,
		CompiledRegex: compiledRegex,
	}
}
func ParseFonri(event, receiverId string) (signal []interface{}, ack string, err error) {
	mainData := map[string]string{}
	eventData := map[string]string{}
	//fmt.Println("BAKMA BAKALIM=", event)
	mainData = applyFonriRegex(event, "mainRegex")
	//fmt.Println("BAK BAKALIM=", mainData["Data"])

	fmt.Println("-----------------------------------------------------------------------")
	if mainData == nil {
		return nil, "", nil
	}

	if mainData["MessageType"] == "SIA-DCS" {
		eventData = applyFonriRegex(mainData["Data"], "SIA-DCS")
		events := strings.Split(eventData["Events"], "/")
		for _, e := range events {
			fmt.Println("events", e)
			evt, zn := model.SiaEventOrZone(e)
			signal = append(signal, model.AlarmSignal{
				SideNo:           mainData["CustomerNumber"],
				ReceiverId:       receiverId,
				ReceiverNo:       mainData["Receiver"],
				LineNo:           mainData["Line"],
				PartNo:           eventData["Partition"],
				MonitoringCenter: 1,
				SignalDateTime:   time.Now(),
				RawSignal:        event,
				EventCode:        evt,
				Zone:             zn,
			})
			//fmt.Println("Test Json", signal)
		}
	} else if mainData["MessageType"] == "ADM-CID" {
		eventData = applyFonriRegex(mainData["Data"], "ADM-CID")
		signal = append(signal, model.AlarmSignal{
			SideNo:           eventData["CustomerNumber"],
			ReceiverId:       receiverId,
			ReceiverNo:       mainData["Receiver"],
			LineNo:           mainData["Line"],
			PartNo:           eventData["Partition"],
			MonitoringCenter: 1,
			SignalDateTime:   time.Now(),
			RawSignal:        event,
			EventCode:        eventData["EventType"] + eventData["Event"],
			Zone:             eventData["Zone"],
		})
	} else if mainData["MessageType"] == "NULL" {
		eventData = applyFonriRegex(mainData["Data"], "NULL")
		signal = append(signal, model.PingSignal{
			Type:             "event",
			ReceiverId:       receiverId,
			MonitoringCenter: 1,
			RawSignal:        event,
		})
	}

	return signal, string([]byte{0x06}), nil
}

func applyFonriRegex(eventStr, regexName string) map[string]string {
	regex, exists := subFonriRegexes[regexName]
	if !exists {
		fmt.Printf("No regex found for %s, %s\n", eventStr, regexName)
		return nil
	}

	match := regex.CompiledRegex.FindStringSubmatch(eventStr)
	if match == nil {
		fmt.Println("No match found", "GELEN=", eventStr, "REGEX", regex)
		return nil
	}

	result := make(map[string]string)
	for i, name := range regex.CompiledRegex.SubexpNames() {
		if i != 0 && name != "" { // İlk eleman tam eşleşmeyi içerir ve isimsizdir
			if name == "EventType" {
				if match[i] == "1" {
					match[i] = "E"
				} else if match[i] == "3" {
					match[i] = "R"
				}
			}
			result[name] = match[i]
		}
	} // BURAYI SİLMEK İSTİYORUM YSF

	return result
}
