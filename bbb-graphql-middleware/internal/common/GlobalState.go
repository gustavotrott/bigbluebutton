package common

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

var uniqueID string

func InitUniqueID() {
	uniqueID = uuid.New().String()
}

func GetUniqueID() string {
	return uniqueID
}

var myHashList = make(map[string]time.Time)

func TestTime(status string, hash string) {
	now := time.Now()
	if _, exists := myHashList[hash]; !exists {
		myHashList[hash] = now
		fmt.Printf("%s %s at %s\n", status, hash, now.Format("2006-01-02 15:04:05.00000"))
	}

	if status == "finishing" {
		fmt.Printf("%s %s at %s (%v)\n", status, hash, now.Format("2006-01-02 15:04:05.00000"), time.Since(myHashList[hash]))
	}

}

var JsonPatchCache = make(map[string][]byte)
