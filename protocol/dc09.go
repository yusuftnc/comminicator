package protocol

import (
	"agent/model"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var subDc09Regexes = map[string]*model.SubRegex{}

/*
OPAX

Å[002A"NULL"0000R8L0#41213[]_21:00:10,02-20-2024
Á~003E"ADM-CID"0379R0L0#10064[10064|1602 00 000]_00:00:43,02-21-2024

PROSEC
B0970029"NULL"0000L000#0809[]_21:00:01,02-20-2024
F68B003B"ADM-CID"0028L0#9757[#9757|1602 00 000]_20:59:45,02-20-2024
B4330036"SIA-DCS"0442L000#36214[36214|NRP]_00:00:00,02-21-2024
3F60003C"SIA-DCS"0001L0#51449[#51449|Nri0000/LR]_21:26:19,12-20-2023
7B620035"SIA-DCS"0005L000#3044[3044|NOP1]_00:03:31,12-21-2023
85340035"SIA-DCS"0004L000#3044[3044|NBC1]_00:03:31,12-21-2023

HIKVISION
99820040"ADM-CID"1937R15L1#21401[#21401|1602 00 000]_20:59:59,02-20-2024
*/
func init() {
	addDc09Regex("mainRegex", `(?<Length>.*)"(?<MessageType>SIA-DCS|ADM-CID|NULL)"(?<Sequence>[0-9]{4})R?(?<Receiver>[A-Fa-f0-9]{1,6})?L(?<Line>[A-Fa-f0-9]{1,6})[#]?(?<CustomerNumber>[A-Fa-f0-9]{3,16})?[\[](?<Data>.*)$`, true)
	addDc09Regex("ADM-CID", `(?P<CustomerNumber>.*)\|(?P<EventType>[a-zA-Z1-9]{1})(?P<Event>\d{3})\s?(?P<Partition>\d{2})\s?(?P<Zone>\d{3}).*$`, true)
	addDc09Regex("SIA-DCS", `[#(?<CustomerNumber>\d+)[F]*\|Nri(?<Partition>\d)\/(?<Events>.+?)\]`, true)
	addDc09Regex("NULL", `[\]][_]?(?<TimeStamp>[0-9:,-_\s]*)?$`, true)
}

func addDc09Regex(name, regexText string, isActive bool) {
	compiledRegex, err := regexp.Compile(regexText)
	if err != nil {
		log.Fatalf("Regex derlenirken hata oluştu: %v", err)
	}
	subDc09Regexes[name] = &model.SubRegex{
		Name:          name,
		RegexText:     regexText,
		IsActive:      isActive,
		CompiledRegex: compiledRegex,
	}
}
func ParseDc09(event, receiverId string) (signal []interface{}, ack string, err error) {
	mainData := map[string]string{}
	eventData := map[string]string{}
	//fmt.Println("BAKMA BAKALIM=", event)
	mainData = applyDc09Regex(event, "mainRegex")
	//fmt.Println("BAK BAKALIM=", mainData["Data"])

	fmt.Println("-----------------------------------------------------------------------")
	if mainData == nil {
		return nil, "", nil
	}

	if mainData["MessageType"] == "SIA-DCS" {
		eventData = applyDc09Regex(mainData["Data"], "SIA-DCS")
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
		eventData = applyDc09Regex(mainData["Data"], "ADM-CID")
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
		eventData = applyDc09Regex(mainData["Data"], "NULL")
		signal = append(signal, model.PingSignal{
			Type:             "ping",
			SideNo:           mainData["CustomerNumber"],
			ReceiverId:       receiverId,
			MonitoringCenter: 1,
			RawSignal:        event,
		})
	}
	// l[003C"ADM-CID"0148R0L0#8362[8362|1602 00 000]_00:00:09,06-11-2022
	// l[003C"ACK"0148R0L0#8362[]
	//<LF><CRC><0LLL><"ACK"><seq><Rrcvr><Lpref><#acct>[]<CR>
	//Y9002A"NULL"0000R8L0#41213[]_21:00:08,06-10-2022
	//Y9002A"ACK"0000R8L0#41213[]
	return signal, "\n" + mainData["Length"] + "\"ACK\"" + mainData["Sequence"] + "R" + mainData["Receiver"] + "L" + mainData["Line"] + "#" + mainData["CustomerNumber"] + "[]" + "\r", nil
}

func applyDc09Regex(eventStr, regexName string) map[string]string {
	regex, exists := subDc09Regexes[regexName]
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
