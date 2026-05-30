package handler

import (
	"encoding/json"
	"fmt"
	"io"

	"diff-lens/internal/review"
)

// WriteSSE 将一个领域事件序列化为 Server-Sent Events 传输格式。
func WriteSSE(w io.Writer, event review.ReviewEvent) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", event.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}

	return nil
}
