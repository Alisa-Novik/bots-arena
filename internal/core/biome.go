package core

type Biome uint8

const (
	BiomeNeutral Biome = iota
	BiomeFertile
	BiomeMineral
	BiomeToxic
)

func (b Biome) String() string {
	switch b {
	case BiomeFertile:
		return "fertile"
	case BiomeMineral:
		return "mineral"
	case BiomeToxic:
		return "toxic"
	default:
		return "neutral"
	}
}

func (b *Board) populateDeterministicBiomes() {
	for idx := range b.biomes {
		b.biomes[idx] = generatedBiome(Position{R: idx / Cols, C: idx % Cols})
	}
}

func generatedBiome(pos Position) Biome {
	if pos.R <= 0 || pos.R >= Rows-1 {
		return BiomeNeutral
	}

	const regionSize = 28
	regionR := pos.R / regionSize
	regionC := pos.C / regionSize
	bestDist := int(^uint(0) >> 1)
	bestR, bestC := regionR, regionC

	for dr := -1; dr <= 1; dr++ {
		candidateR := regionR + dr
		if candidateR < 0 || candidateR*regionSize >= Rows {
			continue
		}
		for dc := -1; dc <= 1; dc++ {
			candidateC := regionC + dc
			h := biomeHash(uint32(candidateR), uint32(candidateC))
			siteR := candidateR*regionSize + int(h%regionSize)
			siteC := wrapCol(candidateC*regionSize + int((h>>8)%regionSize))
			dR := pos.R - siteR
			dC := toroidalColDistance(pos.C, siteC)
			dist := dR*dR + dC*dC
			if dist < bestDist {
				bestDist = dist
				bestR, bestC = candidateR, candidateC
			}
		}
	}

	switch biomeHash(uint32(bestR), uint32(bestC)) % 100 {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19:
		return BiomeFertile
	case 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37:
		return BiomeMineral
	case 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49:
		return BiomeToxic
	default:
		return BiomeNeutral
	}
}

func biomeHash(a, b uint32) uint32 {
	x := a*0x9e3779b9 ^ b*0x85ebca6b ^ 0x27d4eb2d
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	x *= 0x846ca68b
	x ^= x >> 16
	return x
}

func wrapCol(c int) int {
	return (c%Cols + Cols) % Cols
}

func toroidalColDistance(a, b int) int {
	d := a - b
	if d < 0 {
		d = -d
	}
	if wrapped := Cols - d; wrapped < d {
		return wrapped
	}
	return d
}

func OreVeinScore(pos Position) int {
	if !Inside(pos) || pos.R <= 0 || pos.R >= Rows-1 {
		return 0
	}

	const regionSize = 42
	regionR := pos.R / regionSize
	regionC := pos.C / regionSize
	best := 0

	for dr := -2; dr <= 2; dr++ {
		candidateR := regionR + dr
		if candidateR < 0 || candidateR*regionSize >= Rows {
			continue
		}
		for dc := -2; dc <= 2; dc++ {
			candidateC := regionC + dc
			h := biomeHash(uint32(candidateR)^0x517cc1b7, uint32(candidateC)^0x68bc21eb)
			siteR := candidateR*regionSize + 1 + int(h%uint32(regionSize-2))
			if siteR >= Rows-1 {
				siteR = Rows - 2
			}
			siteC := wrapCol(candidateC*regionSize + int((h>>8)%regionSize))
			length := 30 + int((h>>16)%42)
			thickness := 2 + int((h>>22)%4)
			axis := int((h >> 26) % 4)
			score := oreVeinCandidateScore(pos, siteR, siteC, length, thickness, axis)
			if score > best {
				best = score
			}
		}
	}
	if best > 100 {
		return 100
	}
	return best
}

func oreVeinCandidateScore(pos Position, siteR, siteC, length, thickness, axis int) int {
	dx := signedToroidalColDelta(pos.C, siteC)
	dy := pos.R - siteR
	along, cross, scale := 0, 0, 1
	switch axis {
	case 0:
		along = absInt(dx)
		cross = absInt(dy)
	case 1:
		along = absInt(dy)
		cross = absInt(dx)
	case 2:
		along = absInt(dx+dy) / 2
		cross = absInt(dy - dx)
		scale = 2
	default:
		along = absInt(dx-dy) / 2
		cross = absInt(dy + dx)
		scale = 2
	}

	halfLength := length / 2
	if along > halfLength {
		overrun := along - halfLength
		if overrun > thickness*4 {
			return 0
		}
		cross += overrun * scale
	}

	coreWidth := thickness * scale
	fringeWidth := (thickness + 4) * scale
	switch {
	case cross <= coreWidth:
		return 78 + (coreWidth-cross)*22/max(1, coreWidth)
	case cross <= fringeWidth:
		return 34 + (fringeWidth-cross)*42/max(1, fringeWidth-coreWidth)
	default:
		return 0
	}
}

func signedToroidalColDelta(a, b int) int {
	d := a - b
	if d > Cols/2 {
		d -= Cols
	}
	if d < -Cols/2 {
		d += Cols
	}
	return d
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
