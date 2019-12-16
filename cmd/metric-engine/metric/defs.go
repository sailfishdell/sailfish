package metric

import (
	"database/sql/driver"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
)

const (
	MetricValueEvent eh.EventType = "MetricValueEvent"
)

type SqlTimeInt struct {
	time.Time
}

func (m SqlTimeInt) Value() (driver.Value, error) {
	return m.UnixNano(), nil
}

func (m *SqlTimeInt) Scan(src interface{}) error {
	m.Time = time.Unix(0, src.(int64))
	return nil
}

type MetricValueEventData struct {
	Timestamp SqlTimeInt `db:"Timestamp"`
	Name      string     `db:"Name"`
	Value     string
	Property  string `db:"Property"`
	Context   string `db:"Context"`
}

var reglock = sync.Once{}

func RegisterEvent() {
	reglock.Do(func() {
		eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })
	})
}
