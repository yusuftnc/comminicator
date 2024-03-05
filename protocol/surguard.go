package protocol

import (
	"agent/model"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

var surguardRegexes = map[string]*model.SubRegex{}

func init() {
	addSurguardRegex("ADM-CID-1", `5(?<Receiver>\d{2})(?<Line>\d{1,3})[a-zA-Z\s]*(?<ContactCode>\d{2})(?<CustomerNumber>.*)(?<EventType>[a-zA-Z]{1})(?<Event>[\w?\d?]{3})(?<Partition>\d{2})(?<Zone>\d{3}).*$`, true)
	addSurguardRegex("ADM-CID-2", `5(?<Receiver>\w*)[a-zA-Z\s]*(?<ContactCode>\d{2})(?<CustomerNumber>.*)(?<EventType>[a-zA-Z]{1})(?<Event>\d{3})(?<Partition>\d{2})(?<Zone>\d{3}).*$`, true)
	addSurguardRegex("TEL", `4(?<Receiver>\d{2})(?<Line>\d{3})\s*(?<CustomerNumber>.*)(?<TelNumber>\d{10}).*$`, true)
	addSurguardRegex("SIA-DCS", `S(?<Receiver>\d{2})(?<Line>\d{3})\[#(?<CustomerNumber>\d+)[F]*\|Nri(?<Partition>\d)\/(?<Events>.+?)\]`, true)
	addSurguardRegex("IP", `0(?<Receiver>\d{2})(?<Line>\d{3})\[#?(?<CustomerNumber>[a-fA-F0-9]*)\|(?<Payload>[a-zA-Z0-9.]*)].*$`, true)
	addSurguardRegex("PING", `^[\d\s]*@`, true)
}

func addSurguardRegex(name, regexText string, isActive bool) {
	compiledRegex, err := regexp.Compile(regexText)
	if err != nil {
		log.Fatalf("Regex derlenirken hata oluştu: %v", err)
	}
	surguardRegexes[name] = &model.SubRegex{
		Name:          name,
		RegexText:     regexText,
		IsActive:      isActive,
		CompiledRegex: compiledRegex,
	}
}

func ParseSurguard(event, receiverId string) (signal []interface{}, ack string, err error) {
	data := map[string]string{}
	/*
		if event[0:4] == "1011" {
			signal = append(signal, model.PingSignal{
				Type:             "ping",
				ReceiverId:       receiverId,
				MonitoringCenter: 1,
				RawSignal:        event,
			})
		}
		else
	*/
	if event[0] == '4' {
		data = applySurguardRegex(event, "TEL")
		if data == nil {
			return nil, "", nil
		}
		signal = append(signal, model.PhoneSignal{
			Type:             "event",
			SideNo:           data["CustomerNumber"],
			ReceiverId:       receiverId,
			ReceiverNo:       data["Receiver"],
			LineNo:           data["Line"],
			PhoneNo:          data["TelNumber"],
			MonitoringCenter: 1,
		})
	} else if event[0] == '5' {
		data = applySurguardRegex(event, "ADM-CID-1")
		if data == nil {
			data = applySurguardRegex(event, "ADM-CID-2")
		}
		if data == nil {
			return nil, "", nil
		}

		signal = append(signal, model.AlarmSignal{
			Type:             "event",
			SideNo:           data["CustomerNumber"],
			ReceiverId:       receiverId,
			ReceiverNo:       data["Receiver"],
			LineNo:           data["Line"],
			PartNo:           data["Partition"],
			MonitoringCenter: 1,
			SignalDateTime:   time.Now(),
			EventCode:        data["EventType"] + data["Event"],
			Zone:             data["Zone"],
			RawSignal:        event,
		})
	} else if event[0] == '0' {
		data = applySurguardRegex(event, "IP")
	} else if event[0] == 'S' {
		data = applySurguardRegex(event, "SIA-DCS")
		if data == nil {
			return nil, "", nil
		}

		events := strings.Split(data["Events"], "/")
		for _, e := range events {
			fmt.Println("events", e)
			evt, zn := model.SiaEventOrZone(e)
			signal = append(signal, model.AlarmSignal{
				Type:             "event",
				SideNo:           data["CustomerNumber"],
				ReceiverId:       receiverId,
				ReceiverNo:       data["Receiver"],
				LineNo:           data["Line"],
				PartNo:           data["Partition"],
				MonitoringCenter: 1,
				SignalDateTime:   time.Now(),
				RawSignal:        event,
				EventCode:        evt,
				Zone:             zn,
			})
			fmt.Println("Test Json", signal)
		}
	}

	return signal, string([]byte{0x06}), nil
}

func applySurguardRegex(eventStr, regexName string) map[string]string {
	regex, exists := surguardRegexes[regexName]
	if !exists {
		fmt.Printf("No regex found for %s, %s\n", eventStr, regexName)
		return nil
	}

	match := regex.CompiledRegex.FindStringSubmatch(eventStr)
	if match == nil {
		fmt.Println("No match found", eventStr, regex)
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
