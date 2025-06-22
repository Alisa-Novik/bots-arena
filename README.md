# Bots Arena

Bots Arena is a small OpenGL playground written in Go. It renders a grid of cells and spawns simple bots that roam around the map. The project is experimental and mainly serves as a demo of using GLFW and go-gl bindings.

## Prerequisites

- Go **1.22** or newer
- A C compiler and an environment capable of building Go packages that depend on OpenGL and GLFW
- Internet access to download the Go module dependencies listed in `go.mod`

## Build

Fetch the dependencies and compile the project with:

```bash
go mod download
go build
```

This produces an executable named `bots-arena` in the project directory.

## Run

Launch the application by running the built binary:

```bash
./bots-arena
```

During development you can also run directly from source:

```bash
go run .
```
