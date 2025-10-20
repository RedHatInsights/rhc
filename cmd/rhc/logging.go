package main

import (
	"log/slog"
)

type LogMessage struct {
	level   slog.Level
	message error
}
