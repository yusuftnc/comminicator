package model

import (
	"fmt"
	"regexp"
	"time"
)

// SubRegex struct, her bir regex ifadesi için metadata ve derlenmiş regex'i tutar
type SubRegex struct {
	Name          string
	RegexText     string
	IsActive      bool
	CompiledRegex *regexp.Regexp
}

type AlarmSignal struct {
	Type             string    `json:"type"`
	SideNo           string    `json:"sideNo"`
	ReceiverId       string    `json:"receiverId"`
	ReceiverNo       string    `json:"receiverNo"`
	LineNo           string    `json:"lineNo"`
	PartNo           string    `json:"partNo"`
	MonitoringCenter int       `json:"monitoringCenter"`
	SignalDateTime   time.Time `json:"signalDateTime"`
	EventCode        string    `json:"eventCode"`
	Zone             string    `json:"zone"`
	RawSignal        string    `json:"rawSignal"`
}

type PhoneSignal struct {
	Type             string `json:"type"`
	SideNo           string `json:"sideNo"`
	ReceiverId       string `json:"receiverId"`
	ReceiverNo       string `json:"receiverNo"`
	LineNo           string `json:"lineNo"`
	PhoneNo          string `json:"phoneNo"`
	MonitoringCenter int    `json:"monitoringCenter"`
}

type PingSignal struct {
	Type             string `json:"type"`
	SideNo           string `json:"sideNo"`
	ReceiverId       string `json:"receiverId"`
	RawSignal        string `json:"rawSignal"`
	MonitoringCenter int    `json:"monitoringCenter"`
}

func SiaEventOrZone(eventZone string) (event string, zone string) {
	pattern := regexp.MustCompile(`([a-zA-Z]+)(\d+)`)

	matches := pattern.FindStringSubmatch(eventZone)
	if matches != nil && len(matches) > 2 {
		event := matches[1]
		zone := matches[2]
		return event, zone
	} else {
		fmt.Println("No match found")
	}
	return "", ""
}
