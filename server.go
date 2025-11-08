package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type SensorServer struct {
	Value     float64   `json:"value"`      // lastest value
	UpdatedAt time.Time `json:"updated_at"` // lastest updated at
	Metadata  any       `json:"metadata"`   // lastest metadata

	sync.RWMutex
}

func (s *SensorServer) SetValue(value float64, metadata any) {
	s.Lock()
	defer s.Unlock()
	s.Value = value
	s.Metadata = metadata
	s.UpdatedAt = time.Now()
}

func (s *SensorServer) GetValueHandler(c *gin.Context) {
	s.RLock()
	defer s.RUnlock()

	if s.UpdatedAt.IsZero() {
		c.JSON(http.StatusTooEarly, gin.H{
			"error": "no value yet",
		})
		return
	}

	c.JSON(http.StatusOK, s)
}
