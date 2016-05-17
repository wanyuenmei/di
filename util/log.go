package util

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

// Formatter implements the log formatter for Quilt.
type Formatter struct{}

// Format converts a logrus entry into a string for logging.
func (f Formatter) Format(entry *log.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	level := strings.ToUpper(entry.Level.String())
	fmt.Fprintf(b, "%s [%s] %-40s", level, entry.Time.Format(time.StampMilli),
		entry.Message)

	for k, v := range entry.Data {
		fmt.Fprintf(b, " %s=%+v", k, v)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}
