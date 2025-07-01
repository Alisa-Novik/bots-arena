package util

import (
	"golab/bot"
	"os"
	"strconv"
	"strings"
)

func ReadGenome() *bot.Genome {
	data, _ := os.ReadFile("genome")
	parts := strings.Split(strings.TrimSuffix(string(data), ","), ",")
	var genome [128]int
	for i := range genome {
		genome[i], _ = strconv.Atoi(parts[i])
	}
	return &bot.Genome{Matrix: genome}
}

func ExportGenome(b bot.Bot) {
	var bld strings.Builder
	for _, v := range b.Genome.Matrix {
		bld.WriteString(strconv.Itoa(v))
		bld.WriteByte(',')
	}
	os.WriteFile("genome", []byte(bld.String()), 0644)
}
