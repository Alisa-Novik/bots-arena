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
go run ./cmd/golab render --seed 42 --ticks 120 --output /tmp/bots-arena.png
```

All command modes are emitted as JSON and are deterministic for a fixed `--seed`:

- `status`: one deterministic summary after a finite number of ticks.
- `match`: one deterministic summary with winner fields.
- `leaderboard`: deterministic aggregate of multiple matches.
- `replay`: per-frame snapshots at a fixed sampling interval.
- `render`: PNG board render using the same atlas-backed tile style as the game by default. Use `--style flat --padding 24 --border --legend` for the compact card-style diagnostic image.

The existing interactive mode remains unchanged when no command name is provided.

---

## 🎮 Controls

| Action                    | Input                       |
| ------------------------- | --------------------------- |
| Pan view                  | Drag with left mouse button |
| Zoom in/out               | Mouse wheel scroll          |
| Pause simulation          | Press `P` (config toggle)   |
| Observe task path overlay | Hover over task-linked bots |

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
