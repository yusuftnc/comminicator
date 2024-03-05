package protocol

import (
	"agent/model"
	"fmt"
	"log"
	"regexp"
	"time"
)

var subAdemcoRegexes = map[string]*model.SubRegex{}

/*
49B852128034294
59B8518340101004
*/
func init() {
	addAdemcoRegex("ADM-CID", `^5(?<CustomerNumber>.*)(?<ContactCode>\d{2})(?<EventType>\d{1})(?<Event>\d{3})(?<Partition>\d{2})(?<Zone>\d{3}).*$`, true)
	addAdemcoRegex("TEL", `^4(?<CustomerNumber>.*)(?<TelNumber>\d{10})$`, true)
}

func addAdemcoRegex(name, regexText string, isActive bool) {
	compiledRegex, err := regexp.Compile(regexText)
	if err != nil {
		log.Fatalf("Regex derlenirken hata oluştu: %v", err)
	}
	subAdemcoRegexes[name] = &model.SubRegex{
		Name:          name,
		RegexText:     regexText,
		IsActive:      isActive,
		CompiledRegex: compiledRegex,
	}
}
func ParseAdemco(event, receiverId string) (signal []interface{}, ack string, err error) {
	eventData := map[string]string{}
	//fmt.Println("BAKMA BAKALIM=", event)

	fmt.Println("-----------------------------------------------------------------------")

	if event[0] == '5' {
		eventData = applyAdemcoRegex(event, "ADM-CID")
		signal = append(signal, model.AlarmSignal{
			Type:             "event",
			SideNo:           eventData["CustomerNumber"],
			ReceiverId:       receiverId,
			ReceiverNo:       eventData["Receiver"],
			LineNo:           eventData["Line"],
			PartNo:           eventData["Partition"],
			MonitoringCenter: 1,
			SignalDateTime:   time.Now(),
			RawSignal:        event,
			EventCode:        eventData["EventType"] + eventData["Event"],
			Zone:             eventData["Zone"],
		})
	} else if event[0] == '4' {
		eventData = applyAdemcoRegex(event, "TEL")
		if eventData == nil {
			return nil, "", nil
		}
		signal = append(signal, model.PhoneSignal{
			Type:             "phone",
			SideNo:           eventData["CustomerNumber"],
			ReceiverId:       receiverId,
			ReceiverNo:       eventData["Receiver"],
			LineNo:           eventData["Line"],
			PhoneNo:          eventData["TelNumber"],
			MonitoringCenter: 1,
		})
	}

	return signal, string([]byte{0x06}), nil
}

func applyAdemcoRegex(eventStr, regexName string) map[string]string {
	regex, exists := subAdemcoRegexes[regexName]
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
