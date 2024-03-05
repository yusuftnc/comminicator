package protocol

import "fmt"

func ProcessEBSXML(event string) (ack string, err error) {
	fmt.Println("ProcessEBSXML", event)
	return string([]byte{0x06}), nil
}
