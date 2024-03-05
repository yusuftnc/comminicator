package protocol

import (
	"agent/model"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var subTeknimRegexes = map[string]*model.SubRegex{}

/*
TEKNIM
59630027"NULL"0000L1#699F[]_23:50:57,02-20-2024
56b8003c"ADM-CID"1569L1#6827[#6827|18160201000D]_00:00:00,02-21-2024
39660039"SIA-DCS"0512L1#9764[#9764|Nri1/OP03]_00:41:51,06-13-2000

(?<Crc>[A-Fa-f0-9]{4})(?<Lenght>[A-Fa-f0-9]{4})"(?<MessageType>SIA-DCS|ADM-CID|NULL)"(?<Sequence>[0-9]{4})(?<Receiver>R[A-Fa-f0-9]{1,6})?(?<Line>L[A-Fa-f0-9]{1,6})[#]?(?<Account>[A-Fa-f0-9]{3,16})?[\[](?<Data>.*)
[{"Name":"ADM-CID","RegexText":"[#]?[|](?<Data>[\\d\\s\\w]*)[\\]][_]?(?<TimeStamp>[0-9:,-_\\s]*)?$","IsActive":true},{"Name":"SIA-DCS","RegexText":"[#](?<CustomerNumber>.*)\\|(?<Data>[a-zA-Z]+[\\w\\s\\/.]*)\\]","IsActive":true},{"Name":"NULL","RegexText":"[\\]][_]?(?<TimeStamp>[0-9:,-_\\s]*)?$","IsActive":true}]
*/
func init() {
	addTeknimRegex("mainRegex", `(?<Length>.*)"(?<MessageType>SIA-DCS|ADM-CID|NULL)"(?<Sequence>[0-9]{4})R?(?<Receiver>[A-Fa-f0-9]{1,6})?L(?<Line>[A-Fa-f0-9]{1,6})[#]?(?<CustomerNumber>[A-Fa-f0-9]{3,16})?[\[](?<Data>.*)$`, true)
	addTeknimRegex("ADM-CID", `(?P<CustomerNumber>.*)\|18(?P<EventType>[a-zA-Z1-9]{1})(?P<Event>\d{3})\s?(?P<Partition>\d{2})\s?(?P<Zone>\d{3}).*$`, true)
	addTeknimRegex("SIA-DCS", `[#(?<CustomerNumber>\d+)[F]*\|Nri(?<Partition>\d)\/(?<Events>.+?)\]`, true)
	addTeknimRegex("NULL", `[\]][_]?(?<TimeStamp>[0-9:,-_\\s]*)?$`, true)
}

func addTeknimRegex(name, regexText string, isActive bool) {
	compiledRegex, err := regexp.Compile(regexText)
	if err != nil {
		log.Fatalf("Regex derlenirken hata oluştu: %v", err)
	}
	subTeknimRegexes[name] = &model.SubRegex{
		Name:          name,
		RegexText:     regexText,
		IsActive:      isActive,
		CompiledRegex: compiledRegex,
	}
}
func ParseTeknim(event, receiverId string) (signal []interface{}, ack string, err error) {
	mainData := map[string]string{}
	eventData := map[string]string{}
	//fmt.Println("BAKMA BAKALIM=", event)
	mainData = applyTeknimRegex(event, "mainRegex")
	//fmt.Println("BAK BAKALIM=", mainData["Data"])

	fmt.Println("-----------------------------------------------------------------------")
	if mainData == nil {
		return nil, "", nil
	}

	if mainData["MessageType"] == "SIA-DCS" {
		eventData = applyTeknimRegex(mainData["Data"], "SIA-DCS")
		events := strings.Split(eventData["Events"], "/")
		for _, e := range events {
			fmt.Println("events", e)
			evt, zn := model.SiaEventOrZone(e)
			signal = append(signal, model.AlarmSignal{
				Type:             "event",
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
		eventData = applyTeknimRegex(mainData["Data"], "ADM-CID")
		signal = append(signal, model.AlarmSignal{
			Type:             "event",
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
		eventData = applyTeknimRegex(mainData["Data"], "NULL")
		signal = append(signal, model.PingSignal{
			Type:             "ping",
			ReceiverId:       receiverId,
			MonitoringCenter: 1,
			RawSignal:        event,
		})
	}

	return signal, string([]byte{0x06}), nil
}

func applyTeknimRegex(eventStr, regexName string) map[string]string {
	regex, exists := subTeknimRegexes[regexName]
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
