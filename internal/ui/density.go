package ui

import "golab/internal/core"

const DensityChunkSize = 4

type DensityChunk struct {
	Row   int
	Col   int
	Count int
	Color [3]float32
}

var densityBuildState struct {
	counts  []int
	sums    [][3]float32
	touched []int
}

func BuildDensityChunks(brd *core.Board, chunkSize int) []DensityChunk {
	return buildDensityChunksInto(nil, brd, chunkSize, RenderModeNormal)
}

func buildDensityChunksInto(out []DensityChunk, brd *core.Board, chunkSize int, mode RenderMode) []DensityChunk {
	out = out[:0]
	if brd == nil {
		return out
	}
	if chunkSize <= 0 {
		chunkSize = DensityChunkSize
	}

	chunkRows := (core.Rows + chunkSize - 1) / chunkSize
	chunkCols := (core.Cols + chunkSize - 1) / chunkSize
	totalChunks := chunkRows * chunkCols
	ensureDensityBuildCapacity(totalChunks)
	densityBuildState.touched = densityBuildState.touched[:0]

	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if bot == nil {
			continue
		}
		cell := brd.BotCell(id)
		if cell < 0 {
			continue
		}
		row := cell / core.Cols
		col := cell % core.Cols
		chunkIdx := (row/chunkSize)*chunkCols + col/chunkSize
		if densityBuildState.counts[chunkIdx] == 0 {
			densityBuildState.touched = append(densityBuildState.touched, chunkIdx)
		}
		densityBuildState.counts[chunkIdx]++
		color := botDensityColor(bot, mode)
		densityBuildState.sums[chunkIdx][0] += color[0]
		densityBuildState.sums[chunkIdx][1] += color[1]
		densityBuildState.sums[chunkIdx][2] += color[2]
	}

	chunkArea := chunkSize * chunkSize
	for _, chunkIdx := range densityBuildState.touched {
		count := densityBuildState.counts[chunkIdx]
		sum := densityBuildState.sums[chunkIdx]
		chunkRow := chunkIdx / chunkCols
		chunkCol := chunkIdx % chunkCols
		out = append(out, DensityChunk{
			Row:   chunkRow * chunkSize,
			Col:   chunkCol * chunkSize,
			Count: count,
			Color: densityChunkColor(count, sum, chunkArea),
		})
		densityBuildState.counts[chunkIdx] = 0
		densityBuildState.sums[chunkIdx] = [3]float32{}
	}
	return out
}

func ensureDensityBuildCapacity(totalChunks int) {
	if len(densityBuildState.counts) >= totalChunks {
		return
	}
	densityBuildState.counts = make([]int, totalChunks)
	densityBuildState.sums = make([][3]float32, totalChunks)
	densityBuildState.touched = make([]int, 0, totalChunks)
}

func botDensityColor(bot *core.Bot, mode RenderMode) [3]float32 {
	prev := ctrlState.RenderMode
	ctrlState.RenderMode = mode
	color := botRenderColor(bot)
	ctrlState.RenderMode = prev
	return color
}

func densityChunkColor(count int, sum [3]float32, chunkArea int) [3]float32 {
	if count <= 0 {
		return clrDefault
	}
	invCount := 1 / float32(count)
	avg := [3]float32{sum[0] * invCount, sum[1] * invCount, sum[2] * invCount}
	occupancy := clamp01(float32(count) / float32(max(1, chunkArea)))
	return lerpColor(lerpColor(clrDefault, avg, 0.68), clrWhite, occupancy*0.22)
}
