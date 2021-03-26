package session

import (
	"encoding/json"
	"fmt"

	"vindr-lab-api/constants"
)

var SessionItemType = map[string]bool{
	constants.SessionItemTypeTask:  true,
	constants.SessionItemTypeStudy: true,
}

type Session struct {
	Data      []SessionItem `json:"data"`
	SessionID string        `json:"session_id"`
	Created   int64         `json:"created"`
}

type SessionItem struct {
	Type string                  `json:"type"`
	ID   string                  `json:"id"`
	Meta *map[string]interface{} `json:"meta"`
}

func (session *Session) IsValidData() bool {
	if len(session.Data) == 0 {
		return false
	}
	for _, item := range session.Data {
		if _, found := SessionItemType[item.Type]; !found {
			return false
		}
	}
	return true
}

func (session *Session) String() string {
	b, err := json.Marshal(session)
	if err != nil {
		fmt.Println(err)
		return "{}"
	}
	return string(b)
}
