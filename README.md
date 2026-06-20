# 🧠 Bots Arena

**Bots Arena** is an experimental OpenGL-based simulation and AI playground written in **Go**.  
It renders a procedurally generated 2D grid map and simulates autonomous bots with genome-based behavior. Bots can move, evolve, gather resources, form colonies, and build simple structures — all visualized in real-time using GLFW and OpenGL bindings.

---

## 📦 Features

- 🧬 Genome-driven bots with custom instruction set
- ⚙️ Procedural grid world generation with dynamic terrain (walls, farms, poison, resources, etc.)
- 🏗 Simple building mechanics: bots can construct farms, walls, and spawners
- 🧃 Energy and inventory system: bots need to survive and manage resources
- 🧠 Basic colony formation logic with task assignment and color-coded control
- ♻️ Evolution: new generations emerge when populations drop below a threshold
- 📈 Dynamic OpenGL rendering with zoom, pan, overlays, and bot path visualization

---

## 🚀 Getting Started

### Prerequisites

- Go **1.22** or later
- A working C compiler (`gcc`, `clang`, or `mingw` depending on your platform)
- OpenGL development libraries and GLFW installed (OS-dependent)
- Internet access to download Go dependencies

---

### 🔧 Build Instructions

Clone and build the project:

```bash
git clone https://github.com/yourname/bots-arena.git
cd bots-arena

go mod download
go build -o bin/golab ./cmd/golab
````

This will generate a binary at `bin/golab`.

---

### ▶️ Run

To launch the simulation:

```bash
./bin/golab
```

Or directly via:

```bash
go run ./cmd/golab
```

### Deterministic CLI surface (Linux/Discord automation)

The binary also supports JSON-oriented command entry points for deterministic workflows:

```bash
go run ./cmd/golab status --seed 42 --ticks 20 --top-bots 10
go run ./cmd/golab match --seed 42 --ticks 300
go run ./cmd/golab leaderboard --seed 100 --matches 3 --seed-step 7 --ticks 300
go run ./cmd/golab replay --seed 42 --ticks 120 --sample-every 5
go run ./cmd/golab gamemaster --seed 7 --ticks 120 --interval 20
go run ./cmd/golab gamemaster --seed 7 --ticks 120 --interval 20 \
  --advisor external --gm-command /home/alice/projects/coolio-arena-master/bin/coolio-arena-master
go run ./cmd/golab render --seed 42 --ticks 120 --output /tmp/bots-arena.png
```

All command modes are emitted as JSON and are deterministic for a fixed `--seed`:

- `status`: one deterministic summary after a finite number of ticks.
- `match`: one deterministic summary with winner fields.
- `leaderboard`: deterministic aggregate of multiple matches.
- `replay`: per-frame snapshots at a fixed sampling interval.
- `gamemaster`: mock game-master observations plus interventions such as resource rain, poison bloom, cooling rain, famine wind, and emergency bot sparks.
- `render`: PNG board render using the same atlas-backed tile style as the game by default. Use `--style biome --padding 24 --border --legend` for ecological terrain diagnostics, `--style pheromone` for scent fields, `--style colony` for colony tissue, or `--style flat` for compact card-style images.

The existing interactive mode remains unchanged when no command name is provided.
Interactive mode can also use an external local game-master process:

```bash
./bin/golab --gm external \
  --gm-command /home/alice/projects/coolio-arena-master/bin/coolio-arena-master \
  --gm-interval 120 --gm-timeout 750ms
```

The external process receives one compact observation JSON on stdin and returns one
event JSON on stdout. If the command fails or times out, `golab` falls back to the
mock game master for that observation.

---

## 🎮 Controls

| Action                    | Input                       |
| ------------------------- | --------------------------- |
| Pan view                  | Drag with left mouse button |
| Zoom in/out               | Mouse wheel scroll          |
| Pause simulation          | Press `P` (config toggle)   |
| Reset simulation          | Press `R`                   |
| Cycle render mode         | Press `V`                   |
| Save genome               | Press `G`                   |
| Save map                  | Press `M`                   |
| Select god tool           | Press `1`-`0`               |
| Use selected god tool     | Left click or drag on board |
| Observe task path overlay | Hover over task-linked bots |

Interactive saves are written as JSON under `data/saves/genomes/` and `data/saves/maps/`.
Render modes cycle through Normal, Genome, Health, Inventory, Colony, Task, Biome, and Pheromone.

---

## 🧠 Architecture Overview

```
core/
  bot.go         → Bot logic, genome, HP, energy
  colony.go      → Colony structure, task queues
  board.go       → Map and grid cell types
  genome.go      → Genome model and instruction logic 
  ...

game/
  game.go        → World loop, bot stepping, controller handling

ui/
  ui.go          → OpenGL rendering and input handling
```

* **Bots** have an instruction pointer, genome matrix, HP, and inventory
* **Tasks** are delegated by colonies: connect positions, maintain links, etc.
* **Board** is a grid of cells with typed content (walls, food, controller, etc.)
* **Game loop** runs bot logic and environmental updates on a tick-based basis

---
![Simulation Screenshot](https://i.imgur.com/1MuVC4Y.png)
---
