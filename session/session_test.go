package session

import (
	"testing"

	"vindr-lab-api/constants"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type SessionTestSuite struct {
	suite.Suite
	app    *SessionAPI
	engine *gin.Engine
}

func TestVerifySessionData(t *testing.T) {
	session := Session{
		Data: []SessionItem{{
			Type: "x",
			ID:   "y",
		}},
	}
	assert.Equal(t, false, session.IsValidData(), "should be false")
}

func TestVerifySessionData2(t *testing.T) {
	session := Session{
		Data: []SessionItem{},
	}
	assert.Equal(t, false, session.IsValidData(), "should be false")
}

func TestVerifySessionData3(t *testing.T) {
	session := Session{
		Data: []SessionItem{{
			Type: constants.SessionItemTypeTask,
			ID:   "y",
		}},
	}
	assert.Equal(t, true, session.IsValidData(), "should be true")
}

func TestVerifySessionData4(t *testing.T) {
	session := Session{
		Data: []SessionItem{
			{
				Type: constants.SessionItemTypeTask,
				ID:   "y",
			},
			{
				Type: constants.SessionItemTypeStudy,
				ID:   "cccc",
			},
		},
	}
	assert.Equal(t, true, session.IsValidData(), "should be true")
}

func TestVerifySessionData5(t *testing.T) {
	session := Session{
		Data: []SessionItem{
			{
				Type: constants.SessionItemTypeTask,
				ID:   "y",
			},
			{
				Type: constants.SessionItemTypeStudy,
				ID:   "cccc",
			},
			{
				Type: "test",
				ID:   "cccc",
			},
		},
	}
	assert.Equal(t, false, session.IsValidData(), "should be false")
}
