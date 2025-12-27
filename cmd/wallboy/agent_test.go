package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"60 seconds = 1 minute", 60 * time.Second, "1 minute"},
		{"90 seconds = 1.5 minutes", 90 * time.Second, "1.5 minutes"},
		{"120 seconds = 2 minutes", 120 * time.Second, "2 minutes"},
		{"300 seconds = 5 minutes", 300 * time.Second, "5 minutes"},
		{"600 seconds = 10 minutes", 600 * time.Second, "10 minutes"},
		{"1800 seconds = 30 minutes", 1800 * time.Second, "30 minutes"},
		{"150 seconds = 2.5 minutes", 150 * time.Second, "2.5 minutes"},
		{"180 seconds = 3 minutes", 180 * time.Second, "3 minutes"},
		{"45 seconds = 0.8 minutes", 45 * time.Second, "0.8 minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{"home path", "/Users/test/file.txt", ""},
		{"non-home path", "/var/log/test.log", "/var/log/test.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenPath(tt.path)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			}
		})
	}
}

func TestAgentConstants(t *testing.T) {
	assert.Equal(t, 600, defaultInterval)
	assert.Equal(t, 60, minInterval)
}
