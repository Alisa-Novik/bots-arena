package main

import (
	"golab/bot"
	"testing"
)

func Test_drawGrid(t *testing.T) {
	tests := []struct {
		name string // description of this test case
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drawGrid()
		})
	}
}

func Test_generateBots(t *testing.T) {
	tests := []struct {
		name string // description of this test case
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialBotsGeneration()
		})
	}
}

func Test_botAction(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		startPos Position
		b        bot.Bot
		newBots  map[Position]bot.Bot
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			botAction(tt.startPos, tt.b, tt.newBots)
		})
	}
}


func Test_botAction(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		startPos Position
		b        bot.Bot
		newBots  map[Position]bot.Bot
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			botAction(tt.startPos, tt.b, tt.newBots)
		})
	}
}

