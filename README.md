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
go build
````

This will generate a binary called `bots-arena` in the current directory.

---

### ▶️ Run

To launch the simulation:

```bash
./bots-arena
```

Or directly via:

```bash
go run .
```

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
